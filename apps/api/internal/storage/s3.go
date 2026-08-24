package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"golang.org/x/image/webp"
)

const (
	StorefrontContentType = "image/webp"
	MaxStorefrontBytes    = int64(5 * 1024 * 1024)
	presignLifetime       = 10 * time.Minute
)

var ErrNotConfigured = errors.New("S3-compatible object storage is not configured")

type Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicBaseURL   string
	ForcePathStyle  bool
}

type PresignedUpload struct {
	UploadURL string
	PublicURL string
	ObjectKey string
	ExpiresAt time.Time
}

type Store struct {
	client        *s3.Client
	presigner     *s3.PresignClient
	bucket        string
	publicBaseURL string
}

func New(ctx context.Context, config Config) (*Store, error) {
	values := []string{config.Endpoint, config.Bucket, config.AccessKeyID, config.SecretAccessKey, config.PublicBaseURL}
	configured := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			configured++
		}
	}
	if configured == 0 {
		return nil, nil
	}
	if configured != len(values) {
		return nil, ErrNotConfigured
	}
	endpoint, err := url.ParseRequestURI(config.Endpoint)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return nil, fmt.Errorf("invalid S3 endpoint")
	}
	publicBase, err := url.ParseRequestURI(config.PublicBaseURL)
	if err != nil || (publicBase.Scheme != "http" && publicBase.Scheme != "https") {
		return nil, fmt.Errorf("invalid S3 public base URL")
	}
	region := config.Region
	if region == "" {
		region = "us-east-1"
	}
	sdkConfig, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load S3 config: %w", err)
	}
	client := s3.NewFromConfig(sdkConfig, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(strings.TrimRight(config.Endpoint, "/"))
		options.UsePathStyle = config.ForcePathStyle
	})
	return &Store{
		client:        client,
		presigner:     s3.NewPresignClient(client),
		bucket:        config.Bucket,
		publicBaseURL: strings.TrimRight(config.PublicBaseURL, "/"),
	}, nil
}

func (s *Store) PresignStorefrontUpload(ctx context.Context, operatorID, kind string, sizeBytes int64) (PresignedUpload, error) {
	if s == nil {
		return PresignedUpload{}, ErrNotConfigured
	}
	if sizeBytes <= 0 || sizeBytes > MaxStorefrontBytes {
		return PresignedUpload{}, fmt.Errorf("invalid object size")
	}
	switch kind {
	case "logo", "hero", "gallery", "package":
	default:
		return PresignedUpload{}, fmt.Errorf("invalid asset kind")
	}
	if _, err := uuid.Parse(operatorID); err != nil {
		return PresignedUpload{}, fmt.Errorf("invalid operator ID")
	}
	key := path.Join("storefront", operatorID, kind, uuid.NewString()+".webp")
	request, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(StorefrontContentType),
		ContentLength: aws.Int64(sizeBytes),
	}, func(options *s3.PresignOptions) {
		options.Expires = presignLifetime
	})
	if err != nil {
		return PresignedUpload{}, fmt.Errorf("presign storefront upload: %w", err)
	}
	return PresignedUpload{
		UploadURL: request.URL,
		PublicURL: s.publicBaseURL + "/" + key,
		ObjectKey: key,
		ExpiresAt: time.Now().Add(presignLifetime),
	}, nil
}

func (s *Store) ConfirmStorefrontUpload(ctx context.Context, operatorID, objectKey string) (string, error) {
	if s == nil {
		return "", ErrNotConfigured
	}
	prefix := path.Join("storefront", operatorID) + "/"
	if _, err := uuid.Parse(operatorID); err != nil || !strings.HasPrefix(objectKey, prefix) || !strings.HasSuffix(objectKey, ".webp") || path.Clean(objectKey) != objectKey {
		return "", fmt.Errorf("invalid object key")
	}
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(objectKey)})
	if err != nil {
		return "", fmt.Errorf("inspect storefront upload: %w", err)
	}
	validMetadata := head.ContentLength != nil && *head.ContentLength > 0 && *head.ContentLength <= MaxStorefrontBytes && head.ContentType != nil && *head.ContentType == StorefrontContentType
	if !validMetadata {
		s.deleteInvalid(ctx, objectKey)
		return "", fmt.Errorf("uploaded object metadata is invalid")
	}
	object, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(objectKey)})
	if err != nil {
		return "", fmt.Errorf("read storefront upload signature: %w", err)
	}
	defer object.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(object.Body, MaxStorefrontBytes+1))
	if readErr != nil {
		s.deleteInvalid(ctx, objectKey)
		return "", fmt.Errorf("read storefront upload signature: %w", readErr)
	}
	if int64(len(data)) != *head.ContentLength || len(data) < 12 || !bytes.Equal(data[0:4], []byte("RIFF")) || !bytes.Equal(data[8:12], []byte("WEBP")) {
		s.deleteInvalid(ctx, objectKey)
		return "", fmt.Errorf("uploaded object is not a WebP image")
	}
	imageConfig, decodeErr := webp.DecodeConfig(bytes.NewReader(data))
	if decodeErr != nil || imageConfig.Width <= 0 || imageConfig.Height <= 0 || imageConfig.Width > 5000 || imageConfig.Height > 5000 || int64(imageConfig.Width)*int64(imageConfig.Height) > 10_000_000 {
		s.deleteInvalid(ctx, objectKey)
		return "", fmt.Errorf("uploaded WebP dimensions are invalid")
	}
	if _, decodeErr = webp.Decode(bytes.NewReader(data)); decodeErr != nil {
		s.deleteInvalid(ctx, objectKey)
		return "", fmt.Errorf("uploaded WebP payload is invalid")
	}
	return s.publicBaseURL + "/" + objectKey, nil
}

func (s *Store) deleteInvalid(ctx context.Context, objectKey string) {
	_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(objectKey)})
}
