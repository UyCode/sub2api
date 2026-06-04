package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

const concurrencyTestDefaultPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII="

const (
	concurrencyTestAuthSchemeBearer       = "bearer"
	concurrencyTestAuthSchemeGoogleAPIKey = "google_api_key"
)

type ConcurrencyTestService struct {
	repo        ConcurrencyTestRepository
	accountRepo AccountRepository
	encryptor   SecretEncryptor
	cfg         *config.Config
}

func NewConcurrencyTestService(repo ConcurrencyTestRepository, accountRepo AccountRepository, encryptor SecretEncryptor, cfg *config.Config) *ConcurrencyTestService {
	return &ConcurrencyTestService{repo: repo, accountRepo: accountRepo, encryptor: encryptor, cfg: cfg}
}

func (s *ConcurrencyTestService) List(ctx context.Context, params ConcurrencyTestListParams) ([]*ConcurrencyTestConfig, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}
	items, total, err := s.repo.ListConfigs(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	for _, item := range items {
		s.decryptCustomAPIKeyInPlace(item)
	}
	return items, total, nil
}

func (s *ConcurrencyTestService) Get(ctx context.Context, id int64) (*ConcurrencyTestConfig, error) {
	cfg, err := s.repo.GetConfig(ctx, id)
	if err != nil {
		return nil, err
	}
	s.decryptCustomAPIKeyInPlace(cfg)
	return cfg, nil
}

func (s *ConcurrencyTestService) Create(ctx context.Context, p ConcurrencyTestCreateParams) (*ConcurrencyTestConfig, error) {
	cfg := &ConcurrencyTestConfig{
		Name:           strings.TrimSpace(p.Name),
		Description:    strings.TrimSpace(p.Description),
		Mode:           normalizeConcurrencyTestMode(p.Mode),
		Concurrency:    p.Concurrency,
		AccountIDs:     normalizeConcurrencyTestInt64IDs(p.AccountIDs),
		Endpoint:       strings.TrimSpace(p.Endpoint),
		APIKey:         strings.TrimSpace(p.APIKey),
		Method:         normalizeConcurrencyTestMethod(p.Method),
		Headers:        normalizeStringMap(p.Headers),
		BodyTemplate:   normalizeBodyTemplate(p.BodyTemplate),
		TimeoutSeconds: normalizeConcurrencyTestTimeout(p.TimeoutSeconds),
		CreatedBy:      p.CreatedBy,
	}
	if err := s.validateConcurrencyTestConfig(cfg); err != nil {
		return nil, err
	}
	if len(cfg.AccountIDs) > 0 {
		cfg.APIKey = ""
	}
	plainAPIKey := cfg.APIKey
	if err := s.encryptCustomAPIKeyInPlace(cfg); err != nil {
		return nil, err
	}
	if err := s.repo.CreateConfig(ctx, cfg); err != nil {
		return nil, err
	}
	cfg.APIKey = plainAPIKey
	return cfg, nil
}

func (s *ConcurrencyTestService) Update(ctx context.Context, id int64, p ConcurrencyTestUpdateParams) (*ConcurrencyTestConfig, error) {
	cfg, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	applyConcurrencyTestUpdate(cfg, p)
	if err := s.validateConcurrencyTestConfig(cfg); err != nil {
		return nil, err
	}
	if len(cfg.AccountIDs) > 0 {
		cfg.APIKey = ""
	}
	plainAPIKey := cfg.APIKey
	if err := s.encryptCustomAPIKeyInPlace(cfg); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateConfig(ctx, cfg); err != nil {
		return nil, err
	}
	cfg.APIKey = plainAPIKey
	return cfg, nil
}

func (s *ConcurrencyTestService) Delete(ctx context.Context, id int64) error {
	return s.repo.DeleteConfig(ctx, id)
}

func (s *ConcurrencyTestService) ListRuns(ctx context.Context, configID int64, limit int) ([]*ConcurrencyTestRun, error) {
	if _, err := s.repo.GetConfig(ctx, configID); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListRuns(ctx, configID, limit)
}

func (s *ConcurrencyTestService) ListLogs(ctx context.Context, runID int64, limit int) ([]*ConcurrencyTestLog, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	return s.repo.ListLogs(ctx, runID, limit)
}

