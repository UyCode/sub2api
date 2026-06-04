package service

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestConcurrencyTestRejectsPrivateEndpoint(t *testing.T) {
	svc := NewConcurrencyTestService(nil, nil, nil, &config.Config{})

	cfg := &ConcurrencyTestConfig{
		Name:           "private endpoint",
		Mode:           ConcurrencyTestModeResponses,
		Concurrency:    1,
		Endpoint:       "https://127.0.0.1/v1/responses",
		APIKey:         "sk-test",
		Method:         http.MethodPost,
		Headers:        map[string]string{},
		BodyTemplate:   map[string]any{"input": "ok"},
		TimeoutSeconds: 30,
	}

	require.ErrorIs(t, svc.validateConcurrencyTestConfig(cfg), ErrConcurrencyTestUnsafeEndpoint)
}

func TestConcurrencyTestGeminiOAuthUsesBearer(t *testing.T) {
	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "ya29.token",
		},
	}

	headers := headersForConcurrencyTest(
		authSchemeForAccountConcurrencyTest(account, ConcurrencyTestModeGeminiImageGenerations),
		accountAPIKeyForConcurrencyTest(account),
		nil,
		"application/json",
	)

	require.Equal(t, "Bearer ya29.token", headers["Authorization"])
	require.Empty(t, headers["x-goog-api-key"])
}

func TestConcurrencyTestGeminiBodyDropsModelAndBackfillsInlineData(t *testing.T) {
	body := map[string]any{
		"model": "gemini-2.0-flash-preview-image-generation",
		"contents": []any{map[string]any{
			"parts": []any{
				map[string]any{"text": "edit"},
				map[string]any{"inline_data": map[string]any{"mime_type": "image/png", "data": ""}},
			},
		}},
	}

	encoded, contentType, err := encodeConcurrencyTestRequestBody(
		ConcurrencyTestModeGeminiImageEdits,
		prepareConcurrencyTestBodyForSend(ConcurrencyTestModeGeminiImageEdits, body),
	)
	require.NoError(t, err)
	require.Equal(t, "application/json", contentType)

	raw, err := io.ReadAll(encoded)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"model"`)
	require.Contains(t, string(raw), concurrencyTestDefaultPNGBase64)
}

func TestConcurrencyTestLogRedaction(t *testing.T) {
	raw := `{"error":"bad","api_key":"sk-secret","authorization":"Bearer sk-secret","url":"https://example.com?key=sk-secret"}`

	redacted := redactConcurrencyLog(raw, "sk-secret")

	require.NotContains(t, redacted, "sk-secret")
	require.Contains(t, redacted, "***")
}

func TestConcurrencyTestOpenAIImageEditMultipart(t *testing.T) {
	reader, contentType, err := encodeConcurrencyTestRequestBody(ConcurrencyTestModeOpenAIImageEdits, map[string]any{
		"model":          "gpt-image-1",
		"prompt":         "edit",
		"image_base64":   concurrencyTestDefaultPNGBase64,
		"image_filename": "input.png",
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(contentType, "multipart/form-data; boundary="))

	buf := bytes.Buffer{}
	_, err = buf.ReadFrom(reader)
	require.NoError(t, err)
	require.Contains(t, buf.String(), `name="image"; filename="input.png"`)
	require.Contains(t, buf.String(), `name="prompt"`)
}

func TestConcurrencyTestGeminiDefaultBodyIsNativeShape(t *testing.T) {
	body, err := requestBodyForMode(ConcurrencyTestModeGeminiImageGenerations, nil)
	require.NoError(t, err)

	raw, err := json.Marshal(body)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"model"`)
	require.Contains(t, string(raw), `"contents"`)
}
