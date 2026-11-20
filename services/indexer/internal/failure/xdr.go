package failure

import (
	"encoding/base64"
	"encoding/binary"

	"github.com/sorolens/sorolens/services/indexer/internal/scval"
)

// ---- XDR constants ---------------------------------------------------------

// TransactionResultCode values (Stellar-transaction.x).
const (
	txSuccess           int32 = 0
	txFailed            int32 = -1
	txTooEarly          int32 = -2
	txTooLate           int32 = -3
	txMissingOperation  int32 = -4
	txBadSeq            int32 = -5
	txBadAuth           int32 = -6
	txInsufficientBal   int32 = -7
	txNoAccount         int32 = -8
	txInsufficientFee   int32 = -9
	txBadAuthExtra      int32 = -10
	txInternalError     int32 = -11
	txNotSupported      int32 = -12
	txFeeBumpInner      int32 = -13
	txBadSponsorship    int32 = -14
	txBadMinSeqAgeOrGap int32 = -15
	txMalformed         int32 = -16
	txSorobanInvalid    int32 = -17
)

// OperationResultCode values.
const (
	opInner int32 = 0
)

// OperationTypeInvokeHostFunction is the OperationType discriminant of a
// Soroban invocation.
const opInvokeHostFunction uint32 = 24

// InvokeHostFunctionResultCode values.
const (
	invokeHostMalformed                 int32 = -1
	invokeHostTrapped                   int32 = -2
	invokeHostResourceLimitExceeded     int32 = -3
	invokeHostEntryArchived             int32 = -4
	invokeHostInsufficientRefundableFee int32 = -5
)

// SCErrorType discriminants (Stellar-contract.x).
const (
	scErrorContract  uint32 = 0
	scErrorWasmVM    uint32 = 1
	scErrorContext   uint32 = 2
	scErrorStorage   uint32 = 3
	scErrorObject    uint32 = 4
	scErrorCrypto    uint32 = 5
	scErrorEvents    uint32 = 6
	scErrorBudget    uint32 = 7
	scErrorValueType uint32 = 8
	scErrorAuth      uint32 = 9
)

// SCValTypeScvError is the SCVal discriminant of an ScVal::Error.
const scValTypeError uint32 = 2

const (
	// maxTopics bounds the topic vector of one diagnostic event.
	maxTopics = 64
	// maxElements bounds the vec/map length walked inside one ScVal.
	maxElements = 4096
	// maxDiagnosticEvents bounds how many events one classification scans.
	maxDiagnosticEvents = 1024
)

// TransactionResultResult is the subset of a TransactionResult this package
// needs.
type resultCodes struct {
	txCode     int32
	invokeCode int32
	hasInvoke  bool
}

// decodeTransactionResultXDR decodes a base64 TransactionResult far enough to
// know the TransactionResultCode and, for a failed invokeHostFunction
// operation, the InvokeHostFunctionResultCode.
func decodeTransactionResultXDR(b64 string) (resultCodes, bool) {
	if b64 == "" {
		return resultCodes{}, false
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return resultCodes{}, false
	}
	return decodeTransactionResult(&cursor{buf: raw})
}

