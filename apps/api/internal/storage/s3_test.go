package storage

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestNewAllowsStorageToBeDisabled(t *testing.T) {
	store, err := New(context.Background(), Config{})
	if err != nil || store != nil {
		t.Fatalf("New(empty) = (%v, %v), want (nil, nil)", store, err)
	}
}

func TestS3CompatibleUploadIntegration(t *testing.T) {
	endpoint := os.Getenv("S3_INTEGRATION_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_INTEGRATION_ENDPOINT is not set")
	}
	imageBytes, err := os.ReadFile("../../../web/public/images/tenant-umrah-hero.webp")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	store, err := New(context.Background(), Config{
		Endpoint: endpoint, Region: "us-east-1", Bucket: "safrat-uploads",
		AccessKeyID:     integrationValue("S3_INTEGRATION_ACCESS_KEY_ID", "safrat-local"),
		SecretAccessKey: integrationValue("S3_INTEGRATION_SECRET_ACCESS_KEY", "safrat-local-secret"),
		PublicBaseURL:   endpoint + "/safrat-uploads", ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	operatorID := "00000000-0000-4000-8000-000000000001"
	upload, err := store.PresignStorefrontUpload(context.Background(), operatorID, "hero", int64(len(imageBytes)))
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	t.Cleanup(func() {
		_, _ = store.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(upload.ObjectKey)})
		liveKey := strings.Replace(upload.ObjectKey, "storefront-pending/", "storefront/", 1)
		_, _ = store.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(liveKey)})
	})
	preflight, err := http.NewRequest(http.MethodOptions, upload.UploadURL, nil)
	if err != nil {
		t.Fatalf("new OPTIONS request: %v", err)
	}
	integrationOrigin := integrationValue("S3_INTEGRATION_ORIGIN", "http://localhost:3131")
	preflight.Header.Set("Origin", integrationOrigin)
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPut)
	preflight.Header.Set("Access-Control-Request-Headers", "content-type")
	preflightResponse, err := http.DefaultClient.Do(preflight)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	preflightResponse.Body.Close()
	if preflightResponse.StatusCode < 200 || preflightResponse.StatusCode >= 300 {
		t.Fatalf("OPTIONS status = %d", preflightResponse.StatusCode)
	}
	if allowedOrigin := preflightResponse.Header.Get("Access-Control-Allow-Origin"); allowedOrigin != integrationOrigin && allowedOrigin != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q", allowedOrigin)
	}
	request, err := http.NewRequest(http.MethodPut, upload.UploadURL, bytes.NewReader(imageBytes))
	if err != nil {
		t.Fatalf("new PUT request: %v", err)
	}
	request.Header.Set("Content-Type", StorefrontContentType)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("PUT status = %d", response.StatusCode)
	}
	confirmed, err := store.ConfirmStorefrontUpload(context.Background(), operatorID, upload.ObjectKey)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	wantURL := endpoint + "/safrat-uploads/" + strings.Replace(upload.ObjectKey, "storefront-pending/", "storefront/", 1)
	if confirmed.PublicURL != wantURL {
		t.Fatalf("confirmed URL = %q, want %q", confirmed.PublicURL, wantURL)
	}
	if confirmed.SizeBytes != int64(len(imageBytes)) {
		t.Fatalf("confirmed size = %d, want %d", confirmed.SizeBytes, len(imageBytes))
	}
	publicResponse, err := http.Get(confirmed.PublicURL)
	if err != nil {
		t.Fatalf("public GET: %v", err)
	}
	publicResponse.Body.Close()
	if publicResponse.StatusCode != http.StatusOK {
		t.Fatalf("public GET status = %d", publicResponse.StatusCode)
	}
	if contentType := publicResponse.Header.Get("Content-Type"); contentType != StorefrontContentType {
		t.Fatalf("public GET Content-Type = %q", contentType)
	}
	if _, err := store.client.HeadObject(context.Background(), &s3.HeadObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(upload.ObjectKey)}); err != nil {
		t.Fatalf("pending source should remain for retry safety: %v", err)
	}
}

func integrationValue(key, fallback string) string {
	if current := os.Getenv(key); current != "" {
		return current
	}
	return fallback
}

