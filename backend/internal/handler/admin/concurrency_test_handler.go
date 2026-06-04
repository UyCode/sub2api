package admin

import (
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type ConcurrencyTestHandler struct {
	svc *service.ConcurrencyTestService
}

func NewConcurrencyTestHandler(svc *service.ConcurrencyTestService) *ConcurrencyTestHandler {
	return &ConcurrencyTestHandler{svc: svc}
}

type concurrencyTestUpsertRequest struct {
	Name           string            `json:"name" binding:"required,max=100"`
	Description    string            `json:"description" binding:"max=500"`
	Mode           string            `json:"mode" binding:"required"`
	Concurrency    int               `json:"concurrency" binding:"required,min=1,max=500"`
	AccountIDs     []int64           `json:"account_ids"`
	Endpoint       string            `json:"endpoint" binding:"max=500"`
	APIKey         string            `json:"api_key" binding:"max=4000"`
	Method         string            `json:"method" binding:"max=12"`
	Headers        map[string]string `json:"headers"`
	BodyTemplate   map[string]any    `json:"body_template"`
	TimeoutSeconds int               `json:"timeout_seconds" binding:"omitempty,min=1,max=600"`
}

type concurrencyTestPatchRequest struct {
	Name           *string            `json:"name" binding:"omitempty,max=100"`
	Description    *string            `json:"description" binding:"omitempty,max=500"`
	Mode           *string            `json:"mode"`
	Concurrency    *int               `json:"concurrency" binding:"omitempty,min=1,max=500"`
	AccountIDs     *[]int64           `json:"account_ids"`
	Endpoint       *string            `json:"endpoint" binding:"omitempty,max=500"`
	APIKey         *string            `json:"api_key" binding:"omitempty,max=4000"`
	Method         *string            `json:"method" binding:"omitempty,max=12"`
	Headers        *map[string]string `json:"headers"`
	BodyTemplate   *map[string]any    `json:"body_template"`
	TimeoutSeconds *int               `json:"timeout_seconds" binding:"omitempty,min=1,max=600"`
}

type concurrencyTestConfigResponse struct {
	ID             int64                       `json:"id"`
	Name           string                      `json:"name"`
	Description    string                      `json:"description"`
	Mode           string                      `json:"mode"`
	Concurrency    int                         `json:"concurrency"`
	AccountIDs     []int64                     `json:"account_ids"`
	Endpoint       string                      `json:"endpoint"`
	APIKeySet      bool                        `json:"api_key_set"`
	APIKeyMasked   string                      `json:"api_key_masked"`
	Method         string                      `json:"method"`
	Headers        map[string]string           `json:"headers"`
	BodyTemplate   map[string]any              `json:"body_template"`
	TimeoutSeconds int                         `json:"timeout_seconds"`
	CreatedBy      int64                       `json:"created_by"`
	CreatedAt      string                      `json:"created_at"`
	UpdatedAt      string                      `json:"updated_at"`
	LatestRun      *concurrencyTestRunResponse `json:"latest_run,omitempty"`
}

type concurrencyTestRunResponse struct {
	ID              int64                    `json:"id"`
	ConfigID        int64                    `json:"config_id"`
	Name            string                   `json:"name"`
	Mode            string                   `json:"mode"`
	Concurrency     int                      `json:"concurrency"`
	AccountIDs      []int64                  `json:"account_ids"`
	RequestSource   string                   `json:"request_source"`
	Endpoint        string                   `json:"endpoint"`
	Method          string                   `json:"method"`
	StartedAt       string                   `json:"started_at"`
	FinishedAt      string                   `json:"finished_at"`
	Status          string                   `json:"status"`
	TotalRequests   int                      `json:"total_requests"`
	SuccessCount    int                      `json:"success_count"`
	FailureCount    int                      `json:"failure_count"`
	TimeoutCount    int                      `json:"timeout_count"`
	GatewayTimeouts int                      `json:"gateway_timeouts"`
	SuccessRate     float64                  `json:"success_rate"`
	AvgLatencyMs    int                      `json:"avg_latency_ms"`
	MinLatencyMs    int                      `json:"min_latency_ms"`
	MaxLatencyMs    int                      `json:"max_latency_ms"`
	P50LatencyMs    int                      `json:"p50_latency_ms"`
	P90LatencyMs    int                      `json:"p90_latency_ms"`
	P95LatencyMs    int                      `json:"p95_latency_ms"`
	P99LatencyMs    int                      `json:"p99_latency_ms"`
	Summary         map[string]any           `json:"summary"`
	ErrorMessage    string                   `json:"error_message"`
	CreatedAt       string                   `json:"created_at"`
	Logs            []concurrencyTestLogItem `json:"logs,omitempty"`
}

type concurrencyTestLogItem struct {
	ID           int64  `json:"id"`
	RunID        int64  `json:"run_id"`
	RequestIndex int    `json:"request_index"`
	AccountID    *int64 `json:"account_id"`
	Endpoint     string `json:"endpoint"`
	Method       string `json:"method"`
	StatusCode   *int   `json:"status_code"`
	Success      bool   `json:"success"`
	Timeout      bool   `json:"timeout"`
	LatencyMs    int    `json:"latency_ms"`
	ErrorMessage string `json:"error_message"`
	ResponseBody string `json:"response_body"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at"`
	CreatedAt    string `json:"created_at"`
}

func (h *ConcurrencyTestHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	items, total, err := h.svc.List(c.Request.Context(), service.ConcurrencyTestListParams{
		Page:     page,
		PageSize: pageSize,
		Search:   strings.TrimSpace(c.Query("search")),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]*concurrencyTestConfigResponse, 0, len(items))
	for _, item := range items {
		out = append(out, concurrencyTestConfigToResponse(item))
	}
	response.Paginated(c, out, total, page, pageSize)
}

func (h *ConcurrencyTestHandler) Get(c *gin.Context) {
	id, ok := parseConcurrencyTestID(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, concurrencyTestConfigToResponse(item))
}

func (h *ConcurrencyTestHandler) Create(c *gin.Context) {
	var req concurrencyTestUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	item, err := h.svc.Create(c.Request.Context(), service.ConcurrencyTestCreateParams{
		Name:           req.Name,
		Description:    req.Description,
		Mode:           req.Mode,
		Concurrency:    req.Concurrency,
		AccountIDs:     req.AccountIDs,
		Endpoint:       req.Endpoint,
		APIKey:         req.APIKey,
		Method:         req.Method,
		Headers:        req.Headers,
		BodyTemplate:   req.BodyTemplate,
		TimeoutSeconds: req.TimeoutSeconds,
		CreatedBy:      subject.UserID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, concurrencyTestConfigToResponse(item))
}

func (h *ConcurrencyTestHandler) Update(c *gin.Context) {
	id, ok := parseConcurrencyTestID(c, "id")
	if !ok {
		return
	}
	var req concurrencyTestPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	item, err := h.svc.Update(c.Request.Context(), id, service.ConcurrencyTestUpdateParams{
		Name:           req.Name,
		Description:    req.Description,
		Mode:           req.Mode,
		Concurrency:    req.Concurrency,
		AccountIDs:     req.AccountIDs,
		Endpoint:       req.Endpoint,
		APIKey:         req.APIKey,
		Method:         req.Method,
		Headers:        req.Headers,
		BodyTemplate:   req.BodyTemplate,
		TimeoutSeconds: req.TimeoutSeconds,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, concurrencyTestConfigToResponse(item))
}

func (h *ConcurrencyTestHandler) Delete(c *gin.Context) {
	id, ok := parseConcurrencyTestID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *ConcurrencyTestHandler) Run(c *gin.Context) {
	id, ok := parseConcurrencyTestID(c, "id")
	if !ok {
		return
	}
	run, err := h.svc.Run(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, concurrencyTestRunToResponse(run, true))
}

func (h *ConcurrencyTestHandler) ListRuns(c *gin.Context) {
	id, ok := parseConcurrencyTestID(c, "id")
	if !ok {
		return
	}
	limit := parseConcurrencyLimit(c.Query("limit"), 20, 100)
	runs, err := h.svc.ListRuns(c.Request.Context(), id, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]*concurrencyTestRunResponse, 0, len(runs))
	for _, run := range runs {
		out = append(out, concurrencyTestRunToResponse(run, false))
	}
	response.Success(c, gin.H{"items": out})
}

func (h *ConcurrencyTestHandler) ListLogs(c *gin.Context) {
	runID, ok := parseConcurrencyTestID(c, "run_id")
	if !ok {
		return
	}
	limit := parseConcurrencyLimit(c.Query("limit"), 200, 1000)
	logs, err := h.svc.ListLogs(c.Request.Context(), runID, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]concurrencyTestLogItem, 0, len(logs))
	for _, row := range logs {
		out = append(out, concurrencyTestLogToResponse(row))
	}
	response.Success(c, gin.H{"items": out})
}

func parseConcurrencyTestID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_CONCURRENCY_TEST_ID", "invalid concurrency test id"))
		return 0, false
	}
	return id, true
}

