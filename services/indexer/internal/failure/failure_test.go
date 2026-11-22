package failure

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"testing"

	"github.com/sorolens/sorolens/apps/api/failurecategory"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Fixtures are encoded with github.com/stellar/go-stellar-sdk/xdr, the
// reference encoder, rather than with this package's reader, so the tests
// check the hand-written decoding against the real wire format of
// getTransaction's resultXdr and diagnosticEventsXdr.

func mustB64(t *testing.T, v any) string {
	t.Helper()
	s, err := xdr.MarshalBase64(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	return s
}

// invokeResult is the TransactionResult of a failed (or successful)
// InvokeHostFunction transaction with the given operation result code.
func invokeResult(t *testing.T, code xdr.InvokeHostFunctionResultCode) string {
	t.Helper()
	txCode := xdr.TransactionResultCodeTxFailed
	ihf := xdr.InvokeHostFunctionResult{Code: code}
	if code == xdr.InvokeHostFunctionResultCodeInvokeHostFunctionSuccess {
		txCode = xdr.TransactionResultCodeTxSuccess
		ihf.Success = &xdr.Hash{}
	}
	ops := []xdr.OperationResult{{
		Code: xdr.OperationResultCodeOpInner,
		Tr: &xdr.OperationResultTr{
			Type:                     xdr.OperationTypeInvokeHostFunction,
			InvokeHostFunctionResult: &ihf,
		},
	}}
	return mustB64(t, xdr.TransactionResult{
		FeeCharged: 100_000,
		Result:     xdr.TransactionResultResult{Code: txCode, Results: &ops},
	})
}

// feeBumpResult wraps an InvokeHostFunction failure in a fee-bump result.
func feeBumpResult(t *testing.T, code xdr.InvokeHostFunctionResultCode) string {
	t.Helper()
	ops := []xdr.OperationResult{{
		Code: xdr.OperationResultCodeOpInner,
		Tr: &xdr.OperationResultTr{
			Type:                     xdr.OperationTypeInvokeHostFunction,
			InvokeHostFunctionResult: &xdr.InvokeHostFunctionResult{Code: code},
		},
	}}
	return mustB64(t, xdr.TransactionResult{
		FeeCharged: 200_000,
		Result: xdr.TransactionResultResult{
			Code: xdr.TransactionResultCodeTxFeeBumpInnerFailed,
			InnerResultPair: &xdr.InnerTransactionResultPair{
				TransactionHash: xdr.Hash{0xaa},
				Result: xdr.InnerTransactionResult{
					FeeCharged: 150_000,
					Result: xdr.InnerTransactionResultResult{
						Code:    xdr.TransactionResultCodeTxFailed,
						Results: &ops,
					},
				},
			},
		},
	})
}

// txLevelResult is a transaction rejected before any operation ran.
func txLevelResult(t *testing.T, code xdr.TransactionResultCode) string {
	t.Helper()
	return mustB64(t, xdr.TransactionResult{
		FeeCharged: 100,
		Result:     xdr.TransactionResultResult{Code: code},
	})
}

func sym(s string) xdr.ScVal {
	v := xdr.ScSymbol(s)
	return xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &v}
}

func str(s string) xdr.ScVal {
	v := xdr.ScString(s)
	return xdr.ScVal{Type: xdr.ScValTypeScvString, Str: &v}
}

func hostError(typ xdr.ScErrorType, code xdr.ScErrorCode) xdr.ScError {
	return xdr.ScError{Type: typ, Code: &code}
}

func contractError(code uint32) xdr.ScError {
	c := xdr.Uint32(code)
	return xdr.ScError{Type: xdr.ScErrorTypeSceContract, ContractCode: &c}
}

// diagEvent encodes an xdr.DiagnosticEvent with the given topics.
func diagEvent(t *testing.T, withContract bool, topics ...xdr.ScVal) string {
	t.Helper()
	ev := xdr.DiagnosticEvent{
		InSuccessfulContractCall: false,
		Event: xdr.ContractEvent{
			Type: xdr.ContractEventTypeDiagnostic,
			Body: xdr.ContractEventBody{V: 0, V0: &xdr.ContractEventV0{
				Topics: topics,
				Data:   str("escalating error to VM trap from failed host function call"),
			}},
		},
	}
	if withContract {
		id := xdr.ContractId{0x11}
		ev.Event.ContractId = &id
	}
	return mustB64(t, ev)
}