func (s *ConcurrencyTestService) Run(ctx context.Context, id int64) (*ConcurrencyTestRun, error) {
	cfg, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	targets, requestSource, err := s.resolveTargets(ctx, cfg)
	if err != nil {
		return nil, err
	}
	client, err := s.httpClient(cfg.TimeoutSeconds)
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutSeconds)*time.Second+10*time.Second)
	defer cancel()

	started := time.Now().UTC()
	logs := executeConcurrencyTest(runCtx, client, cfg, targets)
	finished := time.Now().UTC()
	run := buildConcurrencyTestRun(cfg, requestSource, started, finished, logs)
	if err := s.repo.CreateRun(ctx, run); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *ConcurrencyTestService) resolveTargets(ctx context.Context, cfg *ConcurrencyTestConfig) ([]concurrencyTestTarget, string, error) {
	if len(cfg.AccountIDs) == 0 {
		if strings.TrimSpace(cfg.Endpoint) == "" || strings.TrimSpace(cfg.APIKey) == "" {
			return nil, "", ErrConcurrencyTestMissingTarget
		}
		body, err := requestBodyForMode(cfg.Mode, cfg.BodyTemplate)
		if err != nil {
			return nil, "", err
		}
		return []concurrencyTestTarget{{
			Endpoint:   cfg.Endpoint,
			APIKey:     cfg.APIKey,
			AuthScheme: authSchemeForCustomConcurrencyTest(cfg.Mode),
			Method:     normalizeConcurrencyTestMethod(cfg.Method),
			Headers:    normalizeStringMap(cfg.Headers),
			Body:       body,
		}}, ConcurrencyTestRequestSourceCustom, nil
	}

	accounts, err := s.accountRepo.GetByIDs(ctx, cfg.AccountIDs)
	if err != nil {
		return nil, "", err
	}
	if len(accounts) == 0 {
		return nil, "", ErrConcurrencyTestMissingTarget
	}

	targets := make([]concurrencyTestTarget, 0, len(accounts))
	for _, account := range accounts {
		target, ok, err := targetForAccount(cfg, account)
		if err != nil {
			return nil, "", err
		}
		if ok {
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 {
		return nil, "", ErrConcurrencyTestMissingTarget
	}
	return targets, ConcurrencyTestRequestSourceAccounts, nil
}

func targetForAccount(cfg *ConcurrencyTestConfig, account *Account) (concurrencyTestTarget, bool, error) {
	if account == nil {
		return concurrencyTestTarget{}, false, nil
	}
	if !accountMatchesConcurrencyTestMode(account, cfg.Mode) {
		return concurrencyTestTarget{}, false, nil
	}
	apiKey := accountAPIKeyForConcurrencyTest(account)
	if strings.TrimSpace(apiKey) == "" {
		return concurrencyTestTarget{}, false, nil
	}
	body, err := requestBodyForMode(cfg.Mode, cfg.BodyTemplate)
	if err != nil {
		return concurrencyTestTarget{}, false, err
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = defaultEndpointForAccountMode(account, cfg.Mode, body)
	}
	if endpoint == "" {
		return concurrencyTestTarget{}, false, nil
	}
	accountID := account.ID
	return concurrencyTestTarget{
		AccountID:  &accountID,
		Endpoint:   endpoint,
		APIKey:     apiKey,
		AuthScheme: authSchemeForAccountConcurrencyTest(account, cfg.Mode),
		Method:     normalizeConcurrencyTestMethod(cfg.Method),
		Headers:    normalizeStringMap(cfg.Headers),
		Body:       body,
	}, true, nil
}

func executeConcurrencyTest(ctx context.Context, client *http.Client, cfg *ConcurrencyTestConfig, targets []concurrencyTestTarget) []*ConcurrencyTestLog {
	logs := make([]*ConcurrencyTestLog, cfg.Concurrency)
	jobs := make(chan int)
	workers := cfg.Concurrency

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				target := targets[idx%len(targets)]
				logs[idx] = executeConcurrencyTestRequest(ctx, client, cfg, idx, target)
			}
		}()
	}
	for i := 0; i < cfg.Concurrency; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return logs
}