// decodeTransactionResult walks:
//
//	struct TransactionResult {
//	    int64 feeCharged;
//	    union switch (TransactionResultCode code) {
//	    case txSUCCESS:
//	    case txFAILED:          OperationResult results<>;
//	    case txFEE_BUMP_INNER_FAILED: TransactionResultPair innerResultPair;
//	    default:                void;
//	    } result;
//	    union switch (int v) { case 0: void; } ext;
//	};
//
// Only the first operation result is inspected: a Soroban transaction carries
// exactly one invokeHostFunction operation, and parsing another operation's
// result body would need the whole operation result union for no benefit.
func decodeTransactionResult(c *cursor) (resultCodes, bool) {
	if _, ok := c.u64(); !ok { // feeCharged
		return resultCodes{}, false
	}
	code, ok := c.i32()
	if !ok {
		return resultCodes{}, false
	}
	out := resultCodes{txCode: code}

	switch code {
	case txFailed:
		n, ok := c.u32()
		if !ok || n == 0 {
			return out, true
		}
		opCode, ok := c.i32()
		if !ok || opCode != opInner {
			return out, true
		}
		opType, ok := c.u32()
		if !ok || opType != opInvokeHostFunction {
			return out, true
		}
		invoke, ok := c.i32()
		if !ok {
			return out, true
		}
		out.invokeCode = invoke
		out.hasInvoke = true

	case txFeeBumpInner:
		// TransactionResultPair { Hash transactionHash; TransactionResult
		// result; }: skip the hash and read the inner result.
		if _, ok := c.fixed(32); !ok {
			return out, true
		}
		inner, ok := decodeTransactionResult(c)
		if !ok || inner.txCode == txSuccess {
			return out, true
		}
		return inner, true
	}
	return out, true
}

// hostErrorFromDiagnostics returns the first SCError carried by the transaction's
// diagnostic events. The host records errors as a diagnostic event whose topics
// are [error, ScVal::Error(SCError)], so the scan looks at every topic of every
// event and accepts an error found in the payload too.
func hostErrorFromDiagnostics(events []string) (scval.Error, bool) {
	for i, raw := range events {
		if i >= maxDiagnosticEvents {
			break
		}
		buf, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			continue
		}
		if e, ok := hostErrorFromDiagnosticEvent(buf); ok {
			return e, true
		}
	}
	return scval.Error{}, false
}

// hostErrorFromDiagnosticEvent decodes one xdr.DiagnosticEvent:
//
//	struct DiagnosticEvent { bool inSuccessfulContractCall; ContractEvent event; }
//	struct ContractEvent {
//	    ExtensionPoint ext;
//	    Hash* contractID;
//	    ContractEventType type;
//	    union switch (int v) { case 0: struct { SCVal topics<>; SCVal data; } v0; } body;
//	};
func hostErrorFromDiagnosticEvent(raw []byte) (scval.Error, bool) {
	c := &cursor{buf: raw}

	// inSuccessfulContractCall (a 4-byte bool in XDR).
	if _, ok := c.u32(); !ok {
		return scval.Error{}, false
	}
	// ContractEvent.ext (ExtensionPoint): only the void case exists.
	if v, ok := c.u32(); !ok || v != 0 {
		return scval.Error{}, false
	}
	// ContractEvent.contractID (optional Hash).
	switch present, ok := c.u32(); {
	case !ok:
		return scval.Error{}, false
	case present == 1:
		if _, ok := c.fixed(32); !ok {
			return scval.Error{}, false
		}
	case present != 0:
		return scval.Error{}, false
	}
	// ContractEvent.type.
	if _, ok := c.u32(); !ok {
		return scval.Error{}, false
	}
	// ContractEvent.body union discriminant.
	if v, ok := c.u32(); !ok || v != 0 {
		return scval.Error{}, false
	}

	n, ok := c.u32()
	if !ok || n > maxTopics {
		return scval.Error{}, false
	}
	var (
		found scval.Error
		has   bool
	)
	for i := uint32(0); i < n; i++ {
		if e, isErr, ok := c.scVal(); !ok {
			return scval.Error{}, false
		} else if isErr && !has {
			found, has = e, true
		}
	}
	// The payload of an "error" event is the message; accept an error value
	// there too in case a future protocol version moves it.
	if e, isErr, ok := c.scVal(); ok && isErr && !has {
		found, has = e, true
	}
	return found, has
}

// ---- tiny XDR cursor -------------------------------------------------------

// cursor walks a big-endian XDR buffer. Every read is bounds-checked and
// reports ok=false on a short buffer.
type cursor struct {
	buf []byte
	off int
}

func (c *cursor) u32() (uint32, bool) {
	if len(c.buf)-c.off < 4 {
		return 0, false
	}
	v := binary.BigEndian.Uint32(c.buf[c.off : c.off+4])
	c.off += 4
	return v, true
}

