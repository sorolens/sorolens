// Package failure sorts a failed Soroban invocation into a category (issue
// #806) from the two places the network reports why it failed:
//
//   - the TransactionResult (getTransaction resultXdr), whose
//     InvokeHostFunction operation result code says whether the host ran out
//     of resources (RESOURCE_LIMIT_EXCEEDED), hit an archived entry
//     (ENTRY_ARCHIVED) or trapped (TRAPPED);
//   - the host diagnostic events, where every host failure is logged as an
//     event with topics ["error", Error(<type>, <code>)]. The error ScVal is
//     decoded with internal/scval.
//
// Classify is pure and total: it never panics and always returns a category.
// Anything it cannot recognise or decode is filed as "other" with the raw
// code kept, so no failed invocation is dropped. The mapping is documented in
// docs/failure-categories.md.
//
// Like internal/callgraph, the package reads only the XDR prefix it needs by
// hand instead of decoding whole structures with the stellar SDK. The SDK
// rejects union discriminants it does not know, which would turn a new error
// type or result code into an undecodable record and lose the raw value.
package failure

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"

	"github.com/sorolens/sorolens/apps/api/failurecategory"
	"github.com/sorolens/sorolens/services/indexer/internal/scval"
)

// CodeUnknown is the raw code stored when neither the transaction result nor
// the diagnostic events could be decoded. The raw TransactionResult is still
// kept in invocations.result_xdr.
const CodeUnknown = "UNKNOWN"

// errorTopic is the first topic of the diagnostic event the host emits for
// every error.
const errorTopic = "error"

// Result is the classification of one failed invocation.
type Result struct {
	Category failurecategory.Category
	// Code is the raw error behind the category, as one of:
	//   "Error(<Type>, <Code>)"         the host ScError, e.g. "Error(Contract, #4)"
	//   "INVOKE_HOST_FUNCTION_<NAME>"   the operation result code
	//   "tx<NAME>"                      the transaction result code
	//   "UNKNOWN"                       nothing could be decoded
	// Unknown enum values are written as numbers ("Error(#12, #3)",
	// "INVOKE_HOST_FUNCTION(#-9)", "tx(#-42)") so they are never lost.
	Code string
}

// XDR discriminants this package reads. Values are from Stellar XDR
// (Stellar-transaction.x, Stellar-contract.x), Protocol 20+, and match
// github.com/stellar/go-stellar-sdk/xdr.
const (
	txFeeBumpInnerSuccess int32 = 1
	txSuccess             int32 = 0
	txFailed              int32 = -1
	txFeeBumpInnerFailed  int32 = -13

	opInner                  int32 = 0
	opTypeInvokeHostFunction int32 = 24

	ihfSuccess                   int32 = 0
	ihfMalformed                 int32 = -1
	ihfTrapped                   int32 = -2
	ihfResourceLimitExceeded     int32 = -3
	ihfEntryArchived             int32 = -4
	ihfInsufficientRefundableFee int32 = -5

	scvSymbol uint32 = 15
)

// invokeHostFunctionCodeNames are the XDR InvokeHostFunctionResultCode names.
var invokeHostFunctionCodeNames = map[int32]string{
	ihfSuccess:                   "INVOKE_HOST_FUNCTION_SUCCESS",
	ihfMalformed:                 "INVOKE_HOST_FUNCTION_MALFORMED",
	ihfTrapped:                   "INVOKE_HOST_FUNCTION_TRAPPED",
	ihfResourceLimitExceeded:     "INVOKE_HOST_FUNCTION_RESOURCE_LIMIT_EXCEEDED",
	ihfEntryArchived:             "INVOKE_HOST_FUNCTION_ENTRY_ARCHIVED",
	ihfInsufficientRefundableFee: "INVOKE_HOST_FUNCTION_INSUFFICIENT_REFUNDABLE_FEE",
}

// transactionCodeNames are the XDR TransactionResultCode names.
var transactionCodeNames = map[int32]string{
	txFeeBumpInnerSuccess: "txFEE_BUMP_INNER_SUCCESS",
	txSuccess:             "txSUCCESS",
	txFailed:              "txFAILED",
	-2:                    "txTOO_EARLY",
	-3:                    "txTOO_LATE",
	-4:                    "txMISSING_OPERATION",
	-5:                    "txBAD_SEQ",
	-6:                    "txBAD_AUTH",
	-7:                    "txINSUFFICIENT_BALANCE",
	-8:                    "txNO_ACCOUNT",
	-9:                    "txINSUFFICIENT_FEE",
	-10:                   "txBAD_AUTH_EXTRA",
	-11:                   "txINTERNAL_ERROR",
	-12:                   "txNOT_SUPPORTED",
	txFeeBumpInnerFailed:  "txFEE_BUMP_INNER_FAILED",
	-14:                   "txBAD_SPONSORSHIP",
	-15:                   "txBAD_MIN_SEQ_AGE_OR_GAP",
	-16:                   "txMALFORMED",
	-17:                   "txSOROBAN_INVALID",
	-18:                   "txFROZEN_KEY_ACCESSED",
}