func executeConcurrencyTestRequest(ctx context.Context, client *http.Client, cfg *ConcurrencyTestConfig, idx int, target concurrencyTestTarget) *ConcurrencyTestLog {
	started := time.Now().UTC()
	log := &ConcurrencyTestLog{
		RequestIndex: idx + 1,
		AccountID:    target.AccountID,
		Endpoint:     redactConcurrencyLog(target.Endpoint, target.APIKey),
		Method:       target.Method,
		StartedAt:    started,
	}

	body := materializeTemplate(target.Body, idx+1, target.AccountID)
	body = prepareConcurrencyTestBodyForSend(cfg.Mode, body)
	rawBody, contentType, err := encodeConcurrencyTestRequestBody(cfg.Mode, body)
	if err != nil {
		log.FinishedAt = time.Now().UTC()
		log.ErrorMessage = redactConcurrencyLog(fmt.Sprintf("build request body: %v", err), target.APIKey)
		log.LatencyMs = int(time.Since(started) / time.Millisecond)
		return log
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, target.Method, target.Endpoint, rawBody)
	if err != nil {
		log.FinishedAt = time.Now().UTC()
		log.ErrorMessage = redactConcurrencyLog(err.Error(), target.APIKey)
		log.LatencyMs = int(time.Since(started) / time.Millisecond)
		return log
	}
	headers := headersForConcurrencyTest(target.AuthScheme, target.APIKey, target.Headers, contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	log.FinishedAt = time.Now().UTC()
	log.LatencyMs = int(log.FinishedAt.Sub(started) / time.Millisecond)
	if err != nil {
		log.Timeout = reqCtx.Err() == context.DeadlineExceeded || strings.Contains(strings.ToLower(err.Error()), "timeout")
		log.ErrorMessage = redactConcurrencyLog(err.Error(), target.APIKey)
		return log
	}
	defer func() { _ = resp.Body.Close() }()
	statusCode := resp.StatusCode
	log.StatusCode = &statusCode
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, concurrencyTestLogPreviewMaxBytes))
	if readErr != nil {
		log.ErrorMessage = redactConcurrencyLog(readErr.Error(), target.APIKey)
	}
	log.ResponseBody = redactConcurrencyLog(string(data), target.APIKey)
	log.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	if !log.Success && log.ErrorMessage == "" {
		log.ErrorMessage = redactConcurrencyLog(http.StatusText(resp.StatusCode), target.APIKey)
	}
	return log
}

func buildConcurrencyTestRun(cfg *ConcurrencyTestConfig, source string, started, finished time.Time, logs []*ConcurrencyTestLog) *ConcurrencyTestRun {
	run := &ConcurrencyTestRun{
		ConfigID:      cfg.ID,
		Name:          cfg.Name,
		Mode:          cfg.Mode,
		Concurrency:   cfg.Concurrency,
		AccountIDs:    cfg.AccountIDs,
		RequestSource: source,
		Endpoint:      redactConcurrencyLog(cfg.Endpoint, ""),
		Method:        cfg.Method,
		StartedAt:     started,
		FinishedAt:    finished,
		Status:        ConcurrencyTestRunStatusCompleted,
		TotalRequests: len(logs),
		Logs:          logs,
	}
	latencies := make([]int, 0, len(logs))
	statusCodes := map[string]int{}
	for _, row := range logs {
		if row == nil {
			run.FailureCount++
			continue
		}
		if row.Success {
			run.SuccessCount++
		} else {
			run.FailureCount++
		}
		if row.Timeout {
			run.TimeoutCount++
		}
		if row.StatusCode != nil {
			statusCodes[strconv.Itoa(*row.StatusCode)]++
			if *row.StatusCode == http.StatusGatewayTimeout {
				run.GatewayTimeouts++
			}
		}
		latencies = append(latencies, row.LatencyMs)
	}
	if run.TotalRequests > 0 {
		run.SuccessRate = float64(run.SuccessCount) * 100 / float64(run.TotalRequests)
	}
	fillLatencyStats(run, latencies)
	run.Summary = map[string]any{
		"status_codes": statusCodes,
		"duration_ms":  int(finished.Sub(started) / time.Millisecond),
	}
	return run
}

