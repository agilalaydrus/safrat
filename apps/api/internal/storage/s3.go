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
	StorefrontContentType      = "image/webp"
	StorefrontAudioContentType = "audio/mpeg"
	MaxStorefrontImageBytes    = int64(5 * 1024 * 1024)
	MaxStorefrontAudioBytes    = int64(10 * 1024 * 1024)
	MaxStorefrontBytes         = MaxStorefrontAudioBytes
	presignLifetime            = 10 * time.Minute
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
	UploadURL   string
	ObjectKey   string
	ContentType string
	ExpiresAt   time.Time
}

type ConfirmedUpload struct {
	PublicURL string
	ObjectKey string
	SizeBytes int64
}

type storefrontAssetSpec struct {
	contentType string
	extension   string
	maxBytes    int64
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
		// aws-sdk-go-v2 defaults to computing/validating checksums on every
		// request since ~v1.30, adding an x-amz-checksum-mode header to
		// presigned GETs. A plain client (a browser <img> tag, a bare fetch)
		// never sends that header back, so MinIO — and any non-AWS
		// S3-compatible store that enforces SignedHeaders strictly — rejects
		// the request with "headers present in the request which were not
		// signed" even though the signature itself is fine. WhenRequired
		// restores the pre-v1.30 behaviour: only checksum when a caller asks.
		options.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		options.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
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
	spec, err := storefrontSpec(kind)
	if err != nil || sizeBytes <= 0 || sizeBytes > spec.maxBytes {
		return PresignedUpload{}, fmt.Errorf("invalid object size")
	}
	if _, err := uuid.Parse(operatorID); err != nil {
		return PresignedUpload{}, fmt.Errorf("invalid operator ID")
	}
	key := path.Join("storefront-pending", operatorID, kind, uuid.NewString()+spec.extension)
	request, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(spec.contentType),
		ContentLength: aws.Int64(sizeBytes),
	}, func(options *s3.PresignOptions) {
		options.Expires = presignLifetime
	})
	if err != nil {
		return PresignedUpload{}, fmt.Errorf("presign storefront upload: %w", err)
	}
	return PresignedUpload{
		UploadURL:   request.URL,
		ObjectKey:   key,
		ContentType: spec.contentType,
		ExpiresAt:   time.Now().Add(presignLifetime),
	}, nil
}

