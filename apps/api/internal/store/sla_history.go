package store

import "time"

// HistoryRange returns the half-open UTC bounds covering the last `months`
// calendar months ending with the month that contains now. It is the window a
// caller should query before calling ComputeMonthlySLAHistory, so the buckets
// and the fetched data always line up.
func HistoryRange(months int, now time.Time) (time.Time, time.Time) {
	if months <= 0 {
		months = 12
	}
	base := time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	return base.AddDate(0, -(months - 1), 0), base.AddDate(0, 1, 0)
}

// ComputeMonthlySLAHistory buckets raw watchdog data into the last `months`
// calendar months ending with the month containing now, oldest first.
//
// It reuses ComputeMonthlySLA for each bucket, so a month in the history is
// byte-for-byte the same calculation as that month's standalone report — a
// trend chart can never disagree with the report it is charting.
func ComputeMonthlySLAHistory(contractID string, months int, now time.Time, checks []HealthCheck, alerts []ContractAlert) []MonthlySLA {
	if months <= 0 {
		months = 12
	}
	base := time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)

	out := make([]MonthlySLA, 0, months)
	for i := months - 1; i >= 0; i-- {
		month := base.AddDate(0, -i, 0).Format("2006-01")
		out = append(out, ComputeMonthlySLA(contractID, month, checks, alerts))
	}
	return out
}
