package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type imageAssetStorageSettingRepo struct {
	values map[string]string
}

func newImageAssetStorageSettingRepo() *imageAssetStorageSettingRepo {
	return &imageAssetStorageSettingRepo{values: map[string]string{}}
}

func (r *imageAssetStorageSettingRepo) Get(_ context.Context, key string) (*Setting, error) {
	if value, ok := r.values[key]; ok {
		return &Setting{Key: key, Value: value}, nil
	}
	return nil, ErrSettingNotFound
}

func (r *imageAssetStorageSettingRepo) GetValue(ctx context.Context, key string) (string, error) {
	setting, err := r.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (r *imageAssetStorageSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *imageAssetStorageSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *imageAssetStorageSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *imageAssetStorageSettingRepo) GetAll(_ context.Context) (map[string]string, error) {
	out := make(map[string]string, len(r.values))
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *imageAssetStorageSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestSaveImageAssetStorageConfigPersistsSettingsRow(t *testing.T) {
	repo := newImageAssetStorageSettingRepo()
	svc := NewSettingService(repo, &config.Config{})

	updated, err := svc.SaveImageAssetStorageConfig(context.Background(), ImageAssetStorageConfig{
		Enabled:                true,
		Endpoint:               " http://localhost:9000 ",
		Region:                 "",
		Bucket:                 "sub2api-images",
		AccessKeyID:            "minioadmin",
		SecretAccessKey:        "minioadmin123",
		Prefix:                 "/dev-image-assets/",
		ForcePathStyle:         true,
		PresignedURLTTLSeconds: 3600,
	})
	require.NoError(t, err)
	require.True(t, updated.SecretAccessKeyConfigured)
	require.Empty(t, updated.SecretAccessKey)

	raw, ok := repo.values[SettingKeyImageAssetStorageConfig]
	require.True(t, ok, "image asset storage config should be written to settings")

	var persisted ImageAssetStorageConfig
	require.NoError(t, json.Unmarshal([]byte(raw), &persisted))
	require.True(t, persisted.Enabled)
	require.Equal(t, "http://localhost:9000", persisted.Endpoint)
	require.Equal(t, "auto", persisted.Region)
	require.Equal(t, "sub2api-images", persisted.Bucket)
	require.Equal(t, "minioadmin", persisted.AccessKeyID)
	require.Equal(t, "minioadmin123", persisted.SecretAccessKey)
	require.Equal(t, "dev-image-assets", persisted.Prefix)
	require.True(t, persisted.ForcePathStyle)

	loaded, err := svc.GetImageAssetStorageConfig(context.Background())
	require.NoError(t, err)
	require.True(t, loaded.SecretAccessKeyConfigured)
	require.Empty(t, loaded.SecretAccessKey)
}
