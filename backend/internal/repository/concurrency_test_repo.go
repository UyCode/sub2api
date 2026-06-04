package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type concurrencyTestRepository struct {
	db *sql.DB
}

func NewConcurrencyTestRepository(db *sql.DB) service.ConcurrencyTestRepository {
	return &concurrencyTestRepository{db: db}
}

func (r *concurrencyTestRepository) CreateConfig(ctx context.Context, cfg *service.ConcurrencyTestConfig) error {
	headers, err := json.Marshal(cfg.Headers)
	if err != nil {
		return err
	}
	body, err := json.Marshal(cfg.BodyTemplate)
	if err != nil {
		return err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO concurrency_test_configs (
			name, description, mode, concurrency, account_ids, endpoint, api_key_encrypted,
			method, headers, body_template, timeout_seconds, created_by, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11, $12, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`, cfg.Name, cfg.Description, cfg.Mode, cfg.Concurrency, pq.Array(cfg.AccountIDs), cfg.Endpoint,
		cfg.APIKey, cfg.Method, headers, body, cfg.TimeoutSeconds, cfg.CreatedBy)
	if err := row.Scan(&cfg.ID, &cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
		return err
	}
	return nil
}

func (r *concurrencyTestRepository) UpdateConfig(ctx context.Context, cfg *service.ConcurrencyTestConfig) error {
	headers, err := json.Marshal(cfg.Headers)
	if err != nil {
		return err
	}
	body, err := json.Marshal(cfg.BodyTemplate)
	if err != nil {
		return err
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE concurrency_test_configs
		SET name = $2,
		    description = $3,
		    mode = $4,
		    concurrency = $5,
		    account_ids = $6,
		    endpoint = $7,
		    api_key_encrypted = $8,
		    method = $9,
		    headers = $10::jsonb,
		    body_template = $11::jsonb,
		    timeout_seconds = $12,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`, cfg.ID, cfg.Name, cfg.Description, cfg.Mode, cfg.Concurrency, pq.Array(cfg.AccountIDs),
		cfg.Endpoint, cfg.APIKey, cfg.Method, headers, body, cfg.TimeoutSeconds)
	if err := row.Scan(&cfg.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrConcurrencyTestNotFound
		}
		return err
	}
	return nil
}

func (r *concurrencyTestRepository) DeleteConfig(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM concurrency_test_configs WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrConcurrencyTestNotFound
	}
	return nil
}