func (s *Store) ConfirmStorefrontUpload(ctx context.Context, operatorID, objectKey string) (ConfirmedUpload, error) {
	if s == nil {
		return ConfirmedUpload{}, ErrNotConfigured
	}
	prefix := path.Join("storefront-pending", operatorID) + "/"
	if _, err := uuid.Parse(operatorID); err != nil || !strings.HasPrefix(objectKey, prefix) || path.Clean(objectKey) != objectKey {
		return ConfirmedUpload{}, fmt.Errorf("invalid object key")
	}
	relativeKey := strings.TrimPrefix(objectKey, prefix)
	parts := strings.Split(relativeKey, "/")
	if len(parts) != 2 {
		return ConfirmedUpload{}, fmt.Errorf("invalid object key")
	}
	spec, err := storefrontSpec(parts[0])
	if err != nil || !strings.HasSuffix(parts[1], spec.extension) {
		return ConfirmedUpload{}, fmt.Errorf("invalid object key")
	}
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(objectKey)})
	if err != nil {
		return ConfirmedUpload{}, fmt.Errorf("inspect storefront upload: %w", err)
	}
	validMetadata := head.ContentLength != nil && *head.ContentLength > 0 && *head.ContentLength <= spec.maxBytes && head.ContentType != nil && *head.ContentType == spec.contentType
	if !validMetadata {
		s.deleteInvalid(ctx, objectKey)
		return ConfirmedUpload{}, fmt.Errorf("uploaded object metadata is invalid")
	}
	object, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(objectKey)})
	if err != nil {
		return ConfirmedUpload{}, fmt.Errorf("read storefront upload signature: %w", err)
	}
	defer object.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(object.Body, spec.maxBytes+1))
	if readErr != nil {
		s.deleteInvalid(ctx, objectKey)
		return ConfirmedUpload{}, fmt.Errorf("read storefront upload signature: %w", readErr)
	}
	if int64(len(data)) != *head.ContentLength || !validStorefrontPayload(spec, data) {
		s.deleteInvalid(ctx, objectKey)
		return ConfirmedUpload{}, fmt.Errorf("uploaded object signature is invalid")
	}
	if spec.contentType == StorefrontContentType {
		imageConfig, decodeErr := webp.DecodeConfig(bytes.NewReader(data))
		if decodeErr != nil || imageConfig.Width <= 0 || imageConfig.Height <= 0 || imageConfig.Width > 5000 || imageConfig.Height > 5000 || int64(imageConfig.Width)*int64(imageConfig.Height) > 10_000_000 {
			s.deleteInvalid(ctx, objectKey)
			return ConfirmedUpload{}, fmt.Errorf("uploaded WebP dimensions are invalid")
		}
		if _, decodeErr = webp.Decode(bytes.NewReader(data)); decodeErr != nil {
			s.deleteInvalid(ctx, objectKey)
			return ConfirmedUpload{}, fmt.Errorf("uploaded WebP payload is invalid")
		}
	}
	liveKey := path.Join("storefront", operatorID, strings.TrimPrefix(objectKey, prefix))
	copySource := url.PathEscape(s.bucket + "/" + objectKey)
	if _, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(liveKey), CopySource: aws.String(copySource),
	}); err != nil {
		return ConfirmedUpload{}, fmt.Errorf("promote storefront upload: %w", err)
	}
	// Keep the pending source until its one-day lifecycle expiry. Confirmation
	// is then safe to retry if registering the promoted object in PostgreSQL
	// fails after this copy succeeds.
	return ConfirmedUpload{PublicURL: s.publicBaseURL + "/" + liveKey, ObjectKey: liveKey, SizeBytes: *head.ContentLength}, nil
}

func (s *Store) DeleteStorefrontObject(ctx context.Context, operatorID, objectKey string) error {
	if s == nil {
		return ErrNotConfigured
	}
	prefix := path.Join("storefront", operatorID) + "/"
	if _, err := uuid.Parse(operatorID); err != nil || !strings.HasPrefix(objectKey, prefix) || path.Clean(objectKey) != objectKey {
		return fmt.Errorf("invalid storefront object key")
	}
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(objectKey)}); err != nil {
		return fmt.Errorf("delete storefront object: %w", err)
	}
	return nil
}

func storefrontSpec(kind string) (storefrontAssetSpec, error) {
	switch kind {
	case "logo", "hero", "gallery", "package", "article", "about":
		return storefrontAssetSpec{contentType: StorefrontContentType, extension: ".webp", maxBytes: MaxStorefrontImageBytes}, nil
	case "background-music":
		return storefrontAssetSpec{contentType: StorefrontAudioContentType, extension: ".mp3", maxBytes: MaxStorefrontAudioBytes}, nil
	default:
		return storefrontAssetSpec{}, fmt.Errorf("invalid asset kind")
	}
}

func validStorefrontPayload(spec storefrontAssetSpec, data []byte) bool {
	if spec.contentType == StorefrontContentType {
		return len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP"))
	}
	if spec.contentType == StorefrontAudioContentType {
		return validMP3Payload(data)
	}
	return false
}