func (c *cursor) i32() (int32, bool) {
	v, ok := c.u32()
	return int32(v), ok
}

func (c *cursor) u64() (uint64, bool) {
	if len(c.buf)-c.off < 8 {
		return 0, false
	}
	v := binary.BigEndian.Uint64(c.buf[c.off : c.off+8])
	c.off += 8
	return v, true
}

func (c *cursor) fixed(n int) ([]byte, bool) {
	if len(c.buf)-c.off < n {
		return nil, false
	}
	out := c.buf[c.off : c.off+n]
	c.off += n
	return out, true
}

// varOpaque reads an XDR variable-length opaque/string: a 4-byte length followed
// by that many bytes, padded to a 4-byte boundary.
func (c *cursor) varOpaque() ([]byte, bool) {
	n, ok := c.u32()
	if !ok || n > maxElements {
		return nil, false
	}
	body, ok := c.fixed(int(n))
	if !ok {
		return nil, false
	}
	if pad := (4 - int(n)%4) % 4; pad > 0 {
		if _, ok := c.fixed(pad); !ok {
			return nil, false
		}
	}
	return body, true
}

// scVal consumes one ScVal and reports the SCError it carries, if any.
func (c *cursor) scVal() (scval.Error, bool, bool) {
	if len(c.buf)-c.off < 4 {
		return scval.Error{}, false, false
	}
	disc := binary.BigEndian.Uint32(c.buf[c.off : c.off+4])
	if disc == scValTypeError {
		c.off += 4
		typ, ok1 := c.u32()
		code, ok2 := c.u32()
		if !ok1 || !ok2 {
			return scval.Error{}, false, false
		}
		return scval.Error{Type: typ, Code: code}, true, true
	}
	if !c.skipScVal() {
		return scval.Error{}, false, false
	}
	return scval.Error{}, false, true
}

// skipScVal consumes one ScVal of any supported type without interpreting it.
func (c *cursor) skipScVal() bool {
	disc, ok := c.u32()
	if !ok {
		return false
	}
	switch disc {
	case 0, 3, 4: // scvBool, scvU32, scvI32
		_, ok = c.u32()

	case 1: // scvVoid

	case 2: // scvError
		var ok1, ok2 bool
		_, ok1 = c.u32()
		_, ok2 = c.u32()
		ok = ok1 && ok2

	case 5, 6, 7, 8: // scvU64, scvI64, scvTimepoint, scvDuration
		_, ok = c.u64()

	case 9, 10: // scvU128, scvI128
		_, ok = c.fixed(16)

	case 11, 12: // scvU256, scvI256
		_, ok = c.fixed(32)

	case 13, 14, 15: // scvBytes, scvString, scvSymbol
		_, ok = c.varOpaque()

	case 16, 17: // scvVec, scvMap
		present, ok := c.u32()
		if !ok {
			return false
		}
		if present == 0 {
			// A null vec/map is a single zero word and fully consumed.
			return true
		}
		if present != 1 {
			return false
		}
		n, ok := c.u32()
		if !ok || n > maxElements {
			return false
		}
		if disc == 17 {
			n *= 2
		}
		for i := uint32(0); i < n; i++ {
			if !c.skipScVal() {
				return false
			}
		}
		return true

	case 18: // scvAddress
		var kind uint32
		kind, ok = c.u32()
		if !ok {
			return false
		}
		switch kind {
		case 0: // account: PublicKey union discriminant + 32-byte key
			var pk uint32
			if pk, ok = c.u32(); !ok || pk > 1 {
				return false
			}
			_, ok = c.fixed(32)
		case 1: // contract
			_, ok = c.fixed(32)
		default:
			return false
		}

	default:
		// scvContractInstance, scvLedgerKey* and future discriminants are
		// not needed to find an error value; stop this event instead of
		// misreading the rest of the buffer.
		return false
	}
	return ok
}
