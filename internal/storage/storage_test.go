package storage

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestR2PresignPutReturnsSignedURL(t *testing.T) {
	store, err := New("r2", "https://account.r2.cloudflarestorage.com", "test-access-key", "test-secret-key")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	address, err := store.PresignPut(context.Background(), "nexo-media", "nexo/demo/episode.mp4", "video/mp4", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignPut() error = %v", err)
	}
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatalf("parse presigned URL: %v", err)
	}
	if parsed.Scheme != "https" || !strings.Contains(parsed.Host, "r2.cloudflarestorage.com") {
		t.Fatalf("unexpected presigned endpoint: %s://%s", parsed.Scheme, parsed.Host)
	}
	if parsed.Query().Get("X-Amz-Signature") == "" {
		t.Fatal("presigned URL has no signature")
	}
	if parsed.Query().Get("X-Amz-Expires") != "900" {
		t.Fatalf("URL expiry = %q, want 900 seconds", parsed.Query().Get("X-Amz-Expires"))
	}
}