func validMP3Payload(data []byte) bool {
	offset := 0
	if len(data) >= 3 && bytes.Equal(data[0:3], []byte("ID3")) {
		if len(data) < 10 || data[6]&0x80 != 0 || data[7]&0x80 != 0 || data[8]&0x80 != 0 || data[9]&0x80 != 0 {
			return false
		}
		tagSize := int(data[6])<<21 | int(data[7])<<14 | int(data[8])<<7 | int(data[9])
		offset = 10 + tagSize
		if data[5]&0x10 != 0 {
			offset += 10
		}
	}
	if len(data) < offset+4 {
		return false
	}
	first, second, third := data[offset], data[offset+1], data[offset+2]
	version := (second >> 3) & 0x03
	layer := (second >> 1) & 0x03
	bitrate := (third >> 4) & 0x0f
	sampleRate := (third >> 2) & 0x03
	return first == 0xff && second&0xe0 == 0xe0 && version != 0x01 && layer != 0 && bitrate != 0 && bitrate != 0x0f && sampleRate != 0x03
}

func (s *Store) deleteInvalid(ctx context.Context, objectKey string) {
	_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(objectKey)})
}

// Handover proof is stored privately. A storefront asset is meant to be seen by
// everyone; a delivery receipt shows a person's name, their signature and often
// their doorway. It never reaches the public prefix and is never given a public
// URL — reads go through a short-lived signed link instead.
const (
	HandoverContentType   = "image/jpeg"
	MaxHandoverPhotoBytes = 5 << 20
	// Short enough that a link pasted somewhere by accident stops working
	// quickly, long enough to load a photo on a slow connection.
	handoverViewLifetime = 5 * time.Minute
)

// PresignHandoverUpload returns a one-shot PUT for a delivery photo.
//
// The key is derived from the operator and order rather than supplied, so a
// caller cannot write into another tenant's prefix or overwrite an existing
// proof by naming it.
func (s *Store) PresignHandoverUpload(ctx context.Context, operatorID, orderID string, sizeBytes int64) (PresignedUpload, error) {
	if s == nil {
		return PresignedUpload{}, ErrNotConfigured
	}
	if sizeBytes <= 0 || sizeBytes > MaxHandoverPhotoBytes {
		return PresignedUpload{}, fmt.Errorf("invalid object size")
	}
	if _, err := uuid.Parse(operatorID); err != nil {
		return PresignedUpload{}, fmt.Errorf("invalid operator ID")
	}
	if _, err := uuid.Parse(orderID); err != nil {
		return PresignedUpload{}, fmt.Errorf("invalid order ID")
	}
	key := path.Join("handover", operatorID, orderID, uuid.NewString()+".jpg")
	request, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(HandoverContentType),
		ContentLength: aws.Int64(sizeBytes),
	}, func(options *s3.PresignOptions) {
		options.Expires = presignLifetime
	})
	if err != nil {
		return PresignedUpload{}, fmt.Errorf("presign handover upload: %w", err)
	}
	return PresignedUpload{
		UploadURL: request.URL, ObjectKey: key,
		ContentType: HandoverContentType, ExpiresAt: time.Now().Add(presignLifetime),
	}, nil
}

// ConfirmHandoverUpload checks that something real landed before the key is
// recorded as evidence.
//
// Without this a caller could store any key it liked and the record would claim
// a photo exists where none does — which is worse than no photo, because the
// row would say the handover was documented.
func (s *Store) ConfirmHandoverUpload(ctx context.Context, operatorID, orderID, objectKey string) error {
	if s == nil {
		return ErrNotConfigured
	}
	if err := ValidHandoverKey(operatorID, orderID, objectKey); err != nil {
		return err
	}
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("handover object not found")
	}
	if head.ContentLength == nil || *head.ContentLength <= 0 || *head.ContentLength > MaxHandoverPhotoBytes {
		s.deleteInvalid(ctx, objectKey)
		return fmt.Errorf("handover object has an unusable size")
	}
	return nil
}

// PresignHandoverView returns a short-lived read link for a stored proof.
func (s *Store) PresignHandoverView(ctx context.Context, operatorID, orderID, objectKey string) (string, error) {
	if s == nil {
		return "", ErrNotConfigured
	}
	if err := ValidHandoverKey(operatorID, orderID, objectKey); err != nil {
		return "", err
	}
	request, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(objectKey),
	}, func(options *s3.PresignOptions) {
		options.Expires = handoverViewLifetime
	})
	if err != nil {
		return "", fmt.Errorf("presign handover view: %w", err)
	}
	return request.URL, nil
}

