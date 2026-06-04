package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	ConcurrencyTestModeResponses              = "responses"
	ConcurrencyTestModeOpenAIImageGenerations = "openai_image_generations"
	ConcurrencyTestModeOpenAIImageEdits       = "openai_image_edits"
	ConcurrencyTestModeGeminiImageGenerations = "gemini_image_generations"
	ConcurrencyTestModeGeminiImageEdits       = "gemini_image_edits"

	ConcurrencyTestRequestSourceAccounts = "accounts"
	ConcurrencyTestRequestSourceCustom   = "custom"

	ConcurrencyTestRunStatusCompleted = "completed"
	ConcurrencyTestRunStatusFailed    = "failed"

	concurrencyTestDefaultTimeoutSeconds = 60
	concurrencyTestMaxConcurrency        = 500
	concurrencyTestLogPreviewMaxBytes    = 4096
)

type ConcurrencyTestRepository interface {
	CreateConfig(ctx context.Context, cfg *ConcurrencyTestConfig) error
	UpdateConfig(ctx context.Context, cfg *ConcurrencyTestConfig) error
	DeleteConfig(ctx context.Context, id int64) error
	GetConfig(ctx context.Context, id int64) (*ConcurrencyTestConfig, error)
	ListConfigs(ctx context.Context, params ConcurrencyTestListParams) ([]*ConcurrencyTestConfig, int64, error)
	CreateRun(ctx context.Context, run *ConcurrencyTestRun) error
	ListRuns(ctx context.Context, configID int64, limit int) ([]*ConcurrencyTestRun, error)
	ListLogs(ctx context.Context, runID int64, limit int) ([]*ConcurrencyTestLog, error)
}

type ConcurrencyTestConfig struct {
	ID             int64
	Name           string
	Description    string
	Mode           string
	Concurrency    int
	AccountIDs     []int64
	Endpoint       string
	APIKey         string
	Method         string
	Headers        map[string]string
	BodyTemplate   map[string]any
	TimeoutSeconds int
	CreatedBy      int64
	CreatedAt      time.Time
	UpdatedAt      time.Time

	LatestRun *ConcurrencyTestRun
}

type ConcurrencyTestListParams struct {
	Page     int
	PageSize int
	Search   string
}

type ConcurrencyTestCreateParams struct {
	Name           string
	Description    string
	Mode           string
	Concurrency    int
	AccountIDs     []int64
	Endpoint       string
	APIKey         string
	Method         string
	Headers        map[string]string
	BodyTemplate   map[string]any
	TimeoutSeconds int
	CreatedBy      int64
}

type ConcurrencyTestUpdateParams struct {
	Name           *string
	Description    *string
	Mode           *string
	Concurrency    *int
	AccountIDs     *[]int64
	Endpoint       *string
	APIKey         *string
	Method         *string
	Headers        *map[string]string
	BodyTemplate   *map[string]any
	TimeoutSeconds *int
}

type ConcurrencyTestRun struct {
	ID              int64
	ConfigID        int64
	Name            string
	Mode            string
	Concurrency     int
	AccountIDs      []int64
	RequestSource   string
	Endpoint        string
	Method          string
	StartedAt       time.Time
	FinishedAt      time.Time
	Status          string
	TotalRequests   int
	SuccessCount    int
	FailureCount    int
	TimeoutCount    int
	GatewayTimeouts int
	SuccessRate     float64
	AvgLatencyMs    int
	MinLatencyMs    int
	MaxLatencyMs    int
	P50LatencyMs    int
	P90LatencyMs    int
	P95LatencyMs    int
	P99LatencyMs    int
	Summary         map[string]any
	ErrorMessage    string
	CreatedAt       time.Time

	Logs []*ConcurrencyTestLog
}

type ConcurrencyTestLog struct {
	ID           int64
	RunID        int64
	RequestIndex int
	AccountID    *int64
	Endpoint     string
	Method       string
	StatusCode   *int
	Success      bool
	Timeout      bool
	LatencyMs    int
	ErrorMessage string
	ResponseBody string
	StartedAt    time.Time
	FinishedAt   time.Time
	CreatedAt    time.Time
}

type concurrencyTestTarget struct {
	AccountID  *int64
	Endpoint   string
	APIKey     string
	AuthScheme string
	Method     string
	Headers    map[string]string
	Body       map[string]any
}

var (
	ErrConcurrencyTestNotFound = infraerrors.NotFound(
		"CONCURRENCY_TEST_NOT_FOUND", "concurrency test not found",
	)
	ErrConcurrencyTestRunNotFound = infraerrors.NotFound(
		"CONCURRENCY_TEST_RUN_NOT_FOUND", "concurrency test run not found",
	)
	ErrConcurrencyTestInvalidName = infraerrors.BadRequest(
		"CONCURRENCY_TEST_INVALID_NAME", "name is required",
	)
	ErrConcurrencyTestInvalidMode = infraerrors.BadRequest(
		"CONCURRENCY_TEST_INVALID_MODE", "mode is not supported",
	)
	ErrConcurrencyTestInvalidConcurrency = infraerrors.BadRequest(
		"CONCURRENCY_TEST_INVALID_CONCURRENCY", "concurrency must be between 1 and 500",
	)
	ErrConcurrencyTestMissingTarget = infraerrors.BadRequest(
		"CONCURRENCY_TEST_MISSING_TARGET", "select at least one account or provide custom endpoint and api_key",
	)
	ErrConcurrencyTestInvalidEndpoint = infraerrors.BadRequest(
		"CONCURRENCY_TEST_INVALID_ENDPOINT", "endpoint must be a valid http or https URL",
	)
	ErrConcurrencyTestUnsafeEndpoint = infraerrors.BadRequest(
		"CONCURRENCY_TEST_UNSAFE_ENDPOINT", "endpoint is not allowed",
	)
	ErrConcurrencyTestInvalidBody = infraerrors.BadRequest(
		"CONCURRENCY_TEST_INVALID_BODY", "body_template must be a JSON object",
	)
)