// hostErrorCategories maps an ScErrorType to its category. Types not listed
// (Context, Object, Crypto, Events, Value and any future type) are "other".
var hostErrorCategories = map[uint32]failurecategory.Category{
	scval.ScErrorTypeContract: failurecategory.ContractError,
	scval.ScErrorTypeWasmVm:   failurecategory.WasmTrap,
	scval.ScErrorTypeStorage:  failurecategory.Storage,
	scval.ScErrorTypeBudget:   failurecategory.BudgetExceeded,
	scval.ScErrorTypeAuth:     failurecategory.AuthFailure,
}

// Classify returns the category and raw code of a failed invocation from its
// base64 TransactionResult and base64 DiagnosticEvent stream. Either input may
// be empty or malformed.
//
// Precedence:
//  1. An operation result code that names the cause on its own wins:
//     RESOURCE_LIMIT_EXCEEDED and INSUFFICIENT_REFUNDABLE_FEE are
//     budget_exceeded, ENTRY_ARCHIVED is storage.
//  2. Otherwise (TRAPPED, or no operation result) the host error from the
//     last "error" diagnostic event decides, by its ScErrorType. The last
//     one is the error that ended the invocation; earlier ones may have been
//     caught by a try_call.
//  3. Otherwise the result code alone is kept as the raw code under other:
//     TRAPPED without diagnostic events does not say why the host trapped.
func Classify(resultXDR string, diagnosticEventsXDR []string) Result {
	txCode, opCode, haveTx, haveOp := decodeResult(resultXDR)
	hostErr, haveHost := lastHostError(diagnosticEventsXDR)

	if haveOp {
		switch opCode {
		case ihfResourceLimitExceeded, ihfInsufficientRefundableFee:
			return Result{Category: failurecategory.BudgetExceeded, Code: preferHost(hostErr, haveHost, opCodeName(opCode))}
		case ihfEntryArchived:
			return Result{Category: failurecategory.Storage, Code: preferHost(hostErr, haveHost, opCodeName(opCode))}
		}
	}

	if haveHost {
		cat, ok := hostErrorCategories[hostErr.Type]
		if !ok {
			cat = failurecategory.Other
		}
		return Result{Category: cat, Code: hostErr.String()}
	}

	switch {
	case haveOp:
		return Result{Category: failurecategory.Other, Code: opCodeName(opCode)}
	case haveTx:
		return Result{Category: failurecategory.Other, Code: txCodeName(txCode)}
	default:
		return Result{Category: failurecategory.Other, Code: CodeUnknown}
	}
}

// preferHost keeps the more specific host error as the raw code when there is
// one, falling back to the operation result code.
func preferHost(e scval.ScError, ok bool, fallback string) string {
	if ok {
		return e.String()
	}
	return fallback
}

// decodeResult extracts the transaction result code and, when the transaction
// carries an InvokeHostFunction operation result, that operation's code.
// Fee-bump transactions are unwrapped to their inner result.
//
//	struct TransactionResult {
//	    int64 feeCharged;
//	    union switch (TransactionResultCode code) {
//	    case txFEE_BUMP_INNER_SUCCESS: case txFEE_BUMP_INNER_FAILED:
//	        InnerTransactionResultPair innerResultPair; // Hash + InnerTransactionResult
//	    case txSUCCESS: case txFAILED:
//	        OperationResult results<>;
//	    default: void;
//	    } result; ...
//	};
//
// Only the prefix up to the first operation's result code is read, so a result
// code this package does not know is still returned verbatim.
func decodeResult(resultXDR string) (tx, op int32, haveTx, haveOp bool) {
	if resultXDR == "" {
		return 0, 0, false, false
	}
	raw, err := base64.StdEncoding.DecodeString(resultXDR)
	if err != nil {
		return 0, 0, false, false
	}
	c := &cursor{buf: raw}
	if !c.skip(8) { // feeCharged
		return 0, 0, false, false
	}
	tx, ok := c.i32()
	if !ok {
		return 0, 0, false, false
	}
	if tx == txFeeBumpInnerSuccess || tx == txFeeBumpInnerFailed {
		// Report the inner transaction's code: the outer one only says
		// that the fee bump wrapped it.
		if !c.skip(32 + 8) { // transactionHash, inner feeCharged
			return tx, 0, true, false
		}
		inner, ok := c.i32()
		if !ok {
			return tx, 0, true, false
		}
		tx = inner
	}
	if tx != txSuccess && tx != txFailed {
		return tx, 0, true, false
	}
	// A Soroban transaction has exactly one operation. Only an
	// InvokeHostFunction result in first position is read, because the arms
	// of other operation results have varying sizes.
	if n, ok := c.u32(); !ok || n == 0 {
		return tx, 0, true, false
	}
	opCode, ok1 := c.i32()
	opType, ok2 := c.i32()
	if !ok1 || !ok2 || opCode != opInner || opType != opTypeInvokeHostFunction {
		return tx, 0, true, false
	}
	op, ok = c.i32()
	if !ok {
		return tx, 0, true, false
	}
	return tx, op, true, true
}