// ValidHandoverKey is the tenant boundary for these objects, expressed once.
//
// The key must sit under this operator and this order. Anything else — a
// traversal, another tenant's prefix, a key for a different order — is refused
// before it reaches S3, so a stored key can never point somewhere it should
// not.
func ValidHandoverKey(operatorID, orderID, objectKey string) error {
	if _, err := uuid.Parse(operatorID); err != nil {
		return fmt.Errorf("invalid operator ID")
	}
	if _, err := uuid.Parse(orderID); err != nil {
		return fmt.Errorf("invalid order ID")
	}
	prefix := path.Join("handover", operatorID, orderID) + "/"
	if !strings.HasPrefix(objectKey, prefix) || path.Clean(objectKey) != objectKey {
		return fmt.Errorf("handover key is outside this order")
	}
	return nil
}

// A moment photo is stored privately, same reasoning as a handover proof: it
// is shown only to the pilgrim's own family (via a presigned link resolved
// per request, see PresignMomentView), never given a public URL. Video is
// deliberately out of scope here — photos only, for now; see the task file
// for why.
const (
	MomentContentType   = "image/jpeg"
	MaxMomentPhotoBytes = 8 << 20
	// Longer than a handover proof's five minutes: a family member opens this
	// from a chat link or a saved bookmark, not the instant it's generated.
	momentViewLifetime = 15 * time.Minute
)

// PresignMomentUpload returns a one-shot PUT for a moment photo. The key is
// derived from the operator rather than supplied, so a caller cannot write
// into another tenant's prefix.
func (s *Store) PresignMomentUpload(ctx context.Context, operatorID string, sizeBytes int64) (PresignedUpload, error) {
	if s == nil {
		return PresignedUpload{}, ErrNotConfigured
	}
	if sizeBytes <= 0 || sizeBytes > MaxMomentPhotoBytes {
		return PresignedUpload{}, fmt.Errorf("invalid object size")
	}
	if _, err := uuid.Parse(operatorID); err != nil {
		return PresignedUpload{}, fmt.Errorf("invalid operator ID")
	}
	key := path.Join("moments", operatorID, uuid.NewString()+".jpg")
	request, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(MomentContentType),
		ContentLength: aws.Int64(sizeBytes),
	}, func(options *s3.PresignOptions) {
		options.Expires = presignLifetime
	})
	if err != nil {
		return PresignedUpload{}, fmt.Errorf("presign moment upload: %w", err)
	}
	return PresignedUpload{
		UploadURL: request.URL, ObjectKey: key,
		ContentType: MomentContentType, ExpiresAt: time.Now().Add(presignLifetime),
	}, nil
}

// ConfirmMomentUpload checks that something real landed before the key is
// recorded — a stored key for an object that does not exist would read as a
// moment that was never actually captured.
func (s *Store) ConfirmMomentUpload(ctx context.Context, operatorID, objectKey string) error {
	if s == nil {
		return ErrNotConfigured
	}
	if err := ValidMomentKey(operatorID, objectKey); err != nil {
		return err
	}
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("moment object not found")
	}
	if head.ContentLength == nil || *head.ContentLength <= 0 || *head.ContentLength > MaxMomentPhotoBytes {
		s.deleteInvalid(ctx, objectKey)
		return fmt.Errorf("moment object has an unusable size")
	}
	return nil
}

// PresignMomentView returns a short-lived read link for a stored moment photo.
func (s *Store) PresignMomentView(ctx context.Context, operatorID, objectKey string) (string, error) {
	if s == nil {
		return "", ErrNotConfigured
	}
	if err := ValidMomentKey(operatorID, objectKey); err != nil {
		return "", err
	}
	request, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(objectKey),
	}, func(options *s3.PresignOptions) {
		options.Expires = momentViewLifetime
	})
	if err != nil {
		return "", fmt.Errorf("presign moment view: %w", err)
	}
	return request.URL, nil
}

