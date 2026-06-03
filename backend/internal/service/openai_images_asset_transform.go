package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const openAIImagesJSONContentType = "application/json"

type OpenAIImagesAssetTransformResult struct {
	Body        []byte
	Parsed      *OpenAIImagesRequest
	Transformed bool
}

func (s *OpenAIGatewayService) TransformOpenAIImagesAssets(
	ctx context.Context,
	groupID *int64,
	body []byte,
	parsed *OpenAIImagesRequest,
) (*OpenAIImagesAssetTransformResult, error) {
	if !s.isImageAssetURLTransformEnabled(ctx, groupID) {
		return &OpenAIImagesAssetTransformResult{Body: body, Parsed: parsed}, nil
	}
	if s == nil || s.settingService == nil {
		return nil, infraerrors.InternalServer("IMAGE_ASSET_STORAGE_NOT_CONFIGURED", "image asset storage is not configured")
	}
	if err := s.settingService.EnsureImageAssetStorageReady(ctx); err != nil {
		return nil, err
	}
	next := parsed.Clone()
	next.AssetURLTransformEnabled = true
	if next.IsEdits() {
		if next.Multipart {
			rewrittenBody, rewrittenParsed, err := s.transformOpenAIImagesMultipartInput(ctx, next)
			if err != nil {
				return nil, err
			}
			return &OpenAIImagesAssetTransformResult{Body: rewrittenBody, Parsed: rewrittenParsed, Transformed: true}, nil
		}
		rewrittenBody, changed, err := transformOpenAIImagesJSONInputAssets(ctx, s.settingService, body)
		if err != nil {
			return nil, err
		}
		if changed {
			parsedAfter, err := parseOpenAIImagesJSONBytes(rewrittenBody, next.Endpoint, next.ContentType)
			if err != nil {
				return nil, err
			}
			parsedAfter.AssetURLTransformEnabled = true
			return &OpenAIImagesAssetTransformResult{Body: rewrittenBody, Parsed: parsedAfter, Transformed: true}, nil
		}
	}
	return &OpenAIImagesAssetTransformResult{Body: body, Parsed: next, Transformed: false}, nil
}

func (s *OpenAIGatewayService) isImageAssetURLTransformEnabled(ctx context.Context, groupID *int64) bool {
	if s == nil || groupID == nil || s.channelService == nil {
		return false
	}
	ch, err := s.channelService.GetChannelForGroup(ctx, *groupID)
	if err != nil {
		return false
	}
	return ch.IsImageAssetURLTransformEnabled(PlatformOpenAI)
}

func (r *OpenAIImagesRequest) Clone() *OpenAIImagesRequest {
	if r == nil {
		return nil
	}
	cp := *r
	if r.InputImageURLs != nil {
		cp.InputImageURLs = append([]string(nil), r.InputImageURLs...)
	}
	if r.Uploads != nil {
		cp.Uploads = append([]OpenAIImagesUpload(nil), r.Uploads...)
	}
	if r.MaskUpload != nil {
		mask := *r.MaskUpload
		cp.MaskUpload = &mask
	}
	if r.Body != nil {
		cp.Body = append([]byte(nil), r.Body...)
	}
	return &cp
}

func parseOpenAIImagesJSONBytes(body []byte, endpoint string, contentType string) (*OpenAIImagesRequest, error) {
	req := &OpenAIImagesRequest{
		Endpoint:    endpoint,
		ContentType: contentType,
		N:           1,
		Body:        body,
	}
	if err := parseOpenAIImagesJSONRequest(body, req); err != nil {
		return nil, err
	}
	applyOpenAIImagesDefaults(req)
	if err := validateOpenAIImagesModel(req.Model); err != nil {
		return nil, err
	}
	req.SizeTier = normalizeOpenAIImageSizeTier(req.Size)
	req.RequiredCapability = classifyOpenAIImagesCapability(req)
	return req, nil
}

