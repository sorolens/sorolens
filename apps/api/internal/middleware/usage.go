package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

var (
	usageMeterRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "sorolens_api_usage_requests_total",
		Help: "Total API requests tracked by usage metering.",
	}, []string{"endpoint_group", "quota_status"})

	usageMeterErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sorolens_api_usage_metering_errors_total",
		Help: "Errors encountered by usage metering when querying Redis.",
	})
)

// UsageMeterCollectors returns prometheus collectors for usage metering.
func UsageMeterCollectors() []prometheus.Collector {
	return []prometheus.Collector{usageMeterRequests, usageMeterErrors}
}

// ClassifyEndpointGroup categorizes an API path into a low-cardinality endpoint group.
func ClassifyEndpointGroup(path string) string {
	trimmed := path
	for _, prefix := range []string{"/api/v1/", "/api/v2/", "/api/", "/"} {
		if strings.HasPrefix(trimmed, prefix) {
			trimmed = strings.TrimPrefix(trimmed, prefix)
			break
		}
	}
	trimmed = strings.TrimPrefix(trimmed, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "other"
	}

	switch parts[0] {
	case "contracts":
		if len(parts) >= 3 {
			switch parts[2] {
			case "events":
				return "events"
			case "invocations":
				return "invocations"
			}
		}
		return "contracts"
	case "events":
		return "events"
	case "invocations":
		return "invocations"
	case "alerts", "subscriptions":
		return "alerts"
	case "admin", "keys", "api-keys":
		return "admin"
	case "stats", "storage", "forecast":
		return "stats"
	case "usage":
		return "usage"
	default:
		return "other"
	}
}

// UsageFlusher buffers in-flight request counts and flushes them to the UsageStore in batches.
type UsageFlusher struct {
	store    store.UsageStore
	mu       sync.Mutex
	buffer   map[string]int64 // key: "key_id|YYYY-MM-DD|group" -> count
	stopCh   chan struct{}
	interval time.Duration
}

// NewUsageFlusher creates a new flusher. Call Start() to run the background flush worker.
func NewUsageFlusher(s store.UsageStore, interval time.Duration) *UsageFlusher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &UsageFlusher{
		store:    s,
		buffer:   make(map[string]int64),
		stopCh:   make(chan struct{}),
		interval: interval,
	}
}

// Record queues an increment for keyID, date, and endpointGroup.
func (f *UsageFlusher) Record(keyID, date, group string) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	bufKey := keyID + "|" + date + "|" + group
	f.buffer[bufKey]++
}

// Flush synchronously writes all buffered records to the UsageStore.
func (f *UsageFlusher) Flush(ctx context.Context) error {
	if f == nil || f.store == nil {
		return nil
	}

	f.mu.Lock()
	if len(f.buffer) == 0 {
		f.mu.Unlock()
		return nil
	}
	snapshot := f.buffer
	f.buffer = make(map[string]int64)
	f.mu.Unlock()

	var records []store.APIKeyUsageRecord
	for bufKey, count := range snapshot {
		parts := strings.Split(bufKey, "|")
		if len(parts) != 3 {
			continue
		}
		d, err := time.Parse("2006-01-02", parts[1])
		if err != nil {
			d = time.Now().UTC()
		}
		records = append(records, store.APIKeyUsageRecord{
			KeyID:         parts[0],
			Date:          d,
			EndpointGroup: parts[2],
			RequestCount:  count,
		})
	}

	return f.store.RecordBatchUsage(ctx, records)
}

// Start begins the periodic flush loop.
func (f *UsageFlusher) Start(ctx context.Context) {
	if f == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(f.interval)
		defer ticker.Stop()
		for {
			select {
			case <-f.stopCh:
				_ = f.Flush(context.Background())
				return
			case <-ctx.Done():
				_ = f.Flush(context.Background())
				return
			case <-ticker.C:
				_ = f.Flush(ctx)
			}
		}
	}()
}

// Stop shuts down the background flush loop and performs a final flush.
func (f *UsageFlusher) Stop() {
	if f == nil {
		return
	}
	select {
	case <-f.stopCh:
	default:
		close(f.stopCh)
	}
}

// UsageMeter returns middleware that meters API requests by API key and enforces monthly quotas.
func UsageMeter(rc RedisClient, flusher *UsageFlusher) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip exempt paths
			if r.URL.Path == "/health" || r.URL.Path == "/readyz" ||
				r.URL.Path == "/api/version" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			// Check authenticated identity
			id, ok := IdentityFromRequest(r)
			if !ok || id.APIKey == nil {
				// Anonymous or session-only caller; skip per-key quota
				next.ServeHTTP(w, r)
				return
			}

			keyID := id.APIKey.ID
			group := ClassifyEndpointGroup(r.URL.Path)
			now := time.Now().UTC()
			today := now.Format("2006-01-02")
			thisMonth := now.Format("2006-01")

			// Check monthly quota if set on key
			if id.APIKey.MonthlyQuota != nil && *id.APIKey.MonthlyQuota > 0 {
				quota := int64(*id.APIKey.MonthlyQuota)
				monthKey := "usage_month:" + keyID + ":" + thisMonth

				if rc != nil {
					count, err := rc.Incr(r.Context(), monthKey)
					if err != nil {
						// Fail open on Redis error
						usageMeterErrors.Inc()
					} else {
						if count == 1 {
							_, _ = rc.Expire(r.Context(), monthKey, 60*24*time.Hour)
						}

						if count > quota {
							usageMeterRequests.WithLabelValues(group, "quota_exceeded").Inc()

							// Calculate seconds until 1st of next month for Retry-After
							firstOfNextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
							retryAfter := int(firstOfNextMonth.Sub(now).Seconds())
							if retryAfter < 1 {
								retryAfter = 1
							}

							w.Header().Set("Content-Type", "application/json")
							w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
							w.Header().Set("X-RateLimit-Monthly-Quota", strconv.FormatInt(quota, 10))
							w.Header().Set("X-RateLimit-Monthly-Remaining", "0")
							w.WriteHeader(http.StatusTooManyRequests)

							reqID := GetRequestID(r.Context())
							_ = json.NewEncoder(w).Encode(map[string]any{
								"error": map[string]string{
									"code":       "QUOTA_EXCEEDED",
									"message":    "monthly request quota exceeded for this API key",
									"request_id": reqID,
								},
							})
							return
						}

						remaining := quota - count
						if remaining < 0 {
							remaining = 0
						}
						w.Header().Set("X-RateLimit-Monthly-Quota", strconv.FormatInt(quota, 10))
						w.Header().Set("X-RateLimit-Monthly-Remaining", strconv.FormatInt(remaining, 10))
					}
				}
			}

			// Record daily endpoint group increment in Redis
			if rc != nil {
				dayGroupKey := "usage:" + keyID + ":" + today + ":" + group
				cnt, err := rc.Incr(r.Context(), dayGroupKey)
				if err != nil {
					usageMeterErrors.Inc()
				} else if cnt == 1 {
					_, _ = rc.Expire(r.Context(), dayGroupKey, 60*24*time.Hour)
				}
			}

			// Queue count for batch flushing to PostgreSQL
			if flusher != nil {
				flusher.Record(keyID, today, group)
			}

			usageMeterRequests.WithLabelValues(group, "ok").Inc()
			next.ServeHTTP(w, r)
		})
	}
}
