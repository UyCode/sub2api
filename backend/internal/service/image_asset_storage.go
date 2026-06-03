package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const defaultImageAssetPresignedURLTTLSeconds = 3600

type ImageAssetStorageConfig struct {
	Enabled                   bool   `json:"enabled"`
	Endpoint                  string `json:"endpoint"`
	Region                    string `json:"region"`
	Bucket                    string `json:"bucket"`
	AccessKeyID               string `json:"access_key_id"`
	SecretAccessKey           string `json:"secret_access_key,omitempty"` //nolint:revive // AWS-compatible field name
	SecretAccessKeyConfigured bool   `json:"secret_access_key_configured"`
	Prefix                    string `json:"prefix"`
	PublicBaseURL             string `json:"public_base_url"`
	ForcePathStyle            bool   `json:"force_path_style"`
	PresignedURLTTLSeconds    int    `json:"presigned_url_ttl_seconds"`
}

func (c ImageAssetStorageConfig) sanitized() ImageAssetStorageConfig {
	c.SecretAccessKeyConfigured = strings.TrimSpace(c.SecretAccessKey) != ""
	c.SecretAccessKey = ""
	return c
}

func (c ImageAssetStorageConfig) isUsable() bool {
	if !c.Enabled {
		return false
	}
	if strings.TrimSpace(c.Bucket) == "" || strings.TrimSpace(c.AccessKeyID) == "" || strings.TrimSpace(c.SecretAccessKey) == "" {
		return false
	}
	if strings.TrimSpace(c.PublicBaseURL) == "" && c.PresignedURLTTLSeconds <= 0 {
		return false
	}
	return true
}

func normalizeImageAssetStorageConfig(c *ImageAssetStorageConfig) {
	if c == nil {
		return
	}
	c.Endpoint = strings.TrimSpace(c.Endpoint)
	c.Region = strings.TrimSpace(c.Region)
	c.Bucket = strings.TrimSpace(c.Bucket)
	c.AccessKeyID = strings.TrimSpace(c.AccessKeyID)
	c.SecretAccessKey = strings.TrimSpace(c.SecretAccessKey)
	c.Prefix = strings.Trim(strings.TrimSpace(c.Prefix), "/")
	c.PublicBaseURL = strings.TrimRight(strings.TrimSpace(c.PublicBaseURL), "/")
	if c.Region == "" {
		c.Region = "auto"
	}
	if c.Enabled && c.PresignedURLTTLSeconds <= 0 && c.PublicBaseURL == "" {
		c.PresignedURLTTLSeconds = defaultImageAssetPresignedURLTTLSeconds
	}
}

func validateImageAssetStorageConfig(c *ImageAssetStorageConfig) error {
	if c == nil || !c.Enabled {
		return nil
	}
	if c.Bucket == "" {
		return fmt.Errorf("bucket is required")
	}
	if c.AccessKeyID == "" {
		return fmt.Errorf("access_key_id is required")
	}
	if c.SecretAccessKey == "" {
		return fmt.Errorf("secret_access_key is required")
	}
	if c.PublicBaseURL == "" && c.PresignedURLTTLSeconds <= 0 {
		return fmt.Errorf("public_base_url or presigned_url_ttl_seconds is required")
	}
	if c.PublicBaseURL != "" {
		u, err := url.Parse(c.PublicBaseURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("public_base_url must be a valid URL")
		}
	}
	if c.PresignedURLTTLSeconds < 0 {
		return fmt.Errorf("presigned_url_ttl_seconds must be greater than or equal to 0")
	}
	return nil
}

func (s *SettingService) GetImageAssetStorageConfig(ctx context.Context) (*ImageAssetStorageConfig, error) {
	cfg, err := s.getImageAssetStorageConfigRaw(ctx)
	if err != nil {
		if errorsIsSettingNotFound(err) {
			return &ImageAssetStorageConfig{}, nil
		}
		return nil, err
	}
	return imageAssetConfigPtr(cfg.sanitized()), nil
}

func (s *SettingService) getImageAssetStorageConfigRaw(ctx context.Context) (ImageAssetStorageConfig, error) {
	var cfg ImageAssetStorageConfig
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyImageAssetStorageConfig)
	if err != nil || strings.TrimSpace(raw) == "" {
		return cfg, err
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, fmt.Errorf("image asset storage config data is corrupted: %w", err)
	}
	normalizeImageAssetStorageConfig(&cfg)
	return cfg, nil
}

func (s *SettingService) SaveImageAssetStorageConfig(ctx context.Context, cfg ImageAssetStorageConfig) (*ImageAssetStorageConfig, error) {
	normalizeImageAssetStorageConfig(&cfg)
	if cfg.SecretAccessKey == "" {
		if existing, err := s.getImageAssetStorageConfigRaw(ctx); err == nil {
			cfg.SecretAccessKey = existing.SecretAccessKey
		}
	}
	if err := validateImageAssetStorageConfig(&cfg); err != nil {
		return nil, infraerrors.BadRequest("INVALID_IMAGE_ASSET_STORAGE_CONFIG", err.Error())
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal image asset storage config: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyImageAssetStorageConfig, string(data)); err != nil {
		return nil, fmt.Errorf("save image asset storage config: %w", err)
	}
	out := cfg.sanitized()
	return &out, nil
}

