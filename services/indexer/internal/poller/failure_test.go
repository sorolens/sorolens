package poller

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// Issue #806: a FAILED invocation is stored with its failure category and raw
// code; any other status leaves both empty. Fixtures are encoded with the
// go-stellar-sdk, as getTransaction returns them.

func failedTxResultXDR(t *testing.T) string {
	t.Helper()
	ops := []xdr.OperationResult{{
		Code: xdr.OperationResultCodeOpInner,
		Tr: &xdr.OperationResultTr{
			Type: xdr.OperationTypeInvokeHostFunction,
			InvokeHostFunctionResult: &xdr.InvokeHostFunctionResult{
				Code: xdr.InvokeHostFunctionResultCodeInvokeHostFunctionTrapped,
			},
		},
	}}
	s, err := xdr.MarshalBase64(xdr.TransactionResult{
		FeeCharged: 1000,
		Result:     xdr.TransactionResultResult{Code: xdr.TransactionResultCodeTxFailed, Results: &ops},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func contractErrorEventXDR(t *testing.T, code uint32) string {
	t.Helper()
	topic := xdr.ScSymbol("error")
	cc := xdr.Uint32(code)
	msg := xdr.ScString("escalating Ok(ScErrorType::Contract) frame-exit to Err")
	s, err := xdr.MarshalBase64(xdr.DiagnosticEvent{
		Event: xdr.ContractEvent{
			Type: xdr.ContractEventTypeDiagnostic,
			Body: xdr.ContractEventBody{V: 0, V0: &xdr.ContractEventV0{
				Topics: []xdr.ScVal{
					{Type: xdr.ScValTypeScvSymbol, Sym: &topic},
					{Type: xdr.ScValTypeScvError, Error: &xdr.ScError{Type: xdr.ScErrorTypeSceContract, ContractCode: &cc}},
				},
				Data: xdr.ScVal{Type: xdr.ScValTypeScvString, Str: &msg},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPoller_ClassifiesFailedInvocation(t *testing.T) {
	t.Parallel()
	f := loadDiagnosticsFixture(t)

	inv := runDiagnosticsPass(t, testConfig(), f.TransferTopics, &TransactionResult{
		Status:              "FAILED",
		Ledger:              499100,
		ResultXDR:           failedTxResultXDR(t),
		DiagnosticEventsXDR: append(append([]string{}, f.GetTransaction.DiagnosticEventsXdr...), contractErrorEventXDR(t, 4)),
	})

	if inv.FailureCategory != "contract_error" || inv.FailureCode != "Error(Contract, #4)" {
		t.Fatalf("failure = %q / %q, want contract_error / Error(Contract, #4)", inv.FailureCategory, inv.FailureCode)
	}
	// Classification must not disturb the diagnostics already decoded.
	if inv.FunctionName != "transfer" {
		t.Fatalf("function name: got %q, want transfer", inv.FunctionName)
	}
}

func TestPoller_FailedInvocationWithoutDetailIsOther(t *testing.T) {
	t.Parallel()
	f := loadDiagnosticsFixture(t)

	inv := runDiagnosticsPass(t, testConfig(), f.TransferTopics, &TransactionResult{
		Status:    "FAILED",
		Ledger:    499100,
		ResultXDR: "not-xdr",
	})
	if inv.FailureCategory != "other" || inv.FailureCode != "UNKNOWN" {
		t.Fatalf("undecodable failure must be kept as other/UNKNOWN, got %q / %q", inv.FailureCategory, inv.FailureCode)
	}
}

func TestPoller_SuccessfulInvocationHasNoFailureCategory(t *testing.T) {
	t.Parallel()
	f := loadDiagnosticsFixture(t)

	inv := runDiagnosticsPass(t, testConfig(), f.TransferTopics, &TransactionResult{
		Status:              "SUCCESS",
		Ledger:              499100,
		DiagnosticEventsXDR: f.GetTransaction.DiagnosticEventsXdr,
	})
	if inv.FailureCategory != "" || inv.FailureCode != "" {
		t.Fatalf("successful invocation got failure %q / %q", inv.FailureCategory, inv.FailureCode)
	}
}
