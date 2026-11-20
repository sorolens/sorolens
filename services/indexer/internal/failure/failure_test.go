package failure

import (
	"encoding/base64"
	"encoding/binary"
	"testing"
)

// The fixtures below are hand-built XDR following Stellar-transaction.x and
// Stellar-contract.x. Building them byte-by-byte keeps the decoder honest: a
// change to an XDR field order breaks these tests rather than silently
// misclassifying production data.

func be32(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

func beI32(v int32) []byte { return be32(uint32(v)) }

func be64(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func join(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// transactionResult builds a TransactionResult with the given code. When the
// code is txFAILED, ops are the OperationResult bodies.
func transactionResult(code int32, ops ...[]byte) string {
	body := join(be64(0), beI32(code)) // feeCharged, result code
	if code == txFailed {
		body = append(body, be32(uint32(len(ops)))...)
		for _, op := range ops {
			body = append(body, op...)
		}
	}
	return b64(body)
}

// feeBumpResult wraps an inner TransactionResult in a fee-bump result.
func feeBumpResult(inner string) string {
	raw, err := base64.StdEncoding.DecodeString(inner)
	if err != nil {
		panic(err)
	}
	return b64(join(be64(0), beI32(txFeeBumpInner), make([]byte, 32), raw))
}

// invokeHostOp builds an OperationResult for a failed invokeHostFunction
// operation carrying invokeCode.
func invokeHostOp(invokeCode int32) []byte {
	return join(beI32(opInner), be32(opInvokeHostFunction), beI32(invokeCode))
}

// scSymbol encodes an ScVal::Symbol.
func scSymbol(s string) []byte {
	padded := s
	if pad := len(s) % 4; pad != 0 {
		padded += string(make([]byte, 4-pad))
	}
	return join(be32(15), be32(uint32(len(s))), []byte(padded))
}

// scErrorValue encodes an ScVal::Error(SCError{Type, Code}).
func scErrorValue(typ, code uint32) []byte {
	return join(be32(scValTypeError), be32(typ), be32(code))
}

// scString encodes an ScVal::String.
func scString(s string) []byte {
	padded := s
	if pad := len(s) % 4; pad != 0 {
		padded += string(make([]byte, 4-pad))
	}
	return join(be32(14), be32(uint32(len(s))), []byte(padded))
}

// diagnosticEvent builds an xdr.DiagnosticEvent carrying the given topics and a
// constant string payload.
func diagnosticEvent(topics ...[]byte) string {
	body := join(
		be32(0), // inSuccessfulContractCall
		be32(0), // ContractEvent.ext (void)
		be32(0), // ContractEvent.contractID absent
		be32(2), // ContractEventType diagnostic
		be32(0), // body union: v0
		be32(uint32(len(topics))),
	)
	for _, topic := range topics {
		body = append(body, topic...)
	}
	body = append(body, scString("invocation failed")...)
	return b64(body)
}

// hostErrorEvent is the diagnostic event the host emits for an error: its
// topics are [error, ScVal::Error(SCError)].
func hostErrorEvent(typ, code uint32) string {
	return diagnosticEvent(scSymbol("error"), scErrorValue(typ, code))
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name      string
		resultXDR string
		events    []string
		wantCat   string
		wantCode  string
	}{
		{
			name:      "contract error from host",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostTrapped)),
			events:    []string{hostErrorEvent(scErrorContract, 5)},
			wantCat:   Contract,
			wantCode:  "sce_contract:5",
		},
		{
			name:      "auth failure from host",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostTrapped)),
			events:    []string{hostErrorEvent(scErrorAuth, 2)},
			wantCat:   Auth,
			wantCode:  "sce_auth:2",
		},
		{
			name:      "storage error from host",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostTrapped)),
			events:    []string{hostErrorEvent(scErrorStorage, 3)},
			wantCat:   Storage,
			wantCode:  "sce_storage:3",
		},
		{
			name:      "budget exceeded from host",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostResourceLimitExceeded)),
			events:    []string{hostErrorEvent(scErrorBudget, 5)},
			wantCat:   Budget,
			wantCode:  "sce_budget:5",
		},
		{
			name:      "wasm trap from host",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostTrapped)),
			events:    []string{hostErrorEvent(scErrorWasmVM, 6)},
			wantCat:   WasmTrap,
			wantCode:  "sce_wasm_vm:6",
		},
		{
			name:      "context error from host",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostTrapped)),
			events:    []string{hostErrorEvent(scErrorContext, 0)},
			wantCat:   Context,
			wantCode:  "sce_context:0",
		},
		{
			name:      "object error from host",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostTrapped)),
			events:    []string{hostErrorEvent(scErrorObject, 8)},
			wantCat:   Object,
			wantCode:  "sce_object:8",
		},
		{
			name:      "crypto error from host",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostTrapped)),
			events:    []string{hostErrorEvent(scErrorCrypto, 1)},
			wantCat:   Crypto,
			wantCode:  "sce_crypto:1",
		},
		{
			name:      "events error from host",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostTrapped)),
			events:    []string{hostErrorEvent(scErrorEvents, 9)},
			wantCat:   Events,
			wantCode:  "sce_events:9",
		},
		{
			name:      "value error from host",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostTrapped)),
			events:    []string{hostErrorEvent(scErrorValueType, 8)},
			wantCat:   Value,
			wantCode:  "sce_value:8",
		},
		{
			name:      "unknown host error type stays other",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostTrapped)),
			events:    []string{hostErrorEvent(42, 1)},
			wantCat:   Other,
			wantCode:  "sce_unknown_42:1",
		},
		{
			name:      "trapped without diagnostics is a wasm trap",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostTrapped)),
			wantCat:   WasmTrap,
			wantCode:  "invoke_host_function_trapped",
		},
		{
			name:      "resource limit exceeded without diagnostics is budget",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostResourceLimitExceeded)),
			wantCat:   Budget,
			wantCode:  "invoke_host_function_resource_limit_exceeded",
		},
		{
			name:      "archived entry is storage",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostEntryArchived)),
			wantCat:   Storage,
			wantCode:  "invoke_host_function_entry_archived",
		},
		{
			name:      "insufficient refundable fee is budget",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostInsufficientRefundableFee)),
			wantCat:   Budget,
			wantCode:  "invoke_host_function_insufficient_refundable_fee",
		},
		{
			name:      "malformed host function result is other",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostMalformed)),
			wantCat:   Other,
			wantCode:  "invoke_host_function_malformed",
		},
		{
			name:      "unknown invoke code keeps its raw code",
			resultXDR: transactionResult(txFailed, invokeHostOp(-99)),
			wantCat:   Transaction,
			wantCode:  "tx_failed",
		},
		{
			name:      "bad sequence is a transaction failure",
			resultXDR: transactionResult(txBadSeq),
			wantCat:   Transaction,
			wantCode:  "tx_bad_seq",
		},
		{
			name:      "transaction bad auth is auth",
			resultXDR: transactionResult(txBadAuth),
			wantCat:   Auth,
			wantCode:  "tx_bad_auth",
		},
		{
			name:      "extra signatures is auth",
			resultXDR: transactionResult(txBadAuthExtra),
			wantCat:   Auth,
			wantCode:  "tx_bad_auth_extra",
		},
		{
			name:      "insufficient fee is a transaction failure",
			resultXDR: transactionResult(txInsufficientFee),
			wantCat:   Transaction,
			wantCode:  "tx_insufficient_fee",
		},
		{
			name:      "fee bump inner failure is unwrapped",
			resultXDR: feeBumpResult(transactionResult(txBadSeq)),
			wantCat:   Transaction,
			wantCode:  "tx_bad_seq",
		},
		{
			name:      "fee bump inner host error keeps the host category",
			resultXDR: feeBumpResult(transactionResult(txFailed, invokeHostOp(invokeHostTrapped))),
			events:    []string{hostErrorEvent(scErrorContract, 7)},
			wantCat:   Contract,
			wantCode:  "sce_contract:7",
		},
		{
			name:      "undecodable result is other",
			resultXDR: "not base64 !!",
			wantCat:   Other,
		},
		{
			name:    "empty result is other",
			wantCat: Other,
		},
		{
			name:      "undecodable diagnostic event is ignored",
			resultXDR: transactionResult(txFailed, invokeHostOp(invokeHostTrapped)),
			events:    []string{"not base64 !!"},
			wantCat:   WasmTrap,
			wantCode:  "invoke_host_function_trapped",
		},
		{
			name:      "success has no code and stays other",
			resultXDR: transactionResult(txSuccess),
			wantCat:   Other,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.resultXDR, tc.events)
			if got.Category != tc.wantCat {
				t.Fatalf("category = %q, want %q", got.Category, tc.wantCat)
			}
			if got.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", got.Code, tc.wantCode)
			}
		})
	}
}

// TestClassifyAlwaysReturnsACategory guards the acceptance criterion that
// unknown errors are not dropped: every failed invocation must be countable.
func TestClassifyAlwaysReturnsACategory(t *testing.T) {
	known := map[string]bool{}
	for _, c := range Categories {
		known[c] = true
	}
	inputs := []string{"", "!!!!", "AAAA", transactionResult(txFailed), transactionResult(txMalformed)}
	for _, in := range inputs {
		if got := Classify(in, nil); !known[got.Category] {
			t.Fatalf("Classify(%q) category = %q, want one of %v", in, got.Category, Categories)
		}
	}
}
