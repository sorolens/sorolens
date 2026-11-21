package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *postgresStore) RecordBatchUsage(ctx context.Context, records []APIKeyUsageRecord) error {
	if len(records) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, rec := range records {
		dateOnly := time.Date(rec.Date.Year(), rec.Date.Month(), rec.Date.Day(), 0, 0, 0, 0, time.UTC)
		batch.Queue(`
			INSERT INTO api_key_usage (key_id, date, endpoint_group, request_count, updated_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (key_id, date, endpoint_group)
			DO UPDATE SET
				request_count = api_key_usage.request_count + EXCLUDED.request_count,
				updated_at = NOW()`,
			rec.KeyID, dateOnly, rec.EndpointGroup, rec.RequestCount,
		)
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(records); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("record usage batch item %d: %w", i, err)
		}
	}
	return nil
}

func (s *postgresStore) GetMonthlyKeyUsage(ctx context.Context, keyID string, year int, month time.Month) (int64, error) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	var total int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(request_count), 0)
		FROM api_key_usage
		WHERE key_id = $1 AND date >= $2 AND date < $3`,
		keyID, start, end,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get monthly key usage: %w", err)
	}
	return total, nil
}

func (s *postgresStore) GetKeyUsage(ctx context.Context, keyID string, from, to time.Time) (*KeyUsageSummary, error) {
	key, err := s.GetAPIKey(ctx, keyID)
	if err != nil {
		return nil, err
	}

	fromDate := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	toDate := time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 999999999, time.UTC)

	rows, err := s.pool.Query(ctx, `
		SELECT date, endpoint_group, request_count
		FROM api_key_usage
		WHERE key_id = $1 AND date >= $2 AND date <= $3
		ORDER BY date ASC, endpoint_group ASC`,
		keyID, fromDate, toDate,
	)
	if err != nil {
		return nil, fmt.Errorf("query key usage: %w", err)
	}
	defer rows.Close()

	totalRequests := int64(0)
	byGroup := make(map[string]int64)
	timelineMap := make(map[string]map[string]int64)
	timelineOrder := []string{}

	for rows.Next() {
		var (
			d     time.Time
			group string
			cnt   int64
		)
		if err := rows.Scan(&d, &group, &cnt); err != nil {
			return nil, fmt.Errorf("scan key usage: %w", err)
		}

		totalRequests += cnt
		byGroup[group] += cnt

		dateKey := d.Format("2006-01-02")
		if _, exists := timelineMap[dateKey]; !exists {
			timelineMap[dateKey] = make(map[string]int64)
			timelineOrder = append(timelineOrder, dateKey)
		}
		timelineMap[dateKey][group] += cnt
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate key usage: %w", err)
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
	monthUsed, err := s.GetMonthlyKeyUsage(ctx, keyID, now.Year(), now.Month())
	if err != nil {
		monthUsed = 0
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
