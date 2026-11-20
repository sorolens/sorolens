// Package failure classifies why a Soroban invocation failed into a small set
// of operator-facing categories.
//
// # Inputs
//
// A failed invocation carries two decodable signals:
//
//   - the transaction result XDR, whose TransactionResultCode says whether the
//     transaction or the operation was rejected and, for a failed
//     invokeHostFunction operation, the InvokeHostFunctionResultCode;
//   - the host diagnostic events, which carry the ScVal::Error (SCError) the
//     host raised — a type (contract, wasm_vm, storage, budget, auth, ...) and
//     the code within that type.
//
// The host error is the actionable part, so it wins when both are present; the
// transaction and operation codes are the fallback.
//
// # Categories
//
// The categories are stable strings stored in invocations.failure_category and
// documented in docs/indexer.md: budget, contract, auth, storage, wasm_trap,
// value, crypto, events, object, context, transaction and other. Anything the
// decoder does not understand is kept as "other"; a failed invocation is never
// dropped, and the raw code is preserved in invocations.failure_code when it
// could be decoded (the original result XDR stays in invocations.result_xdr).
//
// Decoding is defensive, matching internal/scval and internal/callgraph: every
// read is bounds-checked and any mismatch yields no code rather than an error,
// so a malformed stream can never fail an indexing pass.
package failure

import "fmt"

// Failure categories. These strings are part of the API (the
// failure_category filter and the per-category stats map) and must not be
// renamed without a migration.
const (
	// Budget is a resource budget or refundable-fee limit: the host ran out
	// of CPU instructions, memory, or the declared resource fee.
	Budget = "budget"
	// Contract is a contract-defined error raised with panic/revert.
	Contract = "contract"
	// Auth is an authentication failure, either a contract-level auth error
	// or a transaction rejected for bad signatures.
	Auth = "auth"
	// Storage covers storage errors and archived ledger entries.
	Storage = "storage"
	// WasmTrap is a Wasm trap or Wasm VM error while executing contract code.
	WasmTrap = "wasm_trap"
	// Value is an error raised while converting or validating host values.
	Value = "value"
	// Crypto is a failure in the host cryptographic subsystem.
	Crypto = "crypto"
	// Events is a failure while emitting contract events.
	Events = "events"
	// Object is a failure in the host object subsystem.
	Object = "object"
	// Context is a host context error (re-entry, frame, or state misuse).
	Context = "context"
	// Transaction is a transaction-level rejection that is not tied to the
	// contract's execution (bad sequence number, insufficient fee, ...).
	Transaction = "transaction"
	// Other is everything the decoder could not attribute.
	Other = "other"
)

// Categories lists every category this package can produce, in documentation
// order. The API and the stats endpoint expose exactly these strings.
var Categories = []string{
	Budget, Contract, Auth, Storage, WasmTrap, Value, Crypto, Events, Object,
	Context, Transaction, Other,
}

// Failure is the classification of one failed invocation.
type Failure struct {
	// Category is one of the Category constants above.
	Category string
	// Code is the raw, human-readable error code the category was derived
	// from, for example "sce_storage:3", "invoke_host_function_trapped" or
	// "tx_bad_seq". It is empty when nothing could be decoded.
	Code string
}

// Classify derives the failure category and raw code from a transaction's
// result XDR and its base64 host diagnostic events. The caller only invokes it
// for failed invocations; a success decodes to the Other category with no code.
//
// It never returns an empty category: an undecodable failure is Other, so the
// invocation stays counted and queryable.
func Classify(resultXDR string, diagnosticEventsXDR []string) Failure {
	res, ok := decodeTransactionResultXDR(resultXDR)

	// The host error is the most specific signal: it distinguishes a contract
	// revert from a trap, a storage miss, an auth failure, and so on.
	if host, found := hostErrorFromDiagnostics(diagnosticEventsXDR); found {
		return Failure{
			Category: categoryForHostError(host.Type),
			Code:     fmt.Sprintf("%s:%d", scErrorTypeName(host.Type), host.Code),
		}
	}

	if !ok {
		return Failure{Category: Other}
	}

	if res.hasInvoke {
		if cat, known := invokeCategories[res.invokeCode]; known {
			return Failure{Category: cat, Code: invokeCodeName(res.invokeCode)}
		}
	}

	if name, known := txCodeNames[res.txCode]; known {
		return Failure{Category: categoryForTxCode(res.txCode), Code: name}
	}
	return Failure{Category: Other}
}

