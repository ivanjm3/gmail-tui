package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/gmail/v1"
	"pgregory.net/rapid"
)

var consecutiveWhitespaceRe = regexp.MustCompile(`\s{2,}`)

// Feature: gmail-tui-optimization, Property 1: Error wrapping universality
func TestPropertyErrorWrappingUniversality(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		marker := errors.New("transport failure")
		cfg := defaultConfig()
		cfg.DownloadsDir = t.TempDir()
		client := newHTTPTestClient(t, cfg, func(*http.Request) (*http.Response, error) {
			return nil, marker
		})

		var err error
		switch t.IntRange(0, 8) {
		case 0:
			_, err = client.FetchInbox("in:inbox", 5)
		case 1:
			_, err = client.FetchEmail("msg-1")
		case 2:
			_, err = client.FetchLabels()
		case 3:
			err = client.DeleteEmail("msg-1")
		case 4:
			_, err = client.ToggleRead("msg-1", true)
		case 5:
			err = client.SendEmail("to@example.com", "", "", "Subject", "Body", nil)
		case 6:
			_, err = client.DownloadAttachment("msg-1", "att-1", "report.pdf")
		case 7:
			_, _, err = client.FetchInboxPage("in:inbox", 5, "")
		case 8:
			err = client.SendReply("to@example.com", "Subject", "Body", "<msg>", "<ref>", nil)
		}

		if err == nil || !errors.Is(err, marker) {
			t.Fatalf("expected wrapped marker error, got %v", err)
		}
	})
}

// Feature: gmail-tui-optimization, Property 3: Email address validation correctness
func TestPropertyEmailAddressValidation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		valid := randomEmailAddress(t)
		if !ValidateEmailAddress(valid) {
			t.Fatalf("expected valid email address: %q", valid)
		}

		invalid := rapid.Sample(t,
			randomToken(t, 1, 12)+" "+randomToken(t, 1, 12),
			randomToken(t, 1, 12)+"@",
			"@"+randomToken(t, 1, 12)+".com",
			randomToken(t, 1, 12)+"@@"+randomToken(t, 1, 12)+".com",
			randomToken(t, 1, 12)+"@"+randomToken(t, 1, 12)+". com",
			randomToken(t, 1, 12)+"@"+randomToken(t, 1, 12)+"..com",
			randomToken(t, 1, 12)+","+randomToken(t, 1, 12)+"@example.com",
		)
		if ValidateEmailAddress(invalid) {
			t.Fatalf("expected invalid email address: %q", invalid)
		}
	})
}

// Feature: gmail-tui-optimization, Property 4: Concurrent fetch ordering
func TestPropertyConcurrentFetchOrdering(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := t.IntRange(1, 8)
		ids := make([]string, count)
		messages := make([]*gmail.Message, count)
		for i := range ids {
			ids[i] = "msg-" + randomToken(t, 4, 8)
			messages[i] = &gmail.Message{Id: ids[i]}
		}

		client := newHTTPTestClient(t, defaultConfig(), func(r *http.Request) (*http.Response, error) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/gmail/v1/users/me/messages":
				return jsonResponse(mustJSON(t, &gmail.ListMessagesResponse{Messages: messages})), nil
			case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/gmail/v1/users/me/messages/"):
				id := path.Base(r.URL.Path)
				for i, wantID := range ids {
					if id == wantID {
						time.Sleep(time.Duration(count-i) * time.Millisecond)
						return jsonResponse(mustJSON(t, propertyMetadataMessage(id, "subject-"+id))), nil
					}
				}
			}
			return nil, errors.New("unexpected request")
		})

		emails, err := client.FetchInbox("in:inbox", int64(count))
		if err != nil {
			t.Fatalf("FetchInbox() error = %v", err)
		}
		if len(emails) != len(ids) {
			t.Fatalf("FetchInbox() len = %d, want %d", len(emails), len(ids))
		}
		for i := range ids {
			if emails[i].ID != ids[i] {
				t.Fatalf("FetchInbox() order mismatch at %d: got %q want %q", i, emails[i].ID, ids[i])
			}
		}
	})
}

// Feature: gmail-tui-optimization, Property 5: Cache full-entry protection
func TestPropertyCacheFullEntryProtection(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cache := newLRUCache(10)
		id := randomToken(t, 4, 10)
		body := t.StringN(40)
		full := &Email{
			ID:         id,
			Subject:    "subject-" + id,
			Body:       body,
			Labels:     []string{"UNREAD"},
			IsUnread:   true,
			FullLoaded: true,
		}
		cache.Store(full)
		cache.Store(&Email{
			ID:         id,
			Subject:    "metadata-" + id,
			Labels:     []string{"INBOX"},
			IsUnread:   false,
			FullLoaded: false,
		})

		got, ok := cache.Load(id)
		if !ok {
			t.Fatalf("expected cache hit for %q", id)
		}
		if !got.FullLoaded || got.Body != body {
			t.Fatalf("expected full entry preservation, got %+v", got)
		}
		if got.IsUnread {
			t.Fatalf("expected mutable unread flag to refresh, got %+v", got)
		}
	})
}