func fillLatencyStats(run *ConcurrencyTestRun, latencies []int) {
	if len(latencies) == 0 {
		return
	}
	sort.Ints(latencies)
	run.MinLatencyMs = latencies[0]
	run.MaxLatencyMs = latencies[len(latencies)-1]
	total := 0
	for _, v := range latencies {
		total += v
	}
	run.AvgLatencyMs = total / len(latencies)
	run.P50LatencyMs = percentileLatency(latencies, 50)
	run.P90LatencyMs = percentileLatency(latencies, 90)
	run.P95LatencyMs = percentileLatency(latencies, 95)
	run.P99LatencyMs = percentileLatency(latencies, 99)
}

func percentileLatency(sorted []int, pct int) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := (len(sorted)*pct + 99) / 100
	if idx < 1 {
		idx = 1
	}
	if idx > len(sorted) {
		idx = len(sorted)
	}
	return sorted[idx-1]
}

func (s *ConcurrencyTestService) validateConcurrencyTestConfig(cfg *ConcurrencyTestConfig) error {
	if cfg == nil || strings.TrimSpace(cfg.Name) == "" {
		return ErrConcurrencyTestInvalidName
	}
	if !isSupportedConcurrencyTestMode(cfg.Mode) {
		return ErrConcurrencyTestInvalidMode
	}
	if cfg.Concurrency < 1 || cfg.Concurrency > concurrencyTestMaxConcurrency {
		return ErrConcurrencyTestInvalidConcurrency
	}
	if len(cfg.AccountIDs) == 0 && (strings.TrimSpace(cfg.Endpoint) == "" || strings.TrimSpace(cfg.APIKey) == "") {
		return ErrConcurrencyTestMissingTarget
	}
	if strings.TrimSpace(cfg.Endpoint) != "" {
		normalized, err := s.validateConcurrencyTestEndpoint(cfg.Endpoint)
		if err != nil {
			return err
		}
		cfg.Endpoint = normalized
	}
	if cfg.BodyTemplate == nil {
		return ErrConcurrencyTestInvalidBody
	}
	return nil
}

func (s *ConcurrencyTestService) encryptCustomAPIKeyInPlace(cfg *ConcurrencyTestConfig) error {
	if cfg == nil || len(cfg.AccountIDs) > 0 || strings.TrimSpace(cfg.APIKey) == "" {
		return nil
	}
	encrypted, err := s.encryptor.Encrypt(strings.TrimSpace(cfg.APIKey))
	if err != nil {
		return fmt.Errorf("encrypt concurrency test api key: %w", err)
	}
	cfg.APIKey = encrypted
	return nil
}

func (s *ConcurrencyTestService) decryptCustomAPIKeyInPlace(cfg *ConcurrencyTestConfig) {
	if cfg == nil || len(cfg.AccountIDs) > 0 || strings.TrimSpace(cfg.APIKey) == "" {
		return
	}
	plain, err := s.encryptor.Decrypt(cfg.APIKey)
	if err != nil {
		cfg.APIKey = ""
		return
	}
	cfg.APIKey = plain
}

func (s *ConcurrencyTestService) validateConcurrencyTestEndpoint(raw string) (string, error) {
	allowInsecureHTTP := false
	allowedHosts := []string(nil)
	requireAllowlist := false
	if s != nil && s.cfg != nil && s.cfg.Security.URLAllowlist.Enabled {
		allowedHosts = s.cfg.Security.URLAllowlist.UpstreamHosts
		requireAllowlist = true
	}
	normalized, err := urlvalidator.ValidateHTTPURL(raw, allowInsecureHTTP, urlvalidator.ValidationOptions{
		AllowedHosts:     allowedHosts,
		RequireAllowlist: requireAllowlist,
		AllowPrivate:     false,
	})
	if err != nil {
		errText := strings.ToLower(err.Error())
		if strings.Contains(errText, "not allowed") || strings.Contains(errText, "allowlist") {
			return "", ErrConcurrencyTestUnsafeEndpoint
		}
		return "", ErrConcurrencyTestInvalidEndpoint
	}
	return normalized, nil
}