// categoryForHostError maps an SCErrorType discriminant onto a category.
func categoryForHostError(typ uint32) string {
	switch typ {
	case scErrorContract:
		return Contract
	case scErrorWasmVM:
		return WasmTrap
	case scErrorContext:
		return Context
	case scErrorStorage:
		return Storage
	case scErrorObject:
		return Object
	case scErrorCrypto:
		return Crypto
	case scErrorEvents:
		return Events
	case scErrorBudget:
		return Budget
	case scErrorValueType:
		return Value
	case scErrorAuth:
		return Auth
	default:
		return Other
	}
}

// categoryForTxCode maps a TransactionResultCode onto a category. Only the
// auth codes are specific enough to leave the generic transaction category.
func categoryForTxCode(code int32) string {
	switch code {
	case txBadAuth, txBadAuthExtra:
		return Auth
	case txInternalError, txNotSupported, txMalformed, txSuccess:
		return Other
	default:
		return Transaction
	}
}

// ---- raw code names --------------------------------------------------------

// scErrorTypeNames are the XDR SCErrorType names, used to build the raw code.
var scErrorTypeNames = map[uint32]string{
	scErrorContract:  "sce_contract",
	scErrorWasmVM:    "sce_wasm_vm",
	scErrorContext:   "sce_context",
	scErrorStorage:   "sce_storage",
	scErrorObject:    "sce_object",
	scErrorCrypto:    "sce_crypto",
	scErrorEvents:    "sce_events",
	scErrorBudget:    "sce_budget",
	scErrorValueType: "sce_value",
	scErrorAuth:      "sce_auth",
}

func scErrorTypeName(typ uint32) string {
	if name, ok := scErrorTypeNames[typ]; ok {
		return name
	}
	return fmt.Sprintf("sce_unknown_%d", typ)
}

// invokeCategories maps an InvokeHostFunctionResultCode onto a category.
var invokeCategories = map[int32]string{
	invokeHostMalformed:                 Other,
	invokeHostTrapped:                   WasmTrap,
	invokeHostResourceLimitExceeded:     Budget,
	invokeHostEntryArchived:             Storage,
	invokeHostInsufficientRefundableFee: Budget,
}

var invokeCodeNames = map[int32]string{
	invokeHostMalformed:                 "invoke_host_function_malformed",
	invokeHostTrapped:                   "invoke_host_function_trapped",
	invokeHostResourceLimitExceeded:     "invoke_host_function_resource_limit_exceeded",
	invokeHostEntryArchived:             "invoke_host_function_entry_archived",
	invokeHostInsufficientRefundableFee: "invoke_host_function_insufficient_refundable_fee",
}

func invokeCodeName(code int32) string {
	if name, ok := invokeCodeNames[code]; ok {
		return name
	}
	return fmt.Sprintf("invoke_host_function_unknown_%d", code)
}

// txCodeNames are the XDR TransactionResultCode names, used to build the raw
// code for a transaction-level rejection.
var txCodeNames = map[int32]string{
	txFailed:            "tx_failed",
	txTooEarly:          "tx_too_early",
	txTooLate:           "tx_too_late",
	txMissingOperation:  "tx_missing_operation",
	txBadSeq:            "tx_bad_seq",
	txBadAuth:           "tx_bad_auth",
	txInsufficientBal:   "tx_insufficient_balance",
	txNoAccount:         "tx_no_account",
	txInsufficientFee:   "tx_insufficient_fee",
	txBadAuthExtra:      "tx_bad_auth_extra",
	txInternalError:     "tx_internal_error",
	txNotSupported:      "tx_not_supported",
	txFeeBumpInner:      "tx_fee_bump_inner_failed",
	txBadSponsorship:    "tx_bad_sponsorship",
	txBadMinSeqAgeOrGap: "tx_bad_min_seq_age_or_gap",
	txMalformed:         "tx_malformed",
	txSorobanInvalid:    "tx_soroban_invalid",
}
