package service

import (
	"context"
	"encoding/base64"
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