func parseConcurrencyLimit(raw string, fallback, max int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return fallback
	}
	if v > max {
		return max
	}
	return v
}

func concurrencyTestConfigToResponse(cfg *service.ConcurrencyTestConfig) *concurrencyTestConfigResponse {
	if cfg == nil {
		return nil
	}
	apiKeySet := strings.TrimSpace(cfg.APIKey) != ""
	return &concurrencyTestConfigResponse{
		ID:             cfg.ID,
		Name:           cfg.Name,
		Description:    cfg.Description,
		Mode:           cfg.Mode,
		Concurrency:    cfg.Concurrency,
		AccountIDs:     cfg.AccountIDs,
		Endpoint:       cfg.Endpoint,
		APIKeySet:      apiKeySet,
		APIKeyMasked:   maskConcurrencyAPIKey(cfg.APIKey),
		Method:         cfg.Method,
		Headers:        cfg.Headers,
		BodyTemplate:   cfg.BodyTemplate,
		TimeoutSeconds: cfg.TimeoutSeconds,
		CreatedBy:      cfg.CreatedBy,
		CreatedAt:      formatConcurrencyTime(cfg.CreatedAt),
		UpdatedAt:      formatConcurrencyTime(cfg.UpdatedAt),
		LatestRun:      concurrencyTestRunToResponse(cfg.LatestRun, false),
	}
}