// Feature: gmail-tui-optimization, Property 6: HTML stripping completeness
func TestPropertyStripHTMLNoAngleBrackets(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		if got := stripHTML(t.StringN(80)); strings.ContainsAny(got, "<>") {
			t.Fatalf("stripHTML() returned angle brackets: %q", got)
		}
	})
}

// Feature: gmail-tui-optimization, Property 7: HTML whitespace normalization
func TestPropertyStripHTMLWhitespaceNormalization(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		if got := stripHTML(t.StringN(80)); consecutiveWhitespaceRe.MatchString(got) {
			t.Fatalf("stripHTML() returned consecutive whitespace: %q", got)
		}
	})
}

// Feature: gmail-tui-optimization, Property 8: IndentText line prefix
func TestPropertyIndentTextLinePrefix(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		input := t.StringN(80)
		if input == "" {
			input = "x"
		}
		for i, line := range strings.Split(IndentText(input), "\n") {
			if !strings.HasPrefix(line, "> ") {
				t.Fatalf("line %d = %q does not have quote prefix", i, line)
			}
		}
	})
}

// Feature: gmail-tui-optimization, Property 9: Base64 round-trip
func TestPropertyDecodeBodyRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		input := t.StringN(80)
		encoded := base64.URLEncoding.EncodeToString([]byte(input))
		if got := decodeBody(encoded); got != input {
			t.Fatalf("decodeBody(encode(%q)) = %q", input, got)
		}
	})
}

// Feature: gmail-tui-optimization, Property 10: sanitizeFilename path safety
func TestPropertySanitizeFilenamePathSafety(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		input := t.StringN(80)
		safe := sanitizeFilename(input)
		if strings.ContainsAny(safe, `/\`) || strings.Contains(safe, "..") {
			t.Fatalf("sanitizeFilename(%q) = %q is unsafe", input, safe)
		}

		downloadsDir := t.TempDir()
		fullPath := filepath.Join(downloadsDir, safe)
		absDownloads, err := filepath.Abs(downloadsDir)
		if err != nil {
			t.Fatalf("Abs() error = %v", err)
		}
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			t.Fatalf("Abs() error = %v", err)
		}
		if !strings.HasPrefix(absPath, absDownloads+string(filepath.Separator)) {
			t.Fatalf("resolved path %q escaped downloads dir %q", absPath, absDownloads)
		}
	})
}

// Feature: gmail-tui-optimization, Property 11: FormatDate non-panic
func TestPropertyFormatDateNonPanic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		if got := FormatDate(t.StringN(80)); got == "" {
			t.Fatal("FormatDate() returned empty string")
		}
	})
}

// Feature: gmail-tui-optimization, Property 12: LRU cache size bound
func TestPropertyLRUCacheSizeBound(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxSize := t.IntRange(1, 20)
		cache := newLRUCache(maxSize)
		ops := t.IntRange(maxSize, maxSize+100)

		for i := 0; i < ops; i++ {
			cache.Store(&Email{ID: randomToken(t, 4, 12)})
			if cache.Size() > maxSize {
				t.Fatalf("cache size = %d, want <= %d", cache.Size(), maxSize)
			}
		}
	})
}

// Feature: gmail-tui-optimization, Property 14: MIME recursion safety
func TestPropertyMIMERecursionSafety(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		depth := t.IntRange(0, 50)
		part := &gmail.MessagePart{
			MimeType: "text/plain",
			Body:     &gmail.MessagePartBody{Data: base64.URLEncoding.EncodeToString([]byte("body"))},
		}
		for i := 0; i < depth; i++ {
			part = &gmail.MessagePart{
				MimeType: "multipart/mixed",
				Parts:    []*gmail.MessagePart{part},
			}
		}
		_ = extractPlainText(part)
	})
}

func mustJSON(t *rapid.T, v any) string {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(data)
}

func randomEmailAddress(t *rapid.T) string {
	return randomToken(t, 1, 12) + "@" + randomToken(t, 1, 12) + "." + randomToken(t, 2, 6)
}

func randomToken(t *rapid.T, min, max int) string {
	t.Helper()

	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	n := t.IntRange(min, max)
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(alphabet[t.IntRange(0, len(alphabet)-1)])
	}
	return b.String()
}

func propertyMetadataMessage(id, subject string) *gmail.Message {
	return &gmail.Message{
		Id:       id,
		ThreadId: "thread-" + id,
		Snippet:  subject,
		Payload: &gmail.MessagePart{
			Headers: []*gmail.MessagePartHeader{
				{Name: "Subject", Value: subject},
				{Name: "From", Value: "sender@example.com"},
			},
		},
	}
}