// ValidMomentKey is the tenant boundary for these objects, expressed once —
// a key must sit under this operator's own prefix.
func ValidMomentKey(operatorID, objectKey string) error {
	if _, err := uuid.Parse(operatorID); err != nil {
		return fmt.Errorf("invalid operator ID")
	}
	prefix := path.Join("moments", operatorID) + "/"
	if !strings.HasPrefix(objectKey, prefix) || path.Clean(objectKey) != objectKey {
		return fmt.Errorf("moment key is outside this operator")
	}
	return nil
}

// DeleteMomentObject removes a moment photo — called when the database row
// is deleted, so a removed moment does not leave a still-reachable private
// photo sitting in the bucket forever.
func (s *Store) DeleteMomentObject(ctx context.Context, operatorID, objectKey string) error {
	if s == nil {
		return ErrNotConfigured
	}
	if err := ValidMomentKey(operatorID, objectKey); err != nil {
		return err
	}
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(objectKey)}); err != nil {
		return fmt.Errorf("delete moment object: %w", err)
	}
	return nil
}

// exportViewLifetime is longer than a handover proof's five minutes: an export
// is something a person downloads once, deliberately, often onto a slow
// connection, not a link opened the instant it is generated.
const exportViewLifetime = 15 * time.Minute

// PutDataExport writes a finished export directly, since the worker that
// builds it — not a browser — is the one producing the bytes. Every other
// write in this file is a presigned upload a client performs on its own; this
// is the one place the server itself is the uploader.
func (s *Store) PutDataExport(ctx context.Context, operatorID, exportID string, data []byte) (string, error) {
	if s == nil {
		return "", ErrNotConfigured
	}
	if _, err := uuid.Parse(operatorID); err != nil {
		return "", fmt.Errorf("invalid operator ID")
	}
	if _, err := uuid.Parse(exportID); err != nil {
		return "", fmt.Errorf("invalid export ID")
	}
	key := path.Join("exports", operatorID, exportID+".zip")
	if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
		Body: bytes.NewReader(data), ContentType: aws.String("application/zip"),
	}); err != nil {
		return "", fmt.Errorf("put data export: %w", err)
	}
	return key, nil
}

// PresignDataExportView is a time-limited link to a finished export, checked
// against the same tenant boundary as every other object key here — a key
// that does not sit under this operator's own export prefix is refused before
// it reaches S3.
func (s *Store) PresignDataExportView(ctx context.Context, operatorID, objectKey string) (string, error) {
	if s == nil {
		return "", ErrNotConfigured
	}
	if _, err := uuid.Parse(operatorID); err != nil {
		return "", fmt.Errorf("invalid operator ID")
	}
	prefix := path.Join("exports", operatorID) + "/"
	if !strings.HasPrefix(objectKey, prefix) || path.Clean(objectKey) != objectKey {
		return "", fmt.Errorf("export key is outside this operator")
	}
	request, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(objectKey),
	}, func(options *s3.PresignOptions) {
		options.Expires = exportViewLifetime
	})
	if err != nil {
		return "", fmt.Errorf("presign data export view: %w", err)
	}
	return request.URL, nil
}

// DeleteDataExport removes a finished export's file once its download link
// has expired. Deleting an object that is already gone is not an error — the
// sweep that calls this may run twice on the same row before the database
// catches up, and the outcome either way is "the file does not exist".
func (s *Store) DeleteDataExport(ctx context.Context, objectKey string) error {
	if s == nil {
		return ErrNotConfigured
	}
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(objectKey),
	}); err != nil {
		return fmt.Errorf("delete data export: %w", err)
	}
	return nil
}
