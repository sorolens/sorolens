package history

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// BigQueryEndpoint is the BigQuery REST API root.
const BigQueryEndpoint = "https://bigquery.googleapis.com/bigquery/v2"

// TokenSource supplies OAuth2 access tokens for BigQuery. The indexer's
// import mode builds one from Application Default Credentials.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// BigQueryReader runs Hubble queries through BigQuery's REST API
// (jobs.query and jobs.getQueryResults).
//
// It speaks the REST API directly rather than through the Google Cloud Go
// SDK, which would add dozens of modules to the indexer for one query shape.
type BigQueryReader struct {
	// Endpoint defaults to BigQueryEndpoint; tests point it at a fake.
	Endpoint       string
	BillingProject string
	MaxBytesBilled int64
	Tokens         TokenSource
	HTTP           *http.Client
	// PollInterval is how long to wait before asking again for a query
	// that has not finished. Defaults to 2s.
	PollInterval time.Duration
}

type bqParam struct {
	Name string `json:"name"`
	Type struct {
		Type string `json:"type"`
	} `json:"parameterType"`
	Value struct {
		Value string `json:"value"`
	} `json:"parameterValue"`
}

type bqQueryRequest struct {
	Query           string    `json:"query"`
	UseLegacySQL    bool      `json:"useLegacySql"`
	ParameterMode   string    `json:"parameterMode"`
	QueryParameters []bqParam `json:"queryParameters"`
	MaximumBytes    string    `json:"maximumBytesBilled,omitempty"`
	TimeoutMs       int       `json:"timeoutMs"`
	MaxResults      int       `json:"maxResults"`
}

type bqField struct {
	Name string `json:"name"`
}