func applyConcurrencyTestUpdate(cfg *ConcurrencyTestConfig, p ConcurrencyTestUpdateParams) {
	if p.Name != nil {
		cfg.Name = strings.TrimSpace(*p.Name)
	}
	if p.Description != nil {
		cfg.Description = strings.TrimSpace(*p.Description)
	}
	if p.Mode != nil {
		cfg.Mode = normalizeConcurrencyTestMode(*p.Mode)
	}
	if p.Concurrency != nil {
		cfg.Concurrency = *p.Concurrency
	}
	if p.AccountIDs != nil {
		cfg.AccountIDs = normalizeConcurrencyTestInt64IDs(*p.AccountIDs)
	}
	if p.Endpoint != nil {
		cfg.Endpoint = strings.TrimSpace(*p.Endpoint)
	}
	if p.APIKey != nil {
		cfg.APIKey = strings.TrimSpace(*p.APIKey)
	}
	if p.Method != nil {
		cfg.Method = normalizeConcurrencyTestMethod(*p.Method)
	}
	if p.Headers != nil {
		cfg.Headers = normalizeStringMap(*p.Headers)
	}
	if p.BodyTemplate != nil {
		cfg.BodyTemplate = normalizeBodyTemplate(*p.BodyTemplate)
	}
	if p.TimeoutSeconds != nil {
		cfg.TimeoutSeconds = normalizeConcurrencyTestTimeout(*p.TimeoutSeconds)
	}
}

func requestBodyForMode(mode string, body map[string]any) (map[string]any, error) {
	if len(body) > 0 {
		return cloneMap(body), nil
	}
	switch mode {
	case ConcurrencyTestModeResponses:
		return map[string]any{
			"model":             "gpt-4.1-mini",
			"input":             "Return the word ok.",
			"max_output_tokens": 16,
		}, nil
	case ConcurrencyTestModeOpenAIImageGenerations:
		return map[string]any{
			"model":  "gpt-image-1",
			"prompt": "A small blue cube on a white background.",
			"size":   "1024x1024",
			"n":      1,
		}, nil
	case ConcurrencyTestModeOpenAIImageEdits:
		return map[string]any{
			"model":          "gpt-image-1",
			"prompt":         "Make the object brighter.",
			"image_base64":   concurrencyTestDefaultPNGBase64,
			"image_filename": "input.png",
		}, nil
	case ConcurrencyTestModeGeminiImageGenerations:
		return map[string]any{
			"contents": []any{map[string]any{
				"parts": []any{map[string]any{"text": "Generate a small blue cube on a white background."}},
			}},
		}, nil
	case ConcurrencyTestModeGeminiImageEdits:
		return map[string]any{
			"contents": []any{map[string]any{
				"parts": []any{
					map[string]any{"text": "Edit the supplied image according to the prompt."},
					map[string]any{"inline_data": map[string]any{
						"mime_type": "image/png",
						"data":      concurrencyTestDefaultPNGBase64,
					}},
				},
			}},
		}, nil
	default:
		return nil, ErrConcurrencyTestInvalidMode
	}
}

func headersForConcurrencyTest(authScheme, apiKey string, extra map[string]string, contentType string) map[string]string {
	headers := map[string]string{}
	switch authScheme {
	case concurrencyTestAuthSchemeGoogleAPIKey:
		headers["x-goog-api-key"] = apiKey
	default:
		headers["Authorization"] = "Bearer " + apiKey
	}
	for k, v := range extra {
		if strings.TrimSpace(k) == "" {
			continue
		}
		headers[k] = v
	}
	headers["Content-Type"] = contentType
	return headers
}

func authSchemeForCustomConcurrencyTest(mode string) string {
	if isGeminiConcurrencyTestMode(mode) {
		return concurrencyTestAuthSchemeGoogleAPIKey
	}
	return concurrencyTestAuthSchemeBearer
}

func authSchemeForAccountConcurrencyTest(account *Account, mode string) string {
	if account != nil && account.Platform == PlatformGemini && account.Type == AccountTypeAPIKey && isGeminiConcurrencyTestMode(mode) {
		return concurrencyTestAuthSchemeGoogleAPIKey
	}
	return concurrencyTestAuthSchemeBearer
}

