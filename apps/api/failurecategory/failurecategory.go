// Package failurecategory defines the categories a failed Soroban invocation is
// sorted into (issue #806), so operators can see why invocations fail instead
// of only how many do.
//
// The indexer assigns a category when it stores a failed invocation
// (services/indexer/internal/failure) and the API validates the
// failure_category filter and builds the per-category stats from the same
// list. Like apps/api/rulelang, this is a dependency-free leaf package so the
// indexer can import it without pulling in the API.
//
// The mapping from Soroban host errors to categories is documented in
// docs/failure-categories.md.
package failurecategory

// Category is the stored value of invocations.failure_category.
type Category string

const (
	// BudgetExceeded: the invocation ran out of the CPU, memory or other
	// resources it declared (ScErrorType Budget, or the operation result
	// INVOKE_HOST_FUNCTION_RESOURCE_LIMIT_EXCEEDED /
	// INVOKE_HOST_FUNCTION_INSUFFICIENT_REFUNDABLE_FEE).
	BudgetExceeded Category = "budget_exceeded"
	// ContractError: the contract returned one of its own error codes
	// (ScErrorType Contract).
	ContractError Category = "contract_error"
	// AuthFailure: authorization failed (ScErrorType Auth).
	AuthFailure Category = "auth_failure"
	// Storage: a ledger entry was missing, archived or could not be written
	// (ScErrorType Storage, or INVOKE_HOST_FUNCTION_ENTRY_ARCHIVED).
	Storage Category = "storage"
	// WasmTrap: the contract's Wasm trapped, for example on a panic or an
	// unreachable instruction (ScErrorType WasmVm).
	WasmTrap Category = "wasm_trap"
	// Other: any failure the categories above do not cover, including
	// unknown or undecodable errors. The raw code is always kept alongside.
	Other Category = "other"
)

// all lists every category in documentation order.
var all = []Category{BudgetExceeded, ContractError, AuthFailure, Storage, WasmTrap, Other}

// All returns every category in a stable order. The returned slice is a copy.
func All() []Category {
	out := make([]Category, len(all))
	copy(out, all)
	return out
}

// Valid reports whether c is one of the defined categories.
func (c Category) Valid() bool {
	for _, k := range all {
		if c == k {
			return true
		}
	}
	return false
}

// Parse returns the category named s and whether it is a defined category.
// Matching is exact: category values are lower-case identifiers.
func Parse(s string) (Category, bool) {
	c := Category(s)
	return c, c.Valid()
}
