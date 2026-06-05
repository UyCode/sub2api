package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type geminiImageAssetTransformOptions struct {
	Enabled                    bool
	UpstreamSupportsURLAssets  bool
	ResponseFormat             string
	StripResponseFormatRequest bool
}

func normalizeGeminiImageResponseFormat(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "b64_json", "inline_data", "inlinedata", "base64":
		return "b64_json"
	case "url", "file_data", "filedata":
		return "url"
	default:
		return "url"
	}
}

func extractGeminiImageResponseFormat(body []byte) string {
	if len(body) == 0 || !gjsonValidBytes(body) {
		return ""
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ""
	}
	raw, _ := decoded["response_format"].(string)
	return strings.TrimSpace(raw)
}

func transformGeminiImageAssetRequest(ctx context.Context, uploader imageAssetUploader, body []byte) ([]byte, bool, error) {
	return transformGeminiImageAssetRequestWithOptions(ctx, uploader, body, geminiImageAssetTransformOptions{UpstreamSupportsURLAssets: true})
}

func transformGeminiImageAssetRequestWithOptions(ctx context.Context, uploader imageAssetUploader, body []byte, opts geminiImageAssetTransformOptions) ([]byte, bool, error) {
	return transformGeminiImageAssetJSON(ctx, uploader, body, "request", opts)
}

func transformGeminiImageAssetResponse(ctx context.Context, uploader imageAssetUploader, body []byte) ([]byte, bool, error) {
	return transformGeminiImageAssetResponseWithOptions(ctx, uploader, body, geminiImageAssetTransformOptions{ResponseFormat: "url"})
}

func transformGeminiImageAssetResponseWithOptions(ctx context.Context, uploader imageAssetUploader, body []byte, opts geminiImageAssetTransformOptions) ([]byte, bool, error) {
	return transformGeminiImageAssetJSON(ctx, uploader, body, "response", opts)
}