func (s *OpenAIGatewayService) transformOpenAIImagesMultipartInput(ctx context.Context, parsed *OpenAIImagesRequest) ([]byte, *OpenAIImagesRequest, error) {
	if parsed == nil {
		return nil, nil, fmt.Errorf("missing images request")
	}
	payload := openAIImagesRequestToJSONMap(parsed)
	images := make([]map[string]string, 0, len(parsed.Uploads)+len(parsed.InputImageURLs))
	for _, imageURL := range parsed.InputImageURLs {
		if strings.TrimSpace(imageURL) != "" {
			images = append(images, map[string]string{"image_url": strings.TrimSpace(imageURL)})
		}
	}
	for _, upload := range parsed.Uploads {
		url, _, err := s.settingService.UploadImageAsset(ctx, "request", upload.Data, upload.ContentType)
		if err != nil {
			return nil, nil, err
		}
		images = append(images, map[string]string{"image_url": url})
	}
	if parsed.IsEdits() && len(images) == 0 {
		return nil, nil, infraerrors.BadRequest("INVALID_IMAGE_ASSET_REQUEST", "image file is required")
	}
	if len(images) > 0 {
		payload["images"] = images
	}
	if parsed.MaskImageURL != "" {
		payload["mask"] = map[string]string{"image_url": strings.TrimSpace(parsed.MaskImageURL)}
	}
	if parsed.MaskUpload != nil {
		url, _, err := s.settingService.UploadImageAsset(ctx, "request-mask", parsed.MaskUpload.Data, parsed.MaskUpload.ContentType)
		if err != nil {
			return nil, nil, err
		}
		payload["mask"] = map[string]string{"image_url": url}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal transformed images request: %w", err)
	}
	rewritten, err := parseOpenAIImagesJSONBytes(body, parsed.Endpoint, openAIImagesJSONContentType)
	if err != nil {
		return nil, nil, err
	}
	rewritten.AssetURLTransformEnabled = true
	return body, rewritten, nil
}

func openAIImagesRequestToJSONMap(parsed *OpenAIImagesRequest) map[string]any {
	payload := map[string]any{}
	if parsed == nil {
		return payload
	}
	if parsed.Model != "" {
		payload["model"] = parsed.Model
	}
	if parsed.Prompt != "" {
		payload["prompt"] = parsed.Prompt
	}
	if parsed.N > 0 {
		payload["n"] = parsed.N
	}
	if parsed.Size != "" {
		payload["size"] = parsed.Size
	}
	if parsed.ResponseFormat != "" {
		payload["response_format"] = parsed.ResponseFormat
	}
	if parsed.Quality != "" {
		payload["quality"] = parsed.Quality
	}
	if parsed.Background != "" {
		payload["background"] = parsed.Background
	}
	if parsed.OutputFormat != "" {
		payload["output_format"] = parsed.OutputFormat
	}
	if parsed.Moderation != "" {
		payload["moderation"] = parsed.Moderation
	}
	if parsed.InputFidelity != "" {
		payload["input_fidelity"] = parsed.InputFidelity
	}
	if parsed.Style != "" {
		payload["style"] = parsed.Style
	}
	if parsed.OutputCompression != nil {
		payload["output_compression"] = *parsed.OutputCompression
	}
	if parsed.PartialImages != nil {
		payload["partial_images"] = *parsed.PartialImages
	}
	if parsed.Stream {
		payload["stream"] = true
	}
	return payload
}

func transformOpenAIImagesJSONInputAssets(ctx context.Context, uploader imageAssetUploader, body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, false, err
	}
	changed, err := walkOpenAIImagesRequestAssets(ctx, uploader, decoded)
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