// errorEvent is the host's ["error", Error(..)] diagnostic event.
func errorEvent(t *testing.T, e xdr.ScError) string {
	t.Helper()
	return diagEvent(t, true, sym("error"), xdr.ScVal{Type: xdr.ScValTypeScvError, Error: &e})
}

// fnCallEvent is an unrelated diagnostic event in the same stream.
func fnCallEvent(t *testing.T) string {
	t.Helper()
	id := xdr.ContractId{0x11}
	addr := xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeContract, ContractId: &id}
	return diagEvent(t, false, sym("fn_call"), xdr.ScVal{Type: xdr.ScValTypeScvAddress, Address: &addr}, sym("transfer"))
}

// unknownTypeErrorEvent is an error event whose ScErrorType (12) and code (3)
// do not exist in the current XDR, so no SDK can encode it. It is built from
// a real Budget/ExceededLimit event by rewriting the ScError bytes.
func unknownTypeErrorEvent(t *testing.T) string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(errorEvent(t, hostError(xdr.ScErrorTypeSceBudget, xdr.ScErrorCodeScecExceededLimit)))
	if err != nil {
		t.Fatal(err)
	}
	old := []byte{0, 0, 0, 2, 0, 0, 0, 7, 0, 0, 0, 5}
	repl := []byte{0, 0, 0, 2, 0, 0, 0, 12, 0, 0, 0, 3}
	if bytes.Count(raw, old) != 1 {
		t.Fatalf("expected exactly one ScError in the fixture")
	}
	return base64.StdEncoding.EncodeToString(bytes.Replace(raw, old, repl, 1))
}

// rawResult hand-builds a TransactionResult prefix with codes no SDK can
// encode (they are outside the XDR enums).
func rawResult(txCode int32, ihfCode *int32) string {
	var b []byte
	put := func(v uint32) { b = binary.BigEndian.AppendUint32(b, v) }
	put(0)
	put(100) // feeCharged
	put(uint32(txCode))
	if ihfCode != nil {
		put(1)  // one operation result
		put(0)  // opINNER
		put(24) // INVOKE_HOST_FUNCTION
		put(uint32(*ihfCode))
	}
	put(0) // ext
	return base64.StdEncoding.EncodeToString(b)
}