func transformGeminiImageAssetJSON(ctx context.Context, uploader imageAssetUploader, body []byte, kind string, opts geminiImageAssetTransformOptions) ([]byte, bool, error) {
	if len(body) == 0 || !gjsonValidBytes(body) {
		return body, false, nil
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return body, false, nil
	}
	changed, err := walkGeminiImageAssetJSON(ctx, uploader, decoded, kind, opts)
	if err != nil {
		return nil, false, err
	}
	if kind == "request" && opts.StripResponseFormatRequest {
		if root, ok := decoded.(map[string]any); ok {
			if _, exists := root["response_format"]; exists {
				delete(root, "response_format")
				changed = true
			}
		}
	}
	if !changed {
		return body, false, nil
	}
	out, err := json.Marshal(decoded)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

func walkGeminiImageAssetJSON(ctx context.Context, uploader imageAssetUploader, node any, kind string, opts geminiImageAssetTransformOptions) (bool, error) {
	switch value := node.(type) {
	case map[string]any:
		changed := false
		snakeInlineChanged := false
		camelInlineChanged := false
		if childChanged, err := rewriteGeminiInlineDataPart(ctx, uploader, value, kind, opts, "inline_data", "file_data", "mime_type", "file_uri"); err != nil {
			return false, err
		} else if childChanged {
			changed = true
			snakeInlineChanged = true
		}
		if childChanged, err := rewriteGeminiInlineDataPart(ctx, uploader, value, kind, opts, "inlineData", "fileData", "mimeType", "fileUri"); err != nil {
			return false, err
		} else if childChanged {
			changed = true
			camelInlineChanged = true
		}
		if !snakeInlineChanged {
			if childChanged, err := rewriteGeminiFileDataURL(ctx, uploader, value, kind, opts, "file_data", "mime_type", "file_uri"); err != nil {
				return false, err
			} else if childChanged {
				changed = true
			}
		}
		if !camelInlineChanged {
			if childChanged, err := rewriteGeminiFileDataURL(ctx, uploader, value, kind, opts, "fileData", "mimeType", "fileUri"); err != nil {
				return false, err
			} else if childChanged {
				changed = true
			}
		}
		for _, child := range value {
			childChanged, err := walkGeminiImageAssetJSON(ctx, uploader, child, kind, opts)
			if err != nil {
				return false, err
			}
			changed = changed || childChanged
		}
		return changed, nil
	case []any:
		changed := false
		for _, child := range value {
			childChanged, err := walkGeminiImageAssetJSON(ctx, uploader, child, kind, opts)
			if err != nil {
				return false, err
			}
			changed = changed || childChanged
		}
		return changed, nil
	default:
		return false, nil
	}
}

func rewriteGeminiInlineDataPart(ctx context.Context, uploader imageAssetUploader, part map[string]any, kind string, opts geminiImageAssetTransformOptions, inlineKey, fileKey, mimeKey, uriKey string) (bool, error) {
	inlineData, ok := part[inlineKey].(map[string]any)
	if !ok {
		return false, nil
	}
	raw, ok := inlineData["data"].(string)
	if !ok {
		return false, nil
	}
	contentType := firstNonEmptyString(inlineData[mimeKey], inlineData["mime_type"], inlineData["mimeType"])
	data, decodedContentType, decoded, err := decodeImageAssetString(raw, contentType)
	if err != nil {
		return false, err
	}
	if !decoded {
		return false, nil
	}
	if kind == "request" && !opts.UpstreamSupportsURLAssets {
		return false, nil
	}
	if kind == "response" && normalizeGeminiImageResponseFormat(opts.ResponseFormat) == "b64_json" {
		return false, nil
	}
	if uploader == nil {
		return false, infraerrors.InternalServer("IMAGE_ASSET_STORAGE_NOT_CONFIGURED", "image asset storage is not configured")
	}
	url, _, err := uploader.UploadImageAsset(ctx, kind, data, decodedContentType)
	if err != nil {
		return false, err
	}
	part[fileKey] = map[string]any{
		mimeKey: decodedContentType,
		uriKey:  url,
	}
	delete(part, inlineKey)
	return true, nil
}

func rewriteGeminiFileDataURL(ctx context.Context, uploader imageAssetUploader, node map[string]any, kind string, opts geminiImageAssetTransformOptions, fileKey, mimeKey, uriKey string) (bool, error) {
	fileData, ok := node[fileKey].(map[string]any)
	if !ok {
		return false, nil
	}
	uri, ok := fileData[uriKey].(string)
	if !ok {
		return false, nil
	}
	isDataURL := strings.HasPrefix(strings.ToLower(strings.TrimSpace(uri)), "data:")
	isHTTPURL := isImageAssetHTTPURL(uri)
	if !isDataURL && !isHTTPURL {
		return false, nil
	}
	if kind == "request" && isHTTPURL && opts.UpstreamSupportsURLAssets {
		return false, nil
	}
	contentType := firstNonEmptyString(fileData[mimeKey], fileData["mime_type"], fileData["mimeType"])
	data, decodedContentType, decoded, err := decodeImageAssetString(uri, contentType)
	if kind == "response" || (!isDataURL && isHTTPURL) {
		data, decodedContentType, decoded, err = decodeImageAssetURL(ctx, uri, contentType)
	}
	if err != nil {
		return false, err
	}
	if !decoded {
		return false, nil
	}
	if kind == "request" && !opts.UpstreamSupportsURLAssets {
		node[inlineKeyForFileKey(fileKey)] = map[string]any{
			mimeKey: decodedContentType,
			"data":  base64.StdEncoding.EncodeToString(data),
		}
		delete(node, fileKey)
		return true, nil
	}
	if kind == "response" && normalizeGeminiImageResponseFormat(opts.ResponseFormat) == "b64_json" {
		node[inlineKeyForFileKey(fileKey)] = map[string]any{
			mimeKey: decodedContentType,
			"data":  base64.StdEncoding.EncodeToString(data),
		}
		delete(node, fileKey)
		return true, nil
	}
	if uploader == nil {
		return false, infraerrors.InternalServer("IMAGE_ASSET_STORAGE_NOT_CONFIGURED", "image asset storage is not configured")
	}
	url, _, err := uploader.UploadImageAsset(ctx, kind, data, decodedContentType)
	if err != nil {
		return false, err
	}
	fileData[mimeKey] = decodedContentType
	fileData[uriKey] = url
	return true, nil
}

func inlineKeyForFileKey(fileKey string) string {
	if fileKey == "file_data" {
		return "inline_data"
	}
	return "inlineData"
}
