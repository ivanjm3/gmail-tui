package api

import (
	"fmt"
	"net/mail"
	"os"
	"strings"
)

const maxAttachmentBytes = 25 * 1024 * 1024 // 25 MB

// ValidateEmailAddress reports whether addr is a valid RFC 5322 email address.
// Uses net/mail.ParseAddress for standards-compliant parsing.
func ValidateEmailAddress(addr string) bool {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return false
	}
	_, err := mail.ParseAddress(addr)
	return err == nil
}

// ValidateEmailAddresses validates a comma-separated list of email addresses.
// Returns the first invalid address and false if any address is invalid.
func ValidateEmailAddresses(addrs string) (string, bool) {
	trimmed := strings.TrimSpace(addrs)
	if trimmed == "" {
		return "", true // empty is valid (optional field)
	}
	list, err := mail.ParseAddressList(trimmed)
	if err == nil && len(list) > 0 {
		return "", true
	}

	for _, candidate := range strings.Split(trimmed, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if !ValidateEmailAddress(candidate) {
			return candidate, false
		}
	}

	return trimmed, false
}

// ValidateAttachmentPath checks that the file at path exists and is readable.
// Returns the FileInfo and nil on success, or nil and an error on failure.
func ValidateAttachmentPath(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("cannot access file: %s: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", path)
	}
	return info, nil
}

// ValidateAttachmentSize checks that the file does not exceed the 25 MB limit.
func ValidateAttachmentSize(info os.FileInfo) error {
	if info.Size() > maxAttachmentBytes {
		return fmt.Errorf("attachment too large (max 25 MB): %s", info.Name())
	}
	return nil
}
