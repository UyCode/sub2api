package service

import (
	"context"
	"encoding/json"
	"strings"
)

type geminiImageAssetTransformOptions struct {
	Enabled bool
}

func transformGeminiImageAssetRequest(ctx context.Context, uploader imageAssetUploader, body []byte) ([]byte, bool, error) {
	return transformGeminiImageAssetJSON(ctx, uploader, body, "request")
}

func transformGeminiImageAssetResponse(ctx context.Context, uploader imageAssetUploader, body []byte) ([]byte, bool, error) {
	return transformGeminiImageAssetJSON(ctx, uploader, body, "response")
}

func transformGeminiImageAssetJSON(ctx context.Context, uploader imageAssetUploader, body []byte, kind string) ([]byte, bool, error) {
	if uploader == nil || len(body) == 0 || !gjsonValidBytes(body) {
		return body, false, nil
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return body, false, nil
	}
	changed, err := walkGeminiImageAssetJSON(ctx, uploader, decoded, kind)
	if err != nil {
		return nil, false, err
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

func walkGeminiImageAssetJSON(ctx context.Context, uploader imageAssetUploader, node any, kind string) (bool, error) {
	switch value := node.(type) {
	case map[string]any:
		changed := false
		if childChanged, err := rewriteGeminiInlineDataPart(ctx, uploader, value, kind, "inline_data", "file_data", "mime_type", "file_uri"); err != nil {
			return false, err
		} else if childChanged {
			changed = true
		}
		if childChanged, err := rewriteGeminiInlineDataPart(ctx, uploader, value, kind, "inlineData", "fileData", "mimeType", "fileUri"); err != nil {
			return false, err
		} else if childChanged {
			changed = true
		}
		if childChanged, err := rewriteGeminiFileDataURL(ctx, uploader, value, kind, "file_data", "mime_type", "file_uri"); err != nil {
			return false, err
		} else if childChanged {
			changed = true
		}
		if childChanged, err := rewriteGeminiFileDataURL(ctx, uploader, value, kind, "fileData", "mimeType", "fileUri"); err != nil {
			return false, err
		} else if childChanged {
			changed = true
		}
		for _, child := range value {
			childChanged, err := walkGeminiImageAssetJSON(ctx, uploader, child, kind)
			if err != nil {
				return false, err
			}
			changed = changed || childChanged
		}
		return changed, nil
	case []any:
		changed := false
		for _, child := range value {
			childChanged, err := walkGeminiImageAssetJSON(ctx, uploader, child, kind)
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

func rewriteGeminiInlineDataPart(ctx context.Context, uploader imageAssetUploader, part map[string]any, kind, inlineKey, fileKey, mimeKey, uriKey string) (bool, error) {
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

func rewriteGeminiFileDataURL(ctx context.Context, uploader imageAssetUploader, node map[string]any, kind, fileKey, mimeKey, uriKey string) (bool, error) {
	fileData, ok := node[fileKey].(map[string]any)
	if !ok {
		return false, nil
	}
	uri, ok := fileData[uriKey].(string)
	if !ok {
		return false, nil
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(uri)), "data:") {
		return false, nil
	}
	contentType := firstNonEmptyString(fileData[mimeKey], fileData["mime_type"], fileData["mimeType"])
	data, decodedContentType, decoded, err := decodeImageAssetString(uri, contentType)
	if err != nil {
		return false, err
	}
	if !decoded {
		return false, nil
	}
	url, _, err := uploader.UploadImageAsset(ctx, kind, data, decodedContentType)
	if err != nil {
		return false, err
	}
	fileData[mimeKey] = decodedContentType
	fileData[uriKey] = url
	return true, nil
}
