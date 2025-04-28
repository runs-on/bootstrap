package main

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"bootstrap/internal/s3client"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type mockS3Client struct {
	getObjectOutput *s3.GetObjectOutput
	err             error
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.getObjectOutput, nil
}

func TestDownloadFromS3(t *testing.T) {
	testCases := []struct {
		name        string
		bucket      string
		key         string
		mockOutput  *s3.GetObjectOutput
		mockErr     error
		wantErr     bool
		wantContent string
	}{
		{
			name:   "successful download",
			bucket: "test-bucket",
			key:    "test-key",
			mockOutput: &s3.GetObjectOutput{
				Body: io.NopCloser(strings.NewReader("test content")),
			},
			wantContent: "test content",
		},
		{
			name:    "s3 error",
			bucket:  "test-bucket",
			key:     "test-key",
			mockErr: &types.NoSuchKey{},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockSvc := &mockS3Client{
				getObjectOutput: tc.mockOutput,
				err:             tc.mockErr,
			}

			result, err := s3client.Download(context.Background(), mockSvc, tc.bucket, tc.key)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			content, err := io.ReadAll(result)
			if err != nil {
				t.Fatalf("failed to read content: %v", err)
			}
			if string(content) != tc.wantContent {
				t.Errorf("got content %q, want %q", string(content), tc.wantContent)
			}
		})
	}
}

func TestParseS3URL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantBucket  string
		wantKey     string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid S3 URL",
			url:        "s3://my-bucket/path/to/file.txt",
			wantBucket: "my-bucket",
			wantKey:    "path/to/file.txt",
			wantErr:    false,
		},
		{
			name:        "invalid scheme",
			url:         "http://my-bucket/file.txt",
			wantErr:     true,
			errContains: "not an S3 URL",
		},
		{
			name:        "invalid URL format",
			url:         "not-a-url",
			wantErr:     true,
			errContains: "not an S3 URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, key, err := parseS3URL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if bucket != tt.wantBucket {
				t.Errorf("bucket = %q, want %q", bucket, tt.wantBucket)
			}
			if key != tt.wantKey {
				t.Errorf("key = %q, want %q", key, tt.wantKey)
			}
		})
	}
}

func TestExecuteFile(t *testing.T) {
	err := executeFile("nonexistent-file")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestShutdownSystemDebug(t *testing.T) {
	// capture stdout
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	done := make(chan struct{})
	go func() {
		// should return quickly and not actually shutdown
		err := shutdownSystem(10*time.Millisecond, true)
		if err != nil {
			t.Errorf("expected nil error in debug mode, got %v", err)
		}
		w.Close()
		done <- struct{}{}
	}()

	// read output
	var out strings.Builder
	io.Copy(&out, r)
	<-done
	os.Stdout = origStdout

	if !strings.Contains(out.String(), "Debug: Would execute command:") {
		t.Errorf("expected debug output, got: %q", out.String())
	}
}