func isGeminiConcurrencyTestMode(mode string) bool {
	return mode == ConcurrencyTestModeGeminiImageGenerations || mode == ConcurrencyTestModeGeminiImageEdits
}

func isOpenAIConcurrencyTestMode(mode string) bool {
	return mode == ConcurrencyTestModeResponses ||
		mode == ConcurrencyTestModeOpenAIImageGenerations ||
		mode == ConcurrencyTestModeOpenAIImageEdits
}

func accountMatchesConcurrencyTestMode(account *Account, mode string) bool {
	if account == nil {
		return false
	}
	if isGeminiConcurrencyTestMode(mode) {
		return account.Platform == PlatformGemini
	}
	if isOpenAIConcurrencyTestMode(mode) {
		return account.Platform == PlatformOpenAI
	}
	return false
}

func prepareConcurrencyTestBodyForSend(mode string, body map[string]any) map[string]any {
	if !isGeminiConcurrencyTestMode(mode) {
		return body
	}
	out := cloneMap(body)
	delete(out, "model")
	if mode == ConcurrencyTestModeGeminiImageEdits {
		ensureGeminiInlineData(out)
	}
	return out
}

func ensureGeminiInlineData(body map[string]any) {
	contents, ok := body["contents"].([]any)
	if !ok {
		return
	}
	for _, content := range contents {
		contentMap, ok := content.(map[string]any)
		if !ok {
			continue
		}
		parts, ok := contentMap["parts"].([]any)
		if !ok {
			continue
		}
		for _, part := range parts {
			partMap, ok := part.(map[string]any)
			if !ok {
				continue
			}
			inlineData, ok := partMap["inline_data"].(map[string]any)
			if !ok {
				continue
			}
			if strings.TrimSpace(concurrencyTestStringFromAny(inlineData["mime_type"])) == "" {
				inlineData["mime_type"] = "image/png"
			}
			if strings.TrimSpace(concurrencyTestStringFromAny(inlineData["data"])) == "" {
				inlineData["data"] = concurrencyTestDefaultPNGBase64
			}
		}
	}
}

func encodeConcurrencyTestRequestBody(mode string, body map[string]any) (io.Reader, string, error) {
	if mode == ConcurrencyTestModeOpenAIImageEdits {
		return encodeOpenAIImageEditMultipart(body)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, "", err
	}
	return bytes.NewReader(raw), "application/json", nil
}

func encodeOpenAIImageEditMultipart(body map[string]any) (io.Reader, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for key, value := range body {
		switch key {
		case "image_base64", "image_filename":
			continue
		default:
			if err := writer.WriteField(key, stringifyMultipartField(value)); err != nil {
				return nil, "", err
			}
		}
	}
	imageBase64 := concurrencyTestStringFromAny(body["image_base64"])
	if imageBase64 == "" {
		imageBase64 = concurrencyTestDefaultPNGBase64
	}
	imageBytes, err := base64.StdEncoding.DecodeString(stripDataURLPrefix(imageBase64))
	if err != nil {
		return nil, "", err
	}
	filename := concurrencyTestStringFromAny(body["image_filename"])
	if filename == "" {
		filename = "input.png"
	}
	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(imageBytes); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &buf, writer.FormDataContentType(), nil
}

func stringifyMultipartField(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(raw)
	}
}

func concurrencyTestStringFromAny(value any) string {
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func stripDataURLPrefix(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, ","); strings.HasPrefix(strings.ToLower(s), "data:") && idx >= 0 {
		return s[idx+1:]
	}
	return s
}

func accountAPIKeyForConcurrencyTest(account *Account) string {
	if account == nil {
		return ""
	}
	if account.IsOAuth() {
		return account.GetCredential("access_token")
	}
	if key := account.GetCredential("api_key"); key != "" {
		return key
	}
	if key := account.GetCredential("key"); key != "" {
		return key
	}
	return ""
}

