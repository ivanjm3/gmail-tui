package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateEmailAddresses(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantBad string
		wantOK  bool
	}{
		{name: "empty", input: "", wantOK: true},
		{name: "single valid", input: "user@example.com", wantOK: true},
		{name: "list valid", input: "user@example.com, Other <other@example.com>", wantOK: true},
		{name: "invalid single", input: "bad-address", wantBad: "bad-address"},
		{name: "invalid second", input: "user@example.com, bad-address", wantBad: "bad-address"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBad, gotOK := ValidateEmailAddresses(tt.input)
			if gotOK != tt.wantOK || gotBad != tt.wantBad {
				t.Fatalf("ValidateEmailAddresses(%q) = (%q, %v), want (%q, %v)", tt.input, gotBad, gotOK, tt.wantBad, tt.wantOK)
			}
		})
	}
}

func TestValidateAttachmentPath(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	info, err := ValidateAttachmentPath(filePath)
	if err != nil || info == nil || info.Name() != "file.txt" {
		t.Fatalf("ValidateAttachmentPath() = (%v, %v)", info, err)
	}

	if _, err := ValidateAttachmentPath(filepath.Join(tempDir, "missing.txt")); err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("expected missing file error, got %v", err)
	}

	if _, err := ValidateAttachmentPath(tempDir); err == nil || !strings.Contains(err.Error(), "path is a directory") {
		t.Fatalf("expected directory error, got %v", err)
	}
}

func TestValidateAttachmentSize(t *testing.T) {
	tempDir := t.TempDir()
	smallPath := filepath.Join(tempDir, "small.txt")
	if err := os.WriteFile(smallPath, []byte("small"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	smallInfo, err := os.Stat(smallPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if err := ValidateAttachmentSize(smallInfo); err != nil {
		t.Fatalf("ValidateAttachmentSize() error = %v", err)
	}

	largePath := filepath.Join(tempDir, "large.bin")
	f, err := os.Create(largePath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := f.Truncate(maxAttachmentBytes + 1); err != nil {
		t.Fatalf("Truncate() error = %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	largeInfo, err := os.Stat(largePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if err := ValidateAttachmentSize(largeInfo); err == nil || !strings.Contains(err.Error(), "attachment too large") {
		t.Fatalf("expected attachment-too-large error, got %v", err)
	}
}
