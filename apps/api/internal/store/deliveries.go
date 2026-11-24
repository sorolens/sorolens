package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---- DeliveryStore ----------------------------------------------------------

const deliveryColumns = `id, subscription_id, attempt_number, status,
	signed_payload, response_status, response_body, last_error,
	next_retry_at, created_at, updated_at`

func scanDelivery(row pgx.Row) (WebhookDelivery, error) {
	var d WebhookDelivery
	var payloadRaw []byte
	err := row.Scan(
		&d.ID, &d.SubscriptionID, &d.AttemptNumber, &d.Status,
		&payloadRaw, &d.ResponseStatus, &d.ResponseBody, &d.LastError,
		&d.NextRetryAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return WebhookDelivery{}, err
	}
	d.SignedPayload = payloadRaw
	return d, nil
}

func (s *postgresStore) ListDeliveries(ctx context.Context, subscriptionID, cursor string, limit int) ([]WebhookDelivery, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.readDB(ctx).Query(ctx, `
		SELECT `+deliveryColumns+`
		FROM webhook_deliveries
		WHERE subscription_id = $1
		  AND ($2 = '' OR created_at < (SELECT created_at FROM webhook_deliveries WHERE id = $2))
		ORDER BY created_at DESC, id DESC
		LIMIT $3`,
		subscriptionID, cursor, limit+1,
	)
	if err != nil {
		return nil, "", fmt.Errorf("list deliveries: %w", err)
	}
	defer rows.Close()

	var out []WebhookDelivery
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, "", fmt.Errorf("scan delivery: %w", err)
		}
		out = append(out, d)
	}
	if rows.Err() != nil {
		return nil, "", rows.Err()
	}

	var nextCursor string
	if len(out) > limit {
		nextCursor = out[limit-1].ID
		out = out[:limit]
	}
	return out, nextCursor, nil
}

func (s *postgresStore) GetDelivery(ctx context.Context, id string) (WebhookDelivery, error) {
	row := s.readDB(ctx).QueryRow(ctx, `
		SELECT `+deliveryColumns+`
		FROM webhook_deliveries
		WHERE id = $1`, id)
	d, err := scanDelivery(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return WebhookDelivery{}, ErrNotFound
	}
	if err != nil {
		return WebhookDelivery{}, fmt.Errorf("get delivery: %w", err)
	}
	return d, nil
}

func (s *postgresStore) UpsertDelivery(ctx context.Context, d WebhookDelivery) error {
	payload := d.SignedPayload
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO webhook_deliveries
			(id, subscription_id, attempt_number, status, signed_payload,
			 response_status, response_body, last_error, next_retry_at,
			 created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW())
		ON CONFLICT (id) DO UPDATE SET
			attempt_number = EXCLUDED.attempt_number,
			status         = EXCLUDED.status,
			response_status = EXCLUDED.response_status,
			response_body   = EXCLUDED.response_body,
			last_error      = EXCLUDED.last_error,
			next_retry_at   = EXCLUDED.next_retry_at,
			updated_at      = NOW()`,
		d.ID, d.SubscriptionID, d.AttemptNumber, d.Status, payload,
		d.ResponseStatus, d.ResponseBody, d.LastError, d.NextRetryAt,
		d.CreatedAt,
	)
	return err
}

func (s *postgresStore) RetryDelivery(ctx context.Context, subscriptionID, id string) (WebhookDelivery, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE webhook_deliveries
		SET status         = 'pending',
		    attempt_number = attempt_number + 1,
		    next_retry_at  = NULL,
		    last_error     = NULL,
		    updated_at     = NOW()
		WHERE id = $1 AND subscription_id = $2
		RETURNING `+deliveryColumns, id, subscriptionID)
	d, err := scanDelivery(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return WebhookDelivery{}, ErrNotFound
	}
	if err != nil {
		return WebhookDelivery{}, fmt.Errorf("retry delivery: %w", err)
	}
	return d, nil
}

// ---- IdempotencyStore -------------------------------------------------------

func (s *postgresStore) ClaimIdempotencyKey(ctx context.Context, apiKeyID, key, bodyFingerprint string) (IdempotencyRecord, bool, error) {
	// Try to insert; if the row already exists, return the existing record.
	row := s.pool.QueryRow(ctx, `
		INSERT INTO idempotency_keys
			(api_key_id, idempotency_key, body_fingerprint,
			 response_status, response_headers, response_body,
			 in_flight, created_at, expires_at)
		VALUES ($1, $2, $3, 0, '{}', '', TRUE, NOW(), NOW() + INTERVAL '24 hours')
		ON CONFLICT (api_key_id, idempotency_key) DO NOTHING
		RETURNING api_key_id, idempotency_key, body_fingerprint,
		          response_status, response_headers, response_body,
		          in_flight, created_at, expires_at`,
		apiKeyID, key, bodyFingerprint,
	)
	rec, err := scanIdempotencyRecord(row)
	if err == nil {
		// Fresh claim.
		return rec, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return IdempotencyRecord{}, false, fmt.Errorf("claim idempotency key: %w", err)
	}

	// Row already exists: fetch it.
	existing, err := s.GetIdempotencyRecord(ctx, apiKeyID, key)
	if err != nil {
		return IdempotencyRecord{}, false, err
	}
	return existing, false, nil
}

func (s *postgresStore) GetIdempotencyRecord(ctx context.Context, apiKeyID, key string) (IdempotencyRecord, error) {
	row := s.readDB(ctx).QueryRow(ctx, `
		SELECT api_key_id, idempotency_key, body_fingerprint,
		       response_status, response_headers, response_body,
		       in_flight, created_at, expires_at
		FROM idempotency_keys
		WHERE api_key_id = $1 AND idempotency_key = $2`,
		apiKeyID, key,
	)
	rec, err := scanIdempotencyRecord(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return IdempotencyRecord{}, ErrNotFound
	}
	return rec, err
}

func (s *postgresStore) CompleteIdempotencyKey(ctx context.Context, apiKeyID, key string, status int, headers map[string]string, body string) error {
	headersJSON, _ := json.Marshal(headers)
	_, err := s.pool.Exec(ctx, `
		UPDATE idempotency_keys
		SET response_status  = $3,
		    response_headers = $4,
		    response_body    = $5,
		    in_flight        = FALSE
		WHERE api_key_id = $1 AND idempotency_key = $2`,
		apiKeyID, key, status, headersJSON, body,
	)
	return err
}

func (s *postgresStore) ReleaseIdempotencyKey(ctx context.Context, apiKeyID, key string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM idempotency_keys
		WHERE api_key_id = $1 AND idempotency_key = $2 AND in_flight`,
		apiKeyID, key)
	return err
}

func (s *postgresStore) DeleteExpiredIdempotencyKeys(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func scanIdempotencyRecord(row pgx.Row) (IdempotencyRecord, error) {
	var rec IdempotencyRecord
	var headersJSON []byte
	err := row.Scan(
		&rec.APIKeyID, &rec.IdempotencyKey, &rec.BodyFingerprint,
		&rec.ResponseStatus, &headersJSON, &rec.ResponseBody,
		&rec.InFlight, &rec.CreatedAt, &rec.ExpiresAt,
	)
	if err != nil {
		return IdempotencyRecord{}, err
	}
	_ = json.Unmarshal(headersJSON, &rec.ResponseHeaders)
	if rec.ResponseHeaders == nil {
		rec.ResponseHeaders = map[string]string{}
	}
	return rec, nil
}

// unused import guard.
var _ = time.Second