func TestClassify(t *testing.T) {
	trapped := invokeResult(t, xdr.InvokeHostFunctionResultCodeInvokeHostFunctionTrapped)
	unknownIHF := int32(-9)

	tests := []struct {
		name     string
		result   string
		events   []string
		wantCat  failurecategory.Category
		wantCode string
	}{
		// budget_exceeded
		{
			name:     "resource limit exceeded with budget host error",
			result:   invokeResult(t, xdr.InvokeHostFunctionResultCodeInvokeHostFunctionResourceLimitExceeded),
			events:   []string{fnCallEvent(t), errorEvent(t, hostError(xdr.ScErrorTypeSceBudget, xdr.ScErrorCodeScecExceededLimit))},
			wantCat:  failurecategory.BudgetExceeded,
			wantCode: "Error(Budget, ExceededLimit)",
		},
		{
			name:     "resource limit exceeded without diagnostic events",
			result:   invokeResult(t, xdr.InvokeHostFunctionResultCodeInvokeHostFunctionResourceLimitExceeded),
			wantCat:  failurecategory.BudgetExceeded,
			wantCode: "INVOKE_HOST_FUNCTION_RESOURCE_LIMIT_EXCEEDED",
		},
		{
			name:     "insufficient refundable fee",
			result:   invokeResult(t, xdr.InvokeHostFunctionResultCodeInvokeHostFunctionInsufficientRefundableFee),
			wantCat:  failurecategory.BudgetExceeded,
			wantCode: "INVOKE_HOST_FUNCTION_INSUFFICIENT_REFUNDABLE_FEE",
		},
		{
			name:     "trapped on a budget host error",
			result:   trapped,
			events:   []string{errorEvent(t, hostError(xdr.ScErrorTypeSceBudget, xdr.ScErrorCodeScecExceededLimit))},
			wantCat:  failurecategory.BudgetExceeded,
			wantCode: "Error(Budget, ExceededLimit)",
		},
		// contract_error
		{
			name:     "contract returned its own error code",
			result:   trapped,
			events:   []string{fnCallEvent(t), errorEvent(t, contractError(4))},
			wantCat:  failurecategory.ContractError,
			wantCode: "Error(Contract, #4)",
		},
		{
			name:     "fee-bump wrapped contract error",
			result:   feeBumpResult(t, xdr.InvokeHostFunctionResultCodeInvokeHostFunctionTrapped),
			events:   []string{errorEvent(t, contractError(1))},
			wantCat:  failurecategory.ContractError,
			wantCode: "Error(Contract, #1)",
		},
		// auth_failure
		{
			name:     "authorization failed",
			result:   trapped,
			events:   []string{errorEvent(t, hostError(xdr.ScErrorTypeSceAuth, xdr.ScErrorCodeScecInvalidAction))},
			wantCat:  failurecategory.AuthFailure,
			wantCode: "Error(Auth, InvalidAction)",
		},
		// storage
		{
			name:     "missing storage entry",
			result:   trapped,
			events:   []string{errorEvent(t, hostError(xdr.ScErrorTypeSceStorage, xdr.ScErrorCodeScecMissingValue))},
			wantCat:  failurecategory.Storage,
			wantCode: "Error(Storage, MissingValue)",
		},
		{
			name:     "archived entry in the footprint",
			result:   invokeResult(t, xdr.InvokeHostFunctionResultCodeInvokeHostFunctionEntryArchived),
			wantCat:  failurecategory.Storage,
			wantCode: "INVOKE_HOST_FUNCTION_ENTRY_ARCHIVED",
		},
		// wasm_trap
		{
			name:     "wasm trapped",
			result:   trapped,
			events:   []string{errorEvent(t, hostError(xdr.ScErrorTypeSceWasmVm, xdr.ScErrorCodeScecInvalidAction))},
			wantCat:  failurecategory.WasmTrap,
			wantCode: "Error(WasmVm, InvalidAction)",
		},
		// precedence
		{
			name:   "last error event wins over one caught by try_call",
			result: trapped,
			events: []string{
				errorEvent(t, contractError(7)),
				fnCallEvent(t),
				errorEvent(t, hostError(xdr.ScErrorTypeSceAuth, xdr.ScErrorCodeScecInvalidAction)),
			},
			wantCat:  failurecategory.AuthFailure,
			wantCode: "Error(Auth, InvalidAction)",
		},
		{
			name:     "host error decides without a transaction result",
			events:   []string{errorEvent(t, contractError(9))},
			wantCat:  failurecategory.ContractError,
			wantCode: "Error(Contract, #9)",
		},
		// other: known but uncategorised
		{
			name:     "value host error",
			result:   trapped,
			events:   []string{errorEvent(t, hostError(xdr.ScErrorTypeSceValue, xdr.ScErrorCodeScecUnexpectedType))},
			wantCat:  failurecategory.Other,
			wantCode: "Error(Value, UnexpectedType)",
		},
		{
			name:     "trapped without diagnostic events",
			result:   trapped,
			events:   []string{fnCallEvent(t)},
			wantCat:  failurecategory.Other,
			wantCode: "INVOKE_HOST_FUNCTION_TRAPPED",
		},
		{
			name:     "malformed host function",
			result:   invokeResult(t, xdr.InvokeHostFunctionResultCodeInvokeHostFunctionMalformed),
			wantCat:  failurecategory.Other,
			wantCode: "INVOKE_HOST_FUNCTION_MALFORMED",
		},
		{
			name:     "transaction-level failure",
			result:   txLevelResult(t, xdr.TransactionResultCodeTxSorobanInvalid),
			wantCat:  failurecategory.Other,
			wantCode: "txSOROBAN_INVALID",
		},
		// other: unknown values keep the raw number
		{
			name:     "unknown host error type",
			result:   trapped,
			events:   []string{unknownTypeErrorEvent(t)},
			wantCat:  failurecategory.Other,
			wantCode: "Error(#12, #3)",
		},
		{
			name:     "unknown invoke host function result code",
			result:   rawResult(-1, &unknownIHF),
			wantCat:  failurecategory.Other,
			wantCode: "INVOKE_HOST_FUNCTION(#-9)",
		},
		{
			name:     "unknown transaction result code",
			result:   rawResult(-42, nil),
			wantCat:  failurecategory.Other,
			wantCode: "tx(#-42)",
		},
		// other: undecodable
		{
			name:     "undecodable payloads",
			result:   "not base64!",
			events:   []string{"AAAA", "%%%"},
			wantCat:  failurecategory.Other,
			wantCode: CodeUnknown,
		},
		{
			name:     "error topic whose value is not an ScError",
			result:   "",
			events:   []string{diagEvent(t, true, sym("error"), str("boom"))},
			wantCat:  failurecategory.Other,
			wantCode: CodeUnknown,
		},
		{
			name:     "nothing at all",
			wantCat:  failurecategory.Other,
			wantCode: CodeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.result, tt.events)
			if got.Category != tt.wantCat || got.Code != tt.wantCode {
				t.Fatalf("Classify() = {%s %q}, want {%s %q}", got.Category, got.Code, tt.wantCat, tt.wantCode)
			}
		})
	}
}