func walkOpenAIImagesRequestAssets(ctx context.Context, uploader imageAssetUploader, node any) (bool, error) {
	switch value := node.(type) {
	case map[string]any:
		changed := false
		if imageURL, ok := value["image_url"].(string); ok {
			if data, contentType, ok, err := decodeImageAssetString(imageURL, ""); err != nil {
				return false, err
			} else if ok {
				url, _, err := uploader.UploadImageAsset(ctx, "request", data, contentType)
				if err != nil {
					return false, err
				}
				value["image_url"] = url
				changed = true
			}
		}
		if raw, ok := value["image"].(string); ok {
			if data, contentType, ok, err := decodeImageAssetString(raw, ""); err != nil {
				return false, err
			} else if ok {
				url, _, err := uploader.UploadImageAsset(ctx, "request", data, contentType)
				if err != nil {
					return false, err
				}
				value["image_url"] = url
				delete(value, "image")
				changed = true
			}
		}
		if raw, ok := value["b64_json"].(string); ok {
			if data, contentType, ok, err := decodeImageAssetString(raw, firstNonEmptyString(value["mime_type"], value["content_type"])); err != nil {
				return false, err
			} else if ok {
				url, _, err := uploader.UploadImageAsset(ctx, "request", data, contentType)
				if err != nil {
					return false, err
				}
				value["image_url"] = url
				delete(value, "b64_json")
				changed = true
			}
		}
		for _, child := range value {
			childChanged, err := walkOpenAIImagesRequestAssets(ctx, uploader, child)
			if err != nil {
				return false, err
			}
			changed = changed || childChanged
		}
		return changed, nil
	case []any:
		changed := false
		for _, child := range value {
			childChanged, err := walkOpenAIImagesRequestAssets(ctx, uploader, child)
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

func (s *OpenAIGatewayService) transformOpenAIImagesResponseBody(ctx context.Context, parsed *OpenAIImagesRequest, body []byte) ([]byte, bool, error) {
	if parsed == nil || !parsed.AssetURLTransformEnabled || s == nil || s.settingService == nil || len(body) == 0 || !gjsonValidBytes(body) {
		return body, false, nil
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return body, false, nil
	}
	changed, err := walkOpenAIImagesResponseAssets(ctx, s.settingService, decoded, parsed)
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

func (s *OpenAIGatewayService) transformOpenAIImagesSSELine(ctx context.Context, parsed *OpenAIImagesRequest, line []byte) ([]byte, bool, error) {
	if parsed == nil || !parsed.AssetURLTransformEnabled || len(line) == 0 {
		return line, false, nil
	}
	raw := string(line)
	ending := ""
	switch {
	case strings.HasSuffix(raw, "\r\n"):
		ending = "\r\n"
		raw = strings.TrimSuffix(raw, "\r\n")
	case strings.HasSuffix(raw, "\n"):
		ending = "\n"
		raw = strings.TrimSuffix(raw, "\n")
	}
	data, ok := extractOpenAISSEDataLine(raw)
	if !ok || strings.TrimSpace(data) == "" || strings.TrimSpace(data) == "[DONE]" {
		return line, false, nil
	}
	transformed, changed, err := s.transformOpenAIImagesResponseBody(ctx, parsed, []byte(data))
	if err != nil || !changed {
		return line, changed, err
	}
	prefix := "data:"
	if strings.HasPrefix(raw, "data: ") {
		prefix = "data: "
	}
	return []byte(prefix + string(transformed) + ending), true, nil
}

func walkOpenAIImagesResponseAssets(ctx context.Context, uploader imageAssetUploader, node any, parsed *OpenAIImagesRequest) (bool, error) {
	switch value := node.(type) {
	case map[string]any:
		changed := false
		contentTypeHint := firstNonEmptyString(value["mime_type"], value["content_type"])
		if contentTypeHint == "" && parsed != nil {
			contentTypeHint = openAIImageOutputMIMEType(parsed.OutputFormat)
		}
		if raw, ok := value["b64_json"].(string); ok {
			data, contentType, decoded, err := decodeImageAssetString(raw, contentTypeHint)
			if err != nil {
				return false, err
			}
			if decoded {
				url, _, err := uploader.UploadImageAsset(ctx, "response", data, contentType)
				if err != nil {
					return false, err
				}
				value["url"] = url
				delete(value, "b64_json")
				changed = true
			}
		}
		if raw, ok := value["url"].(string); ok {
			data, contentType, decoded, err := decodeImageAssetString(raw, contentTypeHint)
			if err != nil {
				return false, err
			}
			if decoded {
				url, _, err := uploader.UploadImageAsset(ctx, "response", data, contentType)
				if err != nil {
					return false, err
				}
				value["url"] = url
				changed = true
			}
		}
		for _, child := range value {
			childChanged, err := walkOpenAIImagesResponseAssets(ctx, uploader, child, parsed)
			if err != nil {
				return false, err
			}
			changed = changed || childChanged
		}
		return changed, nil
	case []any:
		changed := false
		for _, child := range value {
			childChanged, err := walkOpenAIImagesResponseAssets(ctx, uploader, child, parsed)
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

func decodeImageAssetString(raw string, contentTypeHint string) ([]byte, string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", false, nil
	}
	contentType := strings.TrimSpace(contentTypeHint)
	if strings.HasPrefix(strings.ToLower(raw), "data:") {
		header, encoded, ok := strings.Cut(raw, ",")
		if !ok || strings.TrimSpace(encoded) == "" {
			return nil, "", false, infraerrors.BadRequest("INVALID_IMAGE_ASSET_DATA", "invalid image data URL")
		}
		if contentType == "" {
			meta := strings.TrimPrefix(header, "data:")
			if semi := strings.Index(meta, ";"); semi >= 0 {
				meta = meta[:semi]
			}
			contentType = strings.TrimSpace(meta)
		}
		raw = encoded
	}
	if contentType == "" {
		contentType = "image/png"
	}
	normalized := normalizeOpenAIImageBase64(raw)
	if normalized == "" {
		return nil, "", false, nil
	}
	data, err := base64.StdEncoding.DecodeString(normalized)
	if err != nil {
		return nil, "", false, infraerrors.BadRequest("INVALID_IMAGE_ASSET_DATA", "invalid base64 image data")
	}
	detected := http.DetectContentType(data)
	if strings.HasPrefix(detected, "image/") && (contentType == "" || contentType == "application/octet-stream" || contentType == "image/png") {
		contentType = detected
	}
	return data, contentType, true, nil
}

func gjsonValidBytes(body []byte) bool {
	return len(body) > 0 && json.Valid(body)
}