func (r *concurrencyTestRepository) GetConfig(ctx context.Context, id int64) (*service.ConcurrencyTestConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.name, c.description, c.mode, c.concurrency, c.account_ids,
		       c.endpoint, c.api_key_encrypted, c.method, c.headers, c.body_template,
		       c.timeout_seconds, c.created_by, c.created_at, c.updated_at,
		       r.id, r.config_id, r.name, r.mode, r.concurrency, r.account_ids,
		       r.request_source, r.endpoint, r.method, r.started_at, r.finished_at,
		       r.status, r.total_requests, r.success_count, r.failure_count,
		       r.timeout_count, r.gateway_timeouts, r.success_rate,
		       r.avg_latency_ms, r.min_latency_ms, r.max_latency_ms, r.p50_latency_ms,
		       r.p90_latency_ms, r.p95_latency_ms, r.p99_latency_ms, r.summary,
		       r.error_message, r.created_at
		FROM concurrency_test_configs c
		LEFT JOIN LATERAL (
			SELECT *
			FROM concurrency_test_runs
			WHERE config_id = c.id
			ORDER BY created_at DESC
			LIMIT 1
		) r ON TRUE
		WHERE c.id = $1
	`, id)
	return scanConcurrencyTestConfigWithRun(row)
}

func (r *concurrencyTestRepository) ListConfigs(ctx context.Context, params service.ConcurrencyTestListParams) ([]*service.ConcurrencyTestConfig, int64, error) {
	args := []any{}
	where := ""
	search := strings.TrimSpace(params.Search)
	if search != "" {
		args = append(args, "%"+search+"%")
		where = fmt.Sprintf("WHERE c.name ILIKE $%d OR c.description ILIKE $%d", len(args), len(args))
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM concurrency_test_configs c "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, params.PageSize, (params.Page-1)*params.PageSize)
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.description, c.mode, c.concurrency, c.account_ids,
		       c.endpoint, c.api_key_encrypted, c.method, c.headers, c.body_template,
		       c.timeout_seconds, c.created_by, c.created_at, c.updated_at,
		       r.id, r.config_id, r.name, r.mode, r.concurrency, r.account_ids,
		       r.request_source, r.endpoint, r.method, r.started_at, r.finished_at,
		       r.status, r.total_requests, r.success_count, r.failure_count,
		       r.timeout_count, r.gateway_timeouts, r.success_rate,
		       r.avg_latency_ms, r.min_latency_ms, r.max_latency_ms, r.p50_latency_ms,
		       r.p90_latency_ms, r.p95_latency_ms, r.p99_latency_ms, r.summary,
		       r.error_message, r.created_at
		FROM concurrency_test_configs c
		LEFT JOIN LATERAL (
			SELECT *
			FROM concurrency_test_runs
			WHERE config_id = c.id
			ORDER BY created_at DESC
			LIMIT 1
		) r ON TRUE
		`+where+`
		ORDER BY c.id DESC
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args))+`
	`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]*service.ConcurrencyTestConfig, 0)
	for rows.Next() {
		cfg, err := scanConcurrencyTestConfigWithRun(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *concurrencyTestRepository) CreateRun(ctx context.Context, run *service.ConcurrencyTestRun) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	summary, err := json.Marshal(run.Summary)
	if err != nil {
		return err
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO concurrency_test_runs (
			config_id, name, mode, concurrency, account_ids, request_source, endpoint, method,
			started_at, finished_at, status, total_requests, success_count, failure_count,
			timeout_count, gateway_timeouts, success_rate, avg_latency_ms, min_latency_ms,
			max_latency_ms, p50_latency_ms, p90_latency_ms, p95_latency_ms, p99_latency_ms,
			summary, error_message, created_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19,
			$20, $21, $22, $23, $24,
			$25::jsonb, $26, NOW()
		)
		RETURNING id, created_at
	`, run.ConfigID, run.Name, run.Mode, run.Concurrency, pq.Array(run.AccountIDs),
		run.RequestSource, run.Endpoint, run.Method, run.StartedAt, run.FinishedAt,
		run.Status, run.TotalRequests, run.SuccessCount, run.FailureCount,
		run.TimeoutCount, run.GatewayTimeouts, run.SuccessRate, run.AvgLatencyMs,
		run.MinLatencyMs, run.MaxLatencyMs, run.P50LatencyMs, run.P90LatencyMs,
		run.P95LatencyMs, run.P99LatencyMs, summary, run.ErrorMessage)
	if err = row.Scan(&run.ID, &run.CreatedAt); err != nil {
		return err
	}

	for _, logRow := range run.Logs {
		if logRow == nil {
			continue
		}
		logRow.RunID = run.ID
		if err = insertConcurrencyTestLog(ctx, tx, logRow); err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}