func concurrencyTestRunToResponse(run *service.ConcurrencyTestRun, includeLogs bool) *concurrencyTestRunResponse {
	if run == nil {
		return nil
	}
	out := &concurrencyTestRunResponse{
		ID:              run.ID,
		ConfigID:        run.ConfigID,
		Name:            run.Name,
		Mode:            run.Mode,
		Concurrency:     run.Concurrency,
		AccountIDs:      run.AccountIDs,
		RequestSource:   run.RequestSource,
		Endpoint:        run.Endpoint,
		Method:          run.Method,
		StartedAt:       formatConcurrencyTime(run.StartedAt),
		FinishedAt:      formatConcurrencyTime(run.FinishedAt),
		Status:          run.Status,
		TotalRequests:   run.TotalRequests,
		SuccessCount:    run.SuccessCount,
		FailureCount:    run.FailureCount,
		TimeoutCount:    run.TimeoutCount,
		GatewayTimeouts: run.GatewayTimeouts,
		SuccessRate:     run.SuccessRate,
		AvgLatencyMs:    run.AvgLatencyMs,
		MinLatencyMs:    run.MinLatencyMs,
		MaxLatencyMs:    run.MaxLatencyMs,
		P50LatencyMs:    run.P50LatencyMs,
		P90LatencyMs:    run.P90LatencyMs,
		P95LatencyMs:    run.P95LatencyMs,
		P99LatencyMs:    run.P99LatencyMs,
		Summary:         run.Summary,
		ErrorMessage:    run.ErrorMessage,
		CreatedAt:       formatConcurrencyTime(run.CreatedAt),
	}
	if includeLogs {
		out.Logs = make([]concurrencyTestLogItem, 0, len(run.Logs))
		for _, row := range run.Logs {
			out.Logs = append(out.Logs, concurrencyTestLogToResponse(row))
		}
	}
	return out
}

func concurrencyTestLogToResponse(row *service.ConcurrencyTestLog) concurrencyTestLogItem {
	if row == nil {
		return concurrencyTestLogItem{}
	}
	return concurrencyTestLogItem{
		ID:           row.ID,
		RunID:        row.RunID,
		RequestIndex: row.RequestIndex,
		AccountID:    row.AccountID,
		Endpoint:     row.Endpoint,
		Method:       row.Method,
		StatusCode:   row.StatusCode,
		Success:      row.Success,
		Timeout:      row.Timeout,
		LatencyMs:    row.LatencyMs,
		ErrorMessage: row.ErrorMessage,
		ResponseBody: row.ResponseBody,
		StartedAt:    formatConcurrencyTime(row.StartedAt),
		FinishedAt:   formatConcurrencyTime(row.FinishedAt),
		CreatedAt:    formatConcurrencyTime(row.CreatedAt),
	}
}

func maskConcurrencyAPIKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) <= monitorAPIKeyMaskPrefix {
		return monitorAPIKeyMaskSuffix
	}
	return raw[:monitorAPIKeyMaskPrefix] + monitorAPIKeyMaskSuffix
}

func formatConcurrencyTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