type bqResponse struct {
	JobComplete  bool `json:"jobComplete"`
	JobReference struct {
		ProjectID string `json:"projectId"`
		JobID     string `json:"jobId"`
		Location  string `json:"location"`
	} `json:"jobReference"`
	Schema struct {
		Fields []bqField `json:"fields"`
	} `json:"schema"`
	Rows []struct {
		F []struct {
			V any `json:"v"`
		} `json:"f"`
	} `json:"rows"`
	PageToken string `json:"pageToken"`
	Error     *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// toParam encodes a query parameter in BigQuery's REST form.
func toParam(name string, value any) (bqParam, error) {
	var p bqParam
	p.Name = name
	switch v := value.(type) {
	case string:
		p.Type.Type, p.Value.Value = "STRING", v
	case int64:
		p.Type.Type, p.Value.Value = "INT64", strconv.FormatInt(v, 10)
	case time.Time:
		p.Type.Type, p.Value.Value = "TIMESTAMP", v.UTC().Format("2006-01-02 15:04:05.999999 UTC")
	default:
		return p, fmt.Errorf("hubble: unsupported parameter %s of type %T", name, value)
	}
	return p, nil
}

// Rows implements RowReader.
func (r *BigQueryReader) Rows(ctx context.Context, sql string, params map[string]any) ([]HubbleRow, error) {
	if r.BillingProject == "" || r.Tokens == nil {
		return nil, errors.New("hubble: the BigQuery reader needs a billing project and credentials")
	}
	req := bqQueryRequest{Query: sql, ParameterMode: "NAMED", TimeoutMs: 60_000, MaxResults: 10_000}
	for name, value := range params {
		p, err := toParam(name, value)
		if err != nil {
			return nil, err
		}
		req.QueryParameters = append(req.QueryParameters, p)
	}
	limit := r.MaxBytesBilled
	if limit <= 0 {
		limit = DefaultMaxBytesBilled
	}
	req.MaximumBytes = strconv.FormatInt(limit, 10)

	var resp bqResponse
	if err := r.call(ctx, http.MethodPost, "/projects/"+url.PathEscape(r.BillingProject)+"/queries", req, &resp); err != nil {
		return nil, err
	}
	var rows []HubbleRow
	for {
		if resp.Error != nil {
			return nil, fmt.Errorf("hubble: query failed: %s", resp.Error.Message)
		}
		if resp.JobComplete {
			page, err := decodeRows(resp)
			if err != nil {
				return nil, err
			}
			rows = append(rows, page...)
			if resp.PageToken == "" {
				return rows, nil
			}
		} else if err := r.wait(ctx); err != nil {
			return nil, err
		}
		next, err := r.results(ctx, resp)
		if err != nil {
			return nil, err
		}
		resp = next
	}
}

// results fetches the next page of a job's results, or polls a job that has
// not finished.
func (r *BigQueryReader) results(ctx context.Context, prev bqResponse) (bqResponse, error) {
	ref := prev.JobReference
	if ref.JobID == "" {
		return bqResponse{}, errors.New("hubble: query response has no job reference")
	}
	q := url.Values{"maxResults": {"10000"}, "timeoutMs": {"60000"}}
	if ref.Location != "" {
		q.Set("location", ref.Location)
	}
	if prev.PageToken != "" {
		q.Set("pageToken", prev.PageToken)
	}
	path := fmt.Sprintf("/projects/%s/queries/%s?%s", url.PathEscape(ref.ProjectID), url.PathEscape(ref.JobID), q.Encode())
	var next bqResponse
	if err := r.call(ctx, http.MethodGet, path, nil, &next); err != nil {
		return bqResponse{}, err
	}
	if next.JobReference.JobID == "" {
		next.JobReference = ref
	}
	return next, nil
}

func (r *BigQueryReader) wait(ctx context.Context) error {
	d := r.PollInterval
	if d <= 0 {
		d = 2 * time.Second
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (r *BigQueryReader) call(ctx context.Context, method, path string, body any, out any) error {
	token, err := r.Tokens.Token(ctx)
	if err != nil {
		return fmt.Errorf("hubble: credentials: %w", err)
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	endpoint := r.Endpoint
	if endpoint == "" {
		endpoint = BigQueryEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(endpoint, "/")+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := r.HTTP
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("hubble: bigquery request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response fully read below
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	if err != nil {
		return fmt.Errorf("hubble: read bigquery response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		msg := strings.TrimSpace(string(raw))
		if json.Unmarshal(raw, &apiErr) == nil && apiErr.Error.Message != "" {
			msg = apiErr.Error.Message
		}
		return fmt.Errorf("hubble: bigquery returned %s: %s", resp.Status, msg)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("hubble: decode bigquery response: %w", err)
	}
	return nil
}

// decodeRows maps BigQuery's row format (every cell a string, keyed by the
// schema's field order) onto HubbleRow.
func decodeRows(resp bqResponse) ([]HubbleRow, error) {
	index := map[string]int{}
	for i, f := range resp.Schema.Fields {
		index[f.Name] = i
	}
	for _, want := range []string{"ledger_sequence", "closed_at", "transaction_hash", "transaction_id",
		"event_index", "contract_id", "contract_event_xdr", "in_successful_contract_call"} {
		if _, ok := index[want]; !ok && len(resp.Rows) > 0 {
			return nil, fmt.Errorf("hubble: query result has no %s column", want)
		}
	}
	rows := make([]HubbleRow, 0, len(resp.Rows))
	for n, row := range resp.Rows {
		cell := func(name string) string {
			i := index[name]
			if i >= len(row.F) {
				return ""
			}
			s, _ := row.F[i].V.(string)
			return s
		}
		var out HubbleRow
		var err error
		if out.LedgerSequence, err = strconv.ParseInt(cell("ledger_sequence"), 10, 64); err != nil {
			return nil, fmt.Errorf("hubble: row %d ledger_sequence: %w", n, err)
		}
		if out.TransactionID, err = strconv.ParseInt(cell("transaction_id"), 10, 64); err != nil {
			return nil, fmt.Errorf("hubble: row %d transaction_id: %w", n, err)
		}
		if out.EventIndex, err = strconv.ParseInt(cell("event_index"), 10, 64); err != nil {
			return nil, fmt.Errorf("hubble: row %d event_index: %w", n, err)
		}
		// TIMESTAMP cells are seconds since the epoch, with a fraction.
		secs, err := strconv.ParseFloat(cell("closed_at"), 64)
		if err != nil {
			return nil, fmt.Errorf("hubble: row %d closed_at: %w", n, err)
		}
		out.ClosedAt = time.Unix(0, int64(secs*1e6)*int64(time.Microsecond)).UTC()
		out.TransactionHash = cell("transaction_hash")
		out.ContractID = cell("contract_id")
		out.ContractEventXDR = cell("contract_event_xdr")
		out.InSuccessfulContractCall = cell("in_successful_contract_call") == "true"
		rows = append(rows, out)
	}
	return rows, nil
}