// TestClassifyCoversEveryCategory guards the table above: each category must
// have at least one fixture.
func TestClassifyCoversEveryCategory(t *testing.T) {
	trapped := invokeResult(t, xdr.InvokeHostFunctionResultCodeInvokeHostFunctionTrapped)
	seen := map[failurecategory.Category]bool{}
	for _, ev := range [][]string{
		{errorEvent(t, hostError(xdr.ScErrorTypeSceBudget, xdr.ScErrorCodeScecExceededLimit))},
		{errorEvent(t, contractError(1))},
		{errorEvent(t, hostError(xdr.ScErrorTypeSceAuth, xdr.ScErrorCodeScecInvalidAction))},
		{errorEvent(t, hostError(xdr.ScErrorTypeSceStorage, xdr.ScErrorCodeScecMissingValue))},
		{errorEvent(t, hostError(xdr.ScErrorTypeSceWasmVm, xdr.ScErrorCodeScecInvalidAction))},
		{errorEvent(t, hostError(xdr.ScErrorTypeSceCrypto, xdr.ScErrorCodeScecInvalidInput))},
	} {
		seen[Classify(trapped, ev).Category] = true
	}
	for _, c := range failurecategory.All() {
		if !seen[c] {
			t.Errorf("no fixture produces category %q", c)
		}
	}
}

// TestClassifyNeverPanicsOnTruncatedInput feeds every prefix of real fixtures
// to the classifier: it must always return a defined category and a raw code.
func TestClassifyNeverPanicsOnTruncatedInput(t *testing.T) {
	result, _ := base64.StdEncoding.DecodeString(feeBumpResult(t, xdr.InvokeHostFunctionResultCodeInvokeHostFunctionTrapped))
	event, _ := base64.StdEncoding.DecodeString(errorEvent(t, contractError(3)))
	for i := 0; i <= len(result); i++ {
		for _, j := range []int{0, len(event) / 2, len(event)} {
			got := Classify(
				base64.StdEncoding.EncodeToString(result[:i]),
				[]string{base64.StdEncoding.EncodeToString(event[:j])},
			)
			if !got.Category.Valid() || got.Code == "" {
				t.Fatalf("prefix %d/%d: got %+v", i, j, got)
			}
		}
	}
}

func FuzzClassify(f *testing.F) {
	f.Add(rawResult(-1, nil), "AAAA")
	f.Add("", "")
	f.Fuzz(func(t *testing.T, result, event string) {
		got := Classify(result, []string{event})
		if !got.Category.Valid() || got.Code == "" {
			t.Fatalf("got %+v", got)
		}
	})
}
