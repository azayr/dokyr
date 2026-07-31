package s3

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRequestUsesPathStyleAndSignsExpectedScope(t *testing.T) {
	client, err := New(Config{
		Region:         "auto",
		Bucket:         "dokyr-backups",
		Endpoint:       "https://account.r2.cloudflarestorage.com",
		AccessKey:      "access",
		SecretKey:      "secret",
		ForcePathStyle: true,
		Secure:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := client.request(context.Background(), "GET", "dokyr/server/backup.tar.gz", nil, emptySHA256,
		time.Date(2026, 7, 31, 10, 11, 12, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := request.URL.String(), "https://account.r2.cloudflarestorage.com/dokyr-backups/dokyr/server/backup.tar.gz"; got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
	authorization := request.Header.Get("Authorization")
	if !strings.Contains(authorization, "Credential=access/20260731/auto/s3/aws4_request") {
		t.Fatalf("authorization has unexpected scope: %q", authorization)
	}
	if !strings.Contains(authorization, "SignedHeaders=host;x-amz-content-sha256;x-amz-date") {
		t.Fatalf("authorization has unexpected headers: %q", authorization)
	}
}

func TestObjectURLUsesVirtualHostStyle(t *testing.T) {
	client, err := New(Config{
		Region:    "nyc3",
		Bucket:    "dokyr",
		Endpoint:  "https://nyc3.digitaloceanspaces.com",
		AccessKey: "access",
		SecretKey: "secret",
		Secure:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	target, err := client.objectURL("server/backup.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := target.String(), "https://dokyr.nyc3.digitaloceanspaces.com/server/backup.tar.gz"; got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}