func defaultEndpointForAccountMode(account *Account, mode string, body map[string]any) string {
	baseURL := strings.TrimRight(accountBaseURLForConcurrencyTest(account), "/")
	switch mode {
	case ConcurrencyTestModeGeminiImageGenerations, ConcurrencyTestModeGeminiImageEdits:
		model := modelFromBodyTemplate(body, "gemini-2.0-flash-preview-image-generation")
		if baseURL == "" {
			baseURL = "https://generativelanguage.googleapis.com"
		}
		return baseURL + fmt.Sprintf(providerGeminiPathTemplate, model)
	case ConcurrencyTestModeResponses:
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		return baseURL + providerOpenAIResponsesPath
	case ConcurrencyTestModeOpenAIImageGenerations:
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		return baseURL + "/v1/images/generations"
	case ConcurrencyTestModeOpenAIImageEdits:
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		return baseURL + "/v1/images/edits"
	default:
		return ""
	}
}

func accountBaseURLForConcurrencyTest(account *Account) string {
	if account == nil {
		return ""
	}
	baseURL := strings.TrimSpace(account.GetCredential("base_url"))
	if baseURL == "" {
		return ""
	}
	if account.Platform == PlatformAntigravity && account.Type == AccountTypeAPIKey {
		return strings.TrimRight(baseURL, "/") + "/antigravity"
	}
	return baseURL
}

func modelFromBodyTemplate(body map[string]any, fallback string) string {
	if body != nil {
		if v, ok := body["model"].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return fallback
}

func (s *ConcurrencyTestService) httpClient(timeoutSeconds int) (*http.Client, error) {
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(concurrencyTestDefaultTimeoutSeconds) * time.Second
	}
	return httpclient.GetClient(httpclient.Options{
		Timeout:               timeout + 2*time.Second,
		ResponseHeaderTimeout: timeout,
		ValidateResolvedIP:    true,
		AllowPrivateHosts:     false,
		MaxIdleConns:          512,
		MaxIdleConnsPerHost:   512,
	})
}

func normalizeConcurrencyTestMode(mode string) string {
	return strings.TrimSpace(mode)
}

func normalizeConcurrencyTestMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return http.MethodPost
	}
	return method
}

func normalizeConcurrencyTestTimeout(v int) int {
	if v <= 0 {
		return concurrencyTestDefaultTimeoutSeconds
	}
	if v > 600 {
		return 600
	}
	return v
}

func isSupportedConcurrencyTestMode(mode string) bool {
	switch mode {
	case ConcurrencyTestModeResponses,
		ConcurrencyTestModeOpenAIImageGenerations,
		ConcurrencyTestModeOpenAIImageEdits,
		ConcurrencyTestModeGeminiImageGenerations,
		ConcurrencyTestModeGeminiImageEdits:
		return true
	default:
		return false
	}
}

func normalizeStringMap(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		out[k] = v
	}
	return out
}

func normalizeBodyTemplate(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return cloneMap(in)
}

func normalizeConcurrencyTestInt64IDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func materializeTemplate(body map[string]any, requestIndex int, accountID *int64) map[string]any {
	var accountText string
	if accountID != nil {
		accountText = strconv.FormatInt(*accountID, 10)
	}
	replacer := strings.NewReplacer(
		"{{request_index}}", strconv.Itoa(requestIndex),
		"{{account_id}}", accountText,
		"{{timestamp}}", time.Now().UTC().Format(time.RFC3339),
	)
	return replaceTemplateStrings(body, replacer).(map[string]any)
}

func replaceTemplateStrings(value any, replacer *strings.Replacer) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			out[key] = replaceTemplateStrings(val, replacer)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, val := range v {
			out[i] = replaceTemplateStrings(val, replacer)
		}
		return out
	case string:
		return replacer.Replace(v)
	default:
		return v
	}
}

func truncateConcurrencyLog(s string) string {
	if len(s) <= concurrencyTestLogPreviewMaxBytes {
		return s
	}
	return s[:concurrencyTestLogPreviewMaxBytes]
}

func redactConcurrencyLog(s string, apiKey string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	redacted := logredact.RedactText(s, "api_key", "key", "authorization", "x-goog-api-key")
	if apiKey = strings.TrimSpace(apiKey); apiKey != "" {
		redacted = strings.ReplaceAll(redacted, apiKey, "***")
	}
	return truncateConcurrencyLog(redacted)
}