func TestNewRejectsPartialConfiguration(t *testing.T) {
	_, err := New(context.Background(), Config{Endpoint: "http://localhost:9000"})
	if err == nil {
		t.Fatal("New(partial) succeeded, want error")
	}
}

func TestPresignStorefrontUploadScopesKeyAndContentType(t *testing.T) {
	store, err := New(context.Background(), Config{
		Endpoint: "http://127.0.0.1:9000", Region: "us-east-1", Bucket: "safrat-uploads",
		AccessKeyID: "local-access", SecretAccessKey: "local-secret",
		PublicBaseURL: "http://127.0.0.1:9000/safrat-uploads", ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	operatorID := "00000000-0000-4000-8000-000000000001"
	upload, err := store.PresignStorefrontUpload(context.Background(), operatorID, "hero", 1024)
	if err != nil {
		t.Fatalf("PresignStorefrontUpload: %v", err)
	}
	parsed, err := url.Parse(upload.UploadURL)
	if err != nil {
		t.Fatalf("parse upload URL: %v", err)
	}
	if !strings.HasPrefix(parsed.Path, "/safrat-uploads/storefront-pending/"+operatorID+"/hero/") || !strings.HasSuffix(parsed.Path, ".webp") {
		t.Fatalf("unexpected tenant-scoped key: %s", parsed.Path)
	}
	if !strings.Contains(parsed.Query().Get("X-Amz-SignedHeaders"), "content-type") {
		t.Fatalf("content-type is not signed: %s", parsed.RawQuery)
	}
	if !strings.Contains(parsed.Query().Get("X-Amz-SignedHeaders"), "content-length") {
		t.Fatalf("content-length is not signed: %s", parsed.RawQuery)
	}
}

func TestPresignStorefrontMusicUploadUsesTenantScopedMP3(t *testing.T) {
	store, err := New(context.Background(), Config{
		Endpoint: "http://127.0.0.1:9000", Region: "us-east-1", Bucket: "safrat-uploads",
		AccessKeyID: "local-access", SecretAccessKey: "local-secret",
		PublicBaseURL: "http://127.0.0.1:9000/safrat-uploads", ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	operatorID := "00000000-0000-4000-8000-000000000001"
	upload, err := store.PresignStorefrontUpload(context.Background(), operatorID, "background-music", 1024)
	if err != nil {
		t.Fatalf("PresignStorefrontUpload: %v", err)
	}
	parsed, err := url.Parse(upload.UploadURL)
	if err != nil {
		t.Fatalf("parse upload URL: %v", err)
	}
	if !strings.HasPrefix(parsed.Path, "/safrat-uploads/storefront-pending/"+operatorID+"/background-music/") || !strings.HasSuffix(parsed.Path, ".mp3") {
		t.Fatalf("unexpected tenant-scoped key: %s", parsed.Path)
	}
	if upload.ContentType != StorefrontAudioContentType {
		t.Fatalf("content type = %q, want %q", upload.ContentType, StorefrontAudioContentType)
	}
}

func TestPresignStorefrontArticleUploadUsesTenantScopedWebP(t *testing.T) {
	store, err := New(context.Background(), Config{
		Endpoint: "http://127.0.0.1:9000", Region: "us-east-1", Bucket: "safrat-uploads",
		AccessKeyID: "local-access", SecretAccessKey: "local-secret",
		PublicBaseURL: "http://127.0.0.1:9000/safrat-uploads", ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	operatorID := "00000000-0000-4000-8000-000000000001"
	upload, err := store.PresignStorefrontUpload(context.Background(), operatorID, "article", 1024)
	if err != nil {
		t.Fatalf("PresignStorefrontUpload: %v", err)
	}
	parsed, err := url.Parse(upload.UploadURL)
	if err != nil {
		t.Fatalf("parse upload URL: %v", err)
	}
	if !strings.HasPrefix(parsed.Path, "/safrat-uploads/storefront-pending/"+operatorID+"/article/") || !strings.HasSuffix(parsed.Path, ".webp") {
		t.Fatalf("unexpected tenant-scoped key: %s", parsed.Path)
	}
	if upload.ContentType != StorefrontContentType {
		t.Fatalf("content type = %q, want %q", upload.ContentType, StorefrontContentType)
	}
}

func TestValidStorefrontPayloadRecognizesWebPAndMP3(t *testing.T) {
	imageSpec, _ := storefrontSpec("hero")
	aboutSpec, aboutErr := storefrontSpec("about")
	audioSpec, _ := storefrontSpec("background-music")
	if aboutErr != nil || aboutSpec.contentType != StorefrontContentType || aboutSpec.extension != ".webp" {
		t.Fatalf("about spec is not a verified WebP: %#v, %v", aboutSpec, aboutErr)
	}
	if !validStorefrontPayload(imageSpec, []byte("RIFFxxxxWEBP")) {
		t.Fatal("valid WebP signature rejected")
	}
	if !validStorefrontPayload(audioSpec, []byte{'I', 'D', '3', 4, 0, 0, 0, 0, 0, 0, 0xff, 0xfb, 0x90, 0x64}) {
		t.Fatal("valid ID3 MP3 signature rejected")
	}
	if !validStorefrontPayload(audioSpec, []byte{0xff, 0xfb, 0x90, 0x64}) {
		t.Fatal("valid MPEG frame signature rejected")
	}
	if validStorefrontPayload(audioSpec, []byte("ID3metadata")) {
		t.Fatal("ID3-only payload accepted as MP3")
	}
	if validStorefrontPayload(audioSpec, []byte("RIFFxxxxWEBP")) {
		t.Fatal("WebP signature accepted as MP3")
	}
}

func TestPresignStorefrontUploadValidatesInputs(t *testing.T) {
	store, err := New(context.Background(), Config{
		Endpoint: "http://127.0.0.1:9000", Region: "us-east-1", Bucket: "safrat-uploads",
		AccessKeyID: "local-access", SecretAccessKey: "local-secret",
		PublicBaseURL: "http://127.0.0.1:9000/safrat-uploads", ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, test := range []struct {
		operatorID, kind string
		size             int64
	}{
		{"invalid", "hero", 100},
		{"00000000-0000-4000-8000-000000000001", "document", 100},
		{"00000000-0000-4000-8000-000000000001", "hero", MaxStorefrontImageBytes + 1},
		{"00000000-0000-4000-8000-000000000001", "background-music", MaxStorefrontAudioBytes + 1},
	} {
		if _, err := store.PresignStorefrontUpload(context.Background(), test.operatorID, test.kind, test.size); err == nil {
			t.Fatalf("PresignStorefrontUpload(%q, %q, %d) succeeded", test.operatorID, test.kind, test.size)
		}
	}
}

// The tenant boundary for delivery photos. A stored key is evidence, so it must
// never be able to point at another operator's order — or anywhere outside the
// order it belongs to.
func TestHandoverKeyStaysInsideItsOrder(t *testing.T) {
	operator := "11111111-1111-1111-1111-111111111111"
	order := "22222222-2222-2222-2222-222222222222"
	other := "33333333-3333-3333-3333-333333333333"
	good := "handover/" + operator + "/" + order + "/photo.jpg"

	if err := ValidHandoverKey(operator, order, good); err != nil {
		t.Fatalf("kunci yang sah ditolak: %v", err)
	}

	refused := map[string]string{
		"tenant lain":        "handover/" + other + "/" + order + "/photo.jpg",
		"order lain":         "handover/" + operator + "/" + other + "/photo.jpg",
		"traversal":          "handover/" + operator + "/" + order + "/../../../etc/passwd",
		"prefix storefront":  "storefront/" + operator + "/hero.webp",
		"tanpa prefix":       "photo.jpg",
		"prefix menyerupai":  "handover/" + operator + "/" + order + "-lain/photo.jpg",
	}
	for name, key := range refused {
		if err := ValidHandoverKey(operator, order, key); err == nil {
			t.Fatalf("%s diterima: %q", name, key)
		}
	}

	if err := ValidHandoverKey("bukan-uuid", order, good); err == nil {
		t.Fatal("operator bukan UUID diterima")
	}
}
