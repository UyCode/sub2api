package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type fakeImageAssetUploader struct {
	urls      []string
	urlByData map[string]string
	uploads   []fakeImageAssetUpload
}

type fakeImageAssetUpload struct {
	kind        string
	contentType string
	data        []byte
}

func (f *fakeImageAssetUploader) UploadImageAsset(_ context.Context, kind string, data []byte, contentType string) (string, string, error) {
	f.uploads = append(f.uploads, fakeImageAssetUpload{
		kind:        kind,
		contentType: contentType,
		data:        append([]byte(nil), data...),
	})
	if len(f.urls) == 0 {
		if f.urlByData != nil {
			if url := f.urlByData[string(data)]; url != "" {
				return url, "key.png", nil
			}
		}
		return "https://cdn.example.com/fallback.png", "fallback.png", nil
	}
	url := f.urls[0]
	f.urls = f.urls[1:]
	return url, "key.png", nil
}

func TestTransformOpenAIImagesJSONInputAssets_RewritesDataURLAndB64JSON(t *testing.T) {
	uploader := &fakeImageAssetUploader{urlByData: map[string]string{
		"source-bytes": "https://cdn.example.com/source.png",
		"mask-bytes":   "https://cdn.example.com/mask.png",
	}}
	raw := base64.StdEncoding.EncodeToString([]byte("mask-bytes"))
	body := []byte(`{
		"model":"gpt-image-2",
		"prompt":"edit",
		"images":[{"image_url":"data:image/png;base64,c291cmNlLWJ5dGVz"}],
		"mask":{"b64_json":"` + raw + `","content_type":"image/png"}
	}`)

	rewritten, changed, err := transformOpenAIImagesJSONInputAssets(context.Background(), uploader, body)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "https://cdn.example.com/source.png", gjson.GetBytes(rewritten, "images.0.image_url").String())
	require.Equal(t, "https://cdn.example.com/mask.png", gjson.GetBytes(rewritten, "mask.image_url").String())
	require.False(t, gjson.GetBytes(rewritten, "mask.b64_json").Exists())
	require.Len(t, uploader.uploads, 2)
	require.Equal(t, "source-bytes", string(uploader.uploads[0].data))
	require.Equal(t, "mask-bytes", string(uploader.uploads[1].data))
}

func TestTransformOpenAIImagesResponseBody_RewritesB64JSONToURL(t *testing.T) {
	uploader := &fakeImageAssetUploader{urls: []string{"https://cdn.example.com/out.png"}}
	parsed := &OpenAIImagesRequest{AssetURLTransformEnabled: true, OutputFormat: "png"}
	body := []byte(`{"created":1,"data":[{"b64_json":"b3V0LWJ5dGVz"}]}`)

	rewritten, changed, err := (&openAIImagesResponseTransformHarness{uploader: uploader}).transform(context.Background(), parsed, body)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "https://cdn.example.com/out.png", gjson.GetBytes(rewritten, "data.0.url").String())
	require.False(t, gjson.GetBytes(rewritten, "data.0.b64_json").Exists())
	require.Len(t, uploader.uploads, 1)
	require.Equal(t, "response", uploader.uploads[0].kind)
}

type openAIImagesResponseTransformHarness struct {
	uploader imageAssetUploader
}

func (h *openAIImagesResponseTransformHarness) transform(ctx context.Context, parsed *OpenAIImagesRequest, body []byte) ([]byte, bool, error) {
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, false, err
	}
	changed, err := walkOpenAIImagesResponseAssets(ctx, h.uploader, decoded, parsed)
	if err != nil || !changed {
		return body, changed, err
	}
	out, err := json.Marshal(decoded)
	return out, err == nil, err
}

func TestDecodeImageAssetStringRejectsMalformedDataURL(t *testing.T) {
	_, _, _, err := decodeImageAssetString("data:image/png;base64", "")
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
}