// lastHostError returns the ScError of the last diagnostic event shaped
// ["error", Error(..), ...]. Events that do not decode are skipped.
func lastHostError(diagnosticEventsXDR []string) (scval.ScError, bool) {
	var (
		found scval.ScError
		ok    bool
	)
	for _, b64 := range diagnosticEventsXDR {
		if e, isErr := hostErrorFromEvent(b64); isErr {
			found, ok = e, true
		}
	}
	return found, ok
}

// hostErrorFromEvent reads one base64 xdr.DiagnosticEvent:
//
//	struct DiagnosticEvent { bool inSuccessfulContractCall; ContractEvent event; }
//	struct ContractEvent {
//	    ExtensionPoint ext;
//	    ContractID* contractID;
//	    ContractEventType type;
//	    union switch (int v) { case 0: struct { SCVal topics<>; SCVal data; } v0; } body;
//	};
//
// It walks the envelope to the second topic and decodes that ScVal with
// internal/scval, which keeps unknown error types and codes as numbers.
func hostErrorFromEvent(b64 string) (scval.ScError, bool) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return scval.ScError{}, false
	}
	c := &cursor{buf: raw}
	if v, ok := c.u32(); !ok || v > 1 { // inSuccessfulContractCall
		return scval.ScError{}, false
	}
	if v, ok := c.u32(); !ok || v != 0 { // ext: only the void arm exists
		return scval.ScError{}, false
	}
	switch present, ok := c.u32(); { // optional contractID (32-byte hash)
	case !ok || present > 1:
		return scval.ScError{}, false
	case present == 1:
		if !c.skip(32) {
			return scval.ScError{}, false
		}
	}
	if !c.skip(4) { // type
		return scval.ScError{}, false
	}
	if v, ok := c.u32(); !ok || v != 0 { // body arm
		return scval.ScError{}, false
	}
	if n, ok := c.u32(); !ok || n < 2 { // topic count
		return scval.ScError{}, false
	}
	// Topic 0 must be Symbol("error").
	if disc, ok := c.u32(); !ok || disc != scvSymbol {
		return scval.ScError{}, false
	}
	sym, ok := c.varOpaque()
	if !ok || string(sym) != errorTopic {
		return scval.ScError{}, false
	}
	// Topic 1 is handed to scval as-is: Decode reads only the bytes of the
	// value it recognises and ignores what follows.
	sc, err := scval.Decode(base64.StdEncoding.EncodeToString(c.rest()))
	if err != nil {
		return scval.ScError{}, false
	}
	e, ok := sc.Value.(scval.ScError)
	return e, ok
}

func opCodeName(c int32) string {
	if n, ok := invokeHostFunctionCodeNames[c]; ok {
		return n
	}
	return fmt.Sprintf("INVOKE_HOST_FUNCTION(#%d)", c)
}

func txCodeName(c int32) string {
	if n, ok := transactionCodeNames[c]; ok {
		return n
	}
	return fmt.Sprintf("tx(#%d)", c)
}

// cursor is a bounds-checked reader over XDR bytes.
type cursor struct {
	buf []byte
	off int
}

func (c *cursor) u32() (uint32, bool) {
	if len(c.buf)-c.off < 4 {
		return 0, false
	}
	v := binary.BigEndian.Uint32(c.buf[c.off:])
	c.off += 4
	return v, true
}

func (c *cursor) i32() (int32, bool) {
	v, ok := c.u32()
	return int32(v), ok
}

func (c *cursor) skip(n int) bool {
	if n < 0 || len(c.buf)-c.off < n {
		return false
	}
	c.off += n
	return true
}

// varOpaque reads a length-prefixed byte string padded to 4 bytes.
func (c *cursor) varOpaque() ([]byte, bool) {
	n, ok := c.u32()
	if !ok || int64(n) > int64(len(c.buf)-c.off) {
		return nil, false
	}
	b := c.buf[c.off : c.off+int(n)]
	if !c.skip((int(n) + 3) &^ 3) {
		return nil, false
	}
	return b, true
}

func (c *cursor) rest() []byte { return c.buf[c.off:] }
