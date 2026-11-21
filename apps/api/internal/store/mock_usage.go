package store

import (
	"context"
	"time"
)

func (m *MockStore) RecordBatchUsage(_ context.Context, records []APIKeyUsageRecord) error {
	m.auditMu.Lock()
	defer m.auditMu.Unlock()

	for _, rec := range records {
		dateOnly := time.Date(rec.Date.Year(), rec.Date.Month(), rec.Date.Day(), 0, 0, 0, 0, time.UTC)
		found := false
		for i, existing := range m.usageRecords {
			if existing.KeyID == rec.KeyID && existing.Date.Equal(dateOnly) && existing.EndpointGroup == rec.EndpointGroup {
				m.usageRecords[i].RequestCount += rec.RequestCount
				found = true
				break
			}
		}
		if !found {
			m.usageRecords = append(m.usageRecords, APIKeyUsageRecord{
				KeyID:         rec.KeyID,
				Date:          dateOnly,
				EndpointGroup: rec.EndpointGroup,
				RequestCount:  rec.RequestCount,
			})
		}
	}
	return nil
}

func (m *MockStore) GetMonthlyKeyUsage(_ context.Context, keyID string, year int, month time.Month) (int64, error) {
	m.auditMu.Lock()
	defer m.auditMu.Unlock()

	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	var total int64
	for _, rec := range m.usageRecords {
		if rec.KeyID == keyID && (rec.Date.Equal(start) || rec.Date.After(start)) && rec.Date.Before(end) {
			total += rec.RequestCount
		}
	}
	return total, nil
}

func (m *MockStore) GetKeyUsage(ctx context.Context, keyID string, from, to time.Time) (*KeyUsageSummary, error) {
	key, err := m.GetAPIKey(ctx, keyID)
	if err != nil {
		return nil, err
	}

	m.auditMu.Lock()
	defer m.auditMu.Unlock()

	fromDate := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	toDate := time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 999999999, time.UTC)

	totalRequests := int64(0)
	byGroup := make(map[string]int64)
	timelineMap := make(map[string]map[string]int64)
	timelineOrder := []string{}

	for _, rec := range m.usageRecords {
		if rec.KeyID != keyID {
			continue
		}
		if rec.Date.Before(fromDate) || rec.Date.After(toDate) {
			continue
		}

		totalRequests += rec.RequestCount
		byGroup[rec.EndpointGroup] += rec.RequestCount

		dateKey := rec.Date.Format("2006-01-02")
		if _, exists := timelineMap[dateKey]; !exists {
			timelineMap[dateKey] = make(map[string]int64)
			timelineOrder = append(timelineOrder, dateKey)
		}
		timelineMap[dateKey][rec.EndpointGroup] += rec.RequestCount
	}

	var timeline []UsageTimelinePoint
	for _, dt := range timelineOrder {
		grpMap := timelineMap[dt]
		dayTotal := int64(0)
		for _, c := range grpMap {
			dayTotal += c
		}
		timeline = append(timeline, UsageTimelinePoint{
			Date:          dt,
			TotalRequests: dayTotal,
			ByGroup:       grpMap,
		})
	}
	if timeline == nil {
		timeline = []UsageTimelinePoint{}
	}

	now := time.Now().UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, 0)
	monthUsed := int64(0)
	for _, rec := range m.usageRecords {
		if rec.KeyID == keyID && (rec.Date.Equal(startOfMonth) || rec.Date.After(startOfMonth)) && rec.Date.Before(endOfMonth) {
			monthUsed += rec.RequestCount
		}
	}

	var monthRemaining *int64
	if key.MonthlyQuota != nil {
		rem := int64(*key.MonthlyQuota) - monthUsed
		if rem < 0 {
			rem = 0
		}
		monthRemaining = &rem
	}

	return &KeyUsageSummary{
		KeyID:          keyID,
		From:           from,
		To:             to,
		TotalRequests:  totalRequests,
		ByGroup:        byGroup,
		Timeline:       timeline,
		MonthlyQuota:   key.MonthlyQuota,
		MonthUsed:      monthUsed,
		MonthRemaining: monthRemaining,
	}, nil
}
