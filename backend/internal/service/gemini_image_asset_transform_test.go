package service

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestTransformGeminiImageAssetRequest_RewritesSnakeInlineDataToFileData(t *testing.T) {
	uploader := &fakeImageAssetUploader{urls: []string{"https://cdn.example.com/input.png"}}
	body := []byte(`{
		"contents":[{
			"role":"user",
			"parts":[
				{"text":"edit this"},
				{"inline_data":{"mime_type":"image/png","data":"` + base64.StdEncoding.EncodeToString([]byte("input-bytes")) + `"}}
			]
		}]
	}`)

	rewritten, changed, err := transformGeminiImageAssetRequest(context.Background(), uploader, body)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(rewritten, "contents.0.parts.1.inline_data").Exists())
	require.Equal(t, "image/png", gjson.GetBytes(rewritten, "contents.0.parts.1.file_data.mime_type").String())
	require.Equal(t, "https://cdn.example.com/input.png", gjson.GetBytes(rewritten, "contents.0.parts.1.file_data.file_uri").String())
	require.Len(t, uploader.uploads, 1)
	require.Equal(t, "request", uploader.uploads[0].kind)
	require.Equal(t, "input-bytes", string(uploader.uploads[0].data))
}

func TestTransformGeminiImageAssetResponse_RewritesCamelInlineDataToFileData(t *testing.T) {
	uploader := &fakeImageAssetUploader{urls: []string{"https://cdn.example.com/output.png"}}
	body := []byte(`{
		"candidates":[{
			"content":{
				"parts":[
					{"inlineData":{"mimeType":"image/png","data":"` + base64.StdEncoding.EncodeToString([]byte("output-bytes")) + `"}}
				]
			}
		}]
	}`)

	rewritten, changed, err := transformGeminiImageAssetResponse(context.Background(), uploader, body)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(rewritten, "candidates.0.content.parts.0.inlineData").Exists())
	require.Equal(t, "image/png", gjson.GetBytes(rewritten, "candidates.0.content.parts.0.fileData.mimeType").String())
	require.Equal(t, "https://cdn.example.com/output.png", gjson.GetBytes(rewritten, "candidates.0.content.parts.0.fileData.fileUri").String())
	require.Len(t, uploader.uploads, 1)
	require.Equal(t, "response", uploader.uploads[0].kind)
	require.Equal(t, "output-bytes", string(uploader.uploads[0].data))
}

func TestTransformGeminiImageAssetRequest_RewritesFileDataDataURL(t *testing.T) {
	uploader := &fakeImageAssetUploader{urls: []string{"https://cdn.example.com/file-data.png"}}
	body := []byte(`{
		"contents":[{
			"parts":[
				{"fileData":{"mimeType":"image/png","fileUri":"data:image/png;base64,` + base64.StdEncoding.EncodeToString([]byte("file-data-bytes")) + `"}}
			]
		}]
	}`)

	rewritten, changed, err := transformGeminiImageAssetRequest(context.Background(), uploader, body)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "https://cdn.example.com/file-data.png", gjson.GetBytes(rewritten, "contents.0.parts.0.fileData.fileUri").String())
	require.Len(t, uploader.uploads, 1)
	require.Equal(t, "request", uploader.uploads[0].kind)
	require.Equal(t, "file-data-bytes", string(uploader.uploads[0].data))
}

func TestTransformGeminiImageAssetRequest_UnsupportedURLUpstreamConvertsFileDataURLToInlineData(t *testing.T) {
	imageBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
		0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer upstream.Close()

	body := []byte(`{
		"response_format":"url",
		"contents":[{"parts":[
			{"fileData":{"mimeType":"image/png","fileUri":"` + upstream.URL + `/input.png"}}
		]}]
	}`)

	rewritten, changed, err := transformGeminiImageAssetRequestWithOptions(context.Background(), nil, body, geminiImageAssetTransformOptions{
		UpstreamSupportsURLAssets:  false,
		StripResponseFormatRequest: true,
	})
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(rewritten, "response_format").Exists())
	require.False(t, gjson.GetBytes(rewritten, "contents.0.parts.0.fileData").Exists())
	require.Equal(t, "image/png", gjson.GetBytes(rewritten, "contents.0.parts.0.inlineData.mimeType").String())
	require.Equal(t, base64.StdEncoding.EncodeToString(imageBytes), gjson.GetBytes(rewritten, "contents.0.parts.0.inlineData.data").String())
}

func TestTransformGeminiImageAssetResponse_ExplicitB64ConvertsFileDataURLToInlineData(t *testing.T) {
	imageBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
		0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer upstream.Close()

	body := []byte(`{
		"candidates":[{"content":{"parts":[
			{"fileData":{"mimeType":"image/png","fileUri":"` + upstream.URL + `/output.png"}}
		]}}]
	}`)

	rewritten, changed, err := transformGeminiImageAssetResponseWithOptions(context.Background(), nil, body, geminiImageAssetTransformOptions{ResponseFormat: "b64_json"})
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(rewritten, "candidates.0.content.parts.0.fileData").Exists())
	require.Equal(t, "image/png", gjson.GetBytes(rewritten, "candidates.0.content.parts.0.inlineData.mimeType").String())
	require.Equal(t, base64.StdEncoding.EncodeToString(imageBytes), gjson.GetBytes(rewritten, "candidates.0.content.parts.0.inlineData.data").String())
}

func TestTransformGeminiImageAssetResponse_DownloadsFileDataURL(t *testing.T) {
	imageBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
		0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer upstream.Close()

	uploader := &fakeImageAssetUploader{urls: []string{"https://cdn.example.com/gemini-output.png"}}
	body := []byte(`{
		"candidates":[{"content":{"parts":[
			{"fileData":{"mimeType":"image/png","fileUri":"` + upstream.URL + `/output.png"}}
		]}}]
	}`)

	rewritten, changed, err := transformGeminiImageAssetResponse(context.Background(), uploader, body)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "https://cdn.example.com/gemini-output.png", gjson.GetBytes(rewritten, "candidates.0.content.parts.0.fileData.fileUri").String())
	require.Len(t, uploader.uploads, 1)
	require.Equal(t, "response", uploader.uploads[0].kind)
	require.Equal(t, imageBytes, uploader.uploads[0].data)
	require.Equal(t, "image/png", uploader.uploads[0].contentType)
}
