package store

import "strings"

// Sortable columns for the contracts list endpoint. The set is deliberately
// small: the handler validates client input against it and the store maps it to
// a fixed SQL expression, so the column can never be interpolated from raw user
// input.
const (
	// ContractSortAddedAt orders by contracts.added_at (when tracking began).
	ContractSortAddedAt = "added_at"
	// ContractSortLastActivity orders by the most recent indexed activity
	// (the latest event or invocation ledger close time). Contracts with no
	// indexed activity sort as the Unix epoch.
	ContractSortLastActivity = "last_activity"
	// ContractSortEventsCount orders by the number of indexed events.
	ContractSortEventsCount = "events_count"
)

// Sort directions accepted by list endpoints.
const (
	SortAsc  = "asc"
	SortDesc = "desc"
)

// ValidContractSort reports whether s is a whitelisted sort column for the
// contracts list endpoint.
func ValidContractSort(s string) bool {
	switch s {
	case ContractSortAddedAt, ContractSortLastActivity, ContractSortEventsCount:
		return true
	}
	return false
}

// ValidContractOrder reports whether order is a recognised sort direction.
func ValidContractOrder(order string) bool {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case SortAsc, SortDesc:
		return true
	}
	return false
}

// NormalizeContractSort canonicalises a requested sort column and direction.
// Unknown or empty values fall back to the endpoint defaults (added_at,
// descending). Callers that accept user input should reject invalid values with
// ValidContractSort/ValidContractOrder before calling this, so a bad request is
// reported rather than silently defaulted.
func NormalizeContractSort(sort, order string) (column string, descending bool) {
	column = strings.ToLower(strings.TrimSpace(sort))
	if !ValidContractSort(column) {
		column = ContractSortAddedAt
	}
	descending = !strings.EqualFold(strings.TrimSpace(order), SortAsc)
	return column, descending
}