func (s *SettingService) UploadImageAsset(ctx context.Context, kind string, data []byte, contentType string) (string, string, error) {
	cfg, err := s.getImageAssetStorageConfigRaw(ctx)
	if err != nil {
		if errorsIsSettingNotFound(err) {
			return "", "", infraerrors.InternalServer("IMAGE_ASSET_STORAGE_NOT_CONFIGURED", "image asset storage is not configured")
		}
		return "", "", err
	}
	return uploadImageAssetWithConfig(ctx, cfg, kind, data, contentType)
}

func (s *SettingService) EnsureImageAssetStorageReady(ctx context.Context) error {
	cfg, err := s.getImageAssetStorageConfigRaw(ctx)
	if err != nil {
		if errorsIsSettingNotFound(err) {
			return infraerrors.InternalServer("IMAGE_ASSET_STORAGE_NOT_CONFIGURED", "image asset storage is not configured")
		}
		return err
	}
	if !cfg.isUsable() {
		return infraerrors.InternalServer("IMAGE_ASSET_STORAGE_NOT_CONFIGURED", "image asset storage is not configured")
	}
	return nil
}

func (s *SettingService) TestImageAssetStorageConfig(ctx context.Context, cfg ImageAssetStorageConfig) (string, error) {
	normalizeImageAssetStorageConfig(&cfg)
	cfg.Enabled = true
	if cfg.SecretAccessKey == "" {
		if existing, err := s.getImageAssetStorageConfigRaw(ctx); err == nil {
			cfg.SecretAccessKey = existing.SecretAccessKey
		}
	}
	if err := validateImageAssetStorageConfig(&cfg); err != nil {
		return "", infraerrors.BadRequest("INVALID_IMAGE_ASSET_STORAGE_CONFIG", err.Error())
	}
	url, _, err := uploadImageAssetWithConfig(ctx, cfg, "test", []byte("sub2api image asset storage test"), "text/plain")
	return url, err
}

func uploadImageAssetWithConfig(ctx context.Context, cfg ImageAssetStorageConfig, kind string, data []byte, contentType string) (string, string, error) {
	normalizeImageAssetStorageConfig(&cfg)
	if !cfg.isUsable() {
		return "", "", infraerrors.InternalServer("IMAGE_ASSET_STORAGE_NOT_CONFIGURED", "image asset storage is not configured")
	}
	if len(data) == 0 {
		return "", "", fmt.Errorf("image asset data is empty")
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	client, err := newImageAssetS3Client(ctx, cfg)
	if err != nil {
		return "", "", err
	}
	key := buildImageAssetKey(cfg, kind, data, contentType)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", "", fmt.Errorf("S3 PutObject: %w", err)
	}
	if cfg.PublicBaseURL != "" {
		return joinImageAssetPublicURL(cfg.PublicBaseURL, key), key, nil
	}
	presigned, err := s3.NewPresignClient(client).PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(time.Duration(cfg.PresignedURLTTLSeconds)*time.Second))
	if err != nil {
		return "", "", fmt.Errorf("presign image asset url: %w", err)
	}
	return presigned.URL, key, nil
}

func newImageAssetS3Client(ctx context.Context, cfg ImageAssetStorageConfig) (*s3.Client, error) {
	region := cfg.Region
	if region == "" {
		region = "auto"
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		if cfg.ForcePathStyle {
			o.UsePathStyle = true
		}
		o.APIOptions = append(o.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	}), nil
}

func buildImageAssetKey(cfg ImageAssetStorageConfig, kind string, data []byte, contentType string) string {
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:8]) + "-" + uuid.NewString() + imageAssetExtension(contentType)
	kind = strings.Trim(strings.ToLower(strings.TrimSpace(kind)), "/")
	if kind == "" {
		kind = "image"
	}
	parts := []string{"images", kind, time.Now().UTC().Format("2006/01/02"), name}
	if cfg.Prefix != "" {
		parts = append([]string{cfg.Prefix}, parts...)
	}
	return path.Join(parts...)
}

func imageAssetExtension(contentType string) string {
	contentType = strings.TrimSpace(strings.ToLower(contentType))
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mediaType
	}
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	case "text/plain":
		return ".txt"
	default:
		return ".bin"
	}
}

func joinImageAssetPublicURL(base string, key string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	segments := strings.Split(strings.Trim(key, "/"), "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	if len(segments) == 0 {
		return base
	}
	return base + "/" + strings.Join(segments, "/")
}

func errorsIsSettingNotFound(err error) bool {
	return errors.Is(err, ErrSettingNotFound)
}

func imageAssetConfigPtr(v ImageAssetStorageConfig) *ImageAssetStorageConfig {
	return &v
}

type imageAssetUploader interface {
	UploadImageAsset(ctx context.Context, kind string, data []byte, contentType string) (string, string, error)
}