func (r *concurrencyTestRepository) ListRuns(ctx context.Context, configID int64, limit int) ([]*service.ConcurrencyTestRun, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, config_id, name, mode, concurrency, account_ids, request_source, endpoint, method,
		       started_at, finished_at, status, total_requests, success_count, failure_count,
		       timeout_count, gateway_timeouts, success_rate, avg_latency_ms, min_latency_ms,
		       max_latency_ms, p50_latency_ms, p90_latency_ms, p95_latency_ms, p99_latency_ms,
		       summary, error_message, created_at
		FROM concurrency_test_runs
		WHERE config_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, configID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]*service.ConcurrencyTestRun, 0)
	for rows.Next() {
		run, err := scanConcurrencyTestRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func (r *concurrencyTestRepository) ListLogs(ctx context.Context, runID int64, limit int) ([]*service.ConcurrencyTestLog, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, run_id, request_index, account_id, endpoint, method, status_code, success,
		       timeout, latency_ms, error_message, response_body, started_at, finished_at, created_at
		FROM concurrency_test_logs
		WHERE run_id = $1
		ORDER BY request_index ASC
		LIMIT $2
	`, runID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]*service.ConcurrencyTestLog, 0)
	for rows.Next() {
		row, err := scanConcurrencyTestLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func insertConcurrencyTestLog(ctx context.Context, tx *sql.Tx, row *service.ConcurrencyTestLog) error {
	return tx.QueryRowContext(ctx, `
		INSERT INTO concurrency_test_logs (
			run_id, request_index, account_id, endpoint, method, status_code, success,
			timeout, latency_ms, error_message, response_body, started_at, finished_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW())
		RETURNING id, created_at
	`, row.RunID, row.RequestIndex, nullableInt64(row.AccountID), row.Endpoint, row.Method,
		nullableInt(row.StatusCode), row.Success, row.Timeout, row.LatencyMs,
		row.ErrorMessage, row.ResponseBody, row.StartedAt, row.FinishedAt).Scan(&row.ID, &row.CreatedAt)
}

type concurrencyScannable interface {
	Scan(dest ...any) error
}

func scanConcurrencyTestConfigWithRun(row concurrencyScannable) (*service.ConcurrencyTestConfig, error) {
	cfg := &service.ConcurrencyTestConfig{}
	var accountIDs []int64
	var headersRaw, bodyRaw []byte
	run := nullableConcurrencyTestRunScan{}
	if err := row.Scan(
		&cfg.ID, &cfg.Name, &cfg.Description, &cfg.Mode, &cfg.Concurrency, pq.Array(&accountIDs),
		&cfg.Endpoint, &cfg.APIKey, &cfg.Method, &headersRaw, &bodyRaw,
		&cfg.TimeoutSeconds, &cfg.CreatedBy, &cfg.CreatedAt, &cfg.UpdatedAt,
		&run.ID, &run.ConfigID, &run.Name, &run.Mode, &run.Concurrency, pq.Array(&run.AccountIDs),
		&run.RequestSource, &run.Endpoint, &run.Method, &run.StartedAt, &run.FinishedAt,
		&run.Status, &run.TotalRequests, &run.SuccessCount, &run.FailureCount,
		&run.TimeoutCount, &run.GatewayTimeouts, &run.SuccessRate,
		&run.AvgLatencyMs, &run.MinLatencyMs, &run.MaxLatencyMs, &run.P50LatencyMs,
		&run.P90LatencyMs, &run.P95LatencyMs, &run.P99LatencyMs, &run.Summary,
		&run.ErrorMessage, &run.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrConcurrencyTestNotFound
		}
		return nil, err
	}
	cfg.AccountIDs = accountIDs
	cfg.Headers = map[string]string{}
	_ = json.Unmarshal(headersRaw, &cfg.Headers)
	cfg.BodyTemplate = map[string]any{}
	_ = json.Unmarshal(bodyRaw, &cfg.BodyTemplate)
	cfg.LatestRun = run.toService()
	return cfg, nil
}

func scanConcurrencyTestRun(row concurrencyScannable) (*service.ConcurrencyTestRun, error) {
	run := &service.ConcurrencyTestRun{}
	var accountIDs []int64
	var summaryRaw []byte
	if err := row.Scan(
		&run.ID, &run.ConfigID, &run.Name, &run.Mode, &run.Concurrency, pq.Array(&accountIDs),
		&run.RequestSource, &run.Endpoint, &run.Method, &run.StartedAt, &run.FinishedAt,
		&run.Status, &run.TotalRequests, &run.SuccessCount, &run.FailureCount,
		&run.TimeoutCount, &run.GatewayTimeouts, &run.SuccessRate,
		&run.AvgLatencyMs, &run.MinLatencyMs, &run.MaxLatencyMs, &run.P50LatencyMs,
		&run.P90LatencyMs, &run.P95LatencyMs, &run.P99LatencyMs,
		&summaryRaw, &run.ErrorMessage, &run.CreatedAt,
	); err != nil {
		return nil, err
	}
	run.AccountIDs = accountIDs
	run.Summary = map[string]any{}
	_ = json.Unmarshal(summaryRaw, &run.Summary)
	return run, nil
}

func scanConcurrencyTestLog(row concurrencyScannable) (*service.ConcurrencyTestLog, error) {
	out := &service.ConcurrencyTestLog{}
	var accountID, statusCode sql.NullInt64
	if err := row.Scan(
		&out.ID, &out.RunID, &out.RequestIndex, &accountID, &out.Endpoint, &out.Method,
		&statusCode, &out.Success, &out.Timeout, &out.LatencyMs, &out.ErrorMessage,
		&out.ResponseBody, &out.StartedAt, &out.FinishedAt, &out.CreatedAt,
	); err != nil {
		return nil, err
	}
	if accountID.Valid {
		v := accountID.Int64
		out.AccountID = &v
	}
	if statusCode.Valid {
		v := int(statusCode.Int64)
		out.StatusCode = &v
	}
	return out, nil
}

type nullableConcurrencyTestRunScan struct {
	ID              sql.NullInt64
	ConfigID        sql.NullInt64
	Name            sql.NullString
	Mode            sql.NullString
	Concurrency     sql.NullInt64
	AccountIDs      []int64
	RequestSource   sql.NullString
	Endpoint        sql.NullString
	Method          sql.NullString
	StartedAt       sql.NullTime
	FinishedAt      sql.NullTime
	Status          sql.NullString
	TotalRequests   sql.NullInt64
	SuccessCount    sql.NullInt64
	FailureCount    sql.NullInt64
	TimeoutCount    sql.NullInt64
	GatewayTimeouts sql.NullInt64
	SuccessRate     sql.NullFloat64
	AvgLatencyMs    sql.NullInt64
	MinLatencyMs    sql.NullInt64
	MaxLatencyMs    sql.NullInt64
	P50LatencyMs    sql.NullInt64
	P90LatencyMs    sql.NullInt64
	P95LatencyMs    sql.NullInt64
	P99LatencyMs    sql.NullInt64
	Summary         []byte
	ErrorMessage    sql.NullString
	CreatedAt       sql.NullTime
}

func (r nullableConcurrencyTestRunScan) toService() *service.ConcurrencyTestRun {
	if !r.ID.Valid {
		return nil
	}
	run := &service.ConcurrencyTestRun{
		ID:              r.ID.Int64,
		ConfigID:        r.ConfigID.Int64,
		Name:            r.Name.String,
		Mode:            r.Mode.String,
		Concurrency:     int(r.Concurrency.Int64),
		AccountIDs:      r.AccountIDs,
		RequestSource:   r.RequestSource.String,
		Endpoint:        r.Endpoint.String,
		Method:          r.Method.String,
		StartedAt:       r.StartedAt.Time,
		FinishedAt:      r.FinishedAt.Time,
		Status:          r.Status.String,
		TotalRequests:   int(r.TotalRequests.Int64),
		SuccessCount:    int(r.SuccessCount.Int64),
		FailureCount:    int(r.FailureCount.Int64),
		TimeoutCount:    int(r.TimeoutCount.Int64),
		GatewayTimeouts: int(r.GatewayTimeouts.Int64),
		SuccessRate:     r.SuccessRate.Float64,
		AvgLatencyMs:    int(r.AvgLatencyMs.Int64),
		MinLatencyMs:    int(r.MinLatencyMs.Int64),
		MaxLatencyMs:    int(r.MaxLatencyMs.Int64),
		P50LatencyMs:    int(r.P50LatencyMs.Int64),
		P90LatencyMs:    int(r.P90LatencyMs.Int64),
		P95LatencyMs:    int(r.P95LatencyMs.Int64),
		P99LatencyMs:    int(r.P99LatencyMs.Int64),
		ErrorMessage:    r.ErrorMessage.String,
		CreatedAt:       r.CreatedAt.Time,
		Summary:         map[string]any{},
	}
	_ = json.Unmarshal(r.Summary, &run.Summary)
	return run
}

func nullableInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

var _ = time.Time{}
