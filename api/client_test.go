package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func TestClientCacheStorePreservesFullEntries(t *testing.T) {
	t.Parallel()

	client := &Client{
		cache:  newLRUCache(10),
		cfg:    defaultConfig(),
		logger: NoopLogger(),
	}

	full := &Email{
		ID:         "msg-1",
		Subject:    "Original Subject",
		Body:       "full body",
		Labels:     []string{"UNREAD"},
		IsUnread:   true,
		FullLoaded: true,
	}
	client.cacheStore(full)

	client.cacheStore(&Email{
		ID:         "msg-1",
		Subject:    "Metadata Subject",
		Labels:     []string{"INBOX"},
		IsUnread:   false,
		FullLoaded: false,
	})

	got, ok := client.cache.Load("msg-1")
	if !ok {
		t.Fatal("expected cached email")
	}
	if !got.FullLoaded || got.Body != "full body" {
		t.Fatalf("expected full entry to be preserved, got %+v", got)
	}
	if got.IsUnread {
		t.Fatalf("expected mutable fields to refresh, got %+v", got)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "INBOX" {
		t.Fatalf("expected labels to refresh, got %+v", got)
	}
	if got.Subject != "Original Subject" {
		t.Fatalf("expected original subject to remain, got %+v", got)
	}
}

func TestSanitizeFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: "unnamed"},
		{name: "simple", input: "report.pdf", want: "report.pdf"},
		{name: "relative traversal", input: "../report.pdf", want: "report.pdf"},
		{name: "absolute unix path", input: "/tmp/report.pdf", want: "report.pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := sanitizeFilename(tt.input); got != tt.want {
				t.Fatalf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}

	t.Run("never leaves separators or traversal", func(t *testing.T) {
		t.Parallel()

		for _, input := range []string{
			"..\\..\\evil.txt",
			"..//evil.txt",
			"C:\\temp\\evil.txt",
			"/var/tmp/evil.txt",
			"../../../../evil.txt",
		} {
			got := sanitizeFilename(input)
			if strings.ContainsAny(got, `/\`) || strings.Contains(got, "..") {
				t.Fatalf("sanitizeFilename(%q) = %q is not path-safe", input, got)
			}
		}
	})
}

func TestDownloadAttachmentKeepsPathWithinDownloadsDir(t *testing.T) {
	t.Parallel()

	downloadsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(downloadsDir, "report.pdf"), []byte("existing"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client := newHTTPTestClient(t, &Config{
		MaxResults:       10,
		SearchMaxResults: 30,
		DownloadsDir:     downloadsDir,
		MaxConcurrent:    2,
		CacheMaxSize:     10,
		LogLevel:         "INFO",
	}, func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.Path, "/attachments/att-1") {
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
		body := `{"data":"` + base64.RawURLEncoding.EncodeToString([]byte("hello")) + `","size":5}`
		return jsonResponse(body), nil
	})

	path, err := client.DownloadAttachment("msg-1", "att-1", "../report.pdf")
	if err != nil {
		t.Fatalf("DownloadAttachment() error = %v", err)
	}

	absDownloads, err := filepath.Abs(downloadsDir)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	if !strings.HasPrefix(path, absDownloads+string(filepath.Separator)) {
		t.Fatalf("download path %q escaped downloads dir %q", path, absDownloads)
	}
	if filepath.Base(path) != "report(1).pdf" {
		t.Fatalf("expected deduplicated filename, got %q", filepath.Base(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("downloaded data = %q, want %q", string(data), "hello")
	}
}

func TestClientConvenienceMethods(t *testing.T) {
	t.Parallel()

	client := newHTTPTestClient(t, defaultConfig(), func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/gmail/v1/users/me/messages":
			return jsonResponse(`{"messages":[{"id":"msg-1"}]}`), nil
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/gmail/v1/users/me/messages/"):
			return jsonResponse(`{
				"id":"msg-1",
				"threadId":"thread-1",
				"payload":{"headers":[
					{"name":"Subject","value":"Subject"},
					{"name":"From","value":"sender@example.com"}
				]}
			}`), nil
		case r.Method == http.MethodGet && r.URL.Path == "/gmail/v1/users/me/profile":
			return jsonResponse(`{"emailAddress":"me@example.com"}`), nil
		}
		return nil, errors.New("unexpected request")
	})

	if got, err := client.Search("from:sender", 1); err != nil || len(got) != 1 || got[0].ID != "msg-1" {
		t.Fatalf("Search() = (%+v, %v)", got, err)
	}
	if got, err := client.FetchByLabel("INBOX", 1); err != nil || len(got) != 1 || got[0].ID != "msg-1" {
		t.Fatalf("FetchByLabel() = (%+v, %v)", got, err)
	}
	if got, err := client.GetUserProfile(); err != nil || got != "me@example.com" {
		t.Fatalf("GetUserProfile() = (%q, %v)", got, err)
	}
	if client.CacheSize() == 0 {
		t.Fatalf("CacheSize() = %d, want cache entries after fetches", client.CacheSize())
	}
	if !client.HasSendScope() {
		t.Fatal("HasSendScope() = false, want true")
	}
}

func TestFetchEmailReturnsCachedFullEntry(t *testing.T) {
	t.Parallel()

	want := &Email{ID: "msg-1", Body: "cached", FullLoaded: true}
	client := &Client{
		cache:  newLRUCache(2),
		cfg:    defaultConfig(),
		logger: NoopLogger(),
	}
	client.cacheStore(want)

	got, err := client.FetchEmail("msg-1")
	if err != nil {
		t.Fatalf("FetchEmail() error = %v", err)
	}
	if got != want && (got.ID != want.ID || got.Body != want.Body || !got.FullLoaded) {
		t.Fatalf("FetchEmail() = %+v, want %+v", got, want)
	}
}

func TestFetchLabelsSuccess(t *testing.T) {
	t.Parallel()

	client := newHTTPTestClient(t, defaultConfig(), func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet && r.URL.Path == "/gmail/v1/users/me/labels" {
			return jsonResponse(`{"labels":[{"id":"INBOX","name":"Inbox"}]}`), nil
		}
		return nil, errors.New("unexpected request")
	})

	labels, err := client.FetchLabels()
	if err != nil {
		t.Fatalf("FetchLabels() error = %v", err)
	}
	if len(labels) != 1 || labels[0].ID != "INBOX" || labels[0].Name != "Inbox" {
		t.Fatalf("FetchLabels() = %+v", labels)
	}
}

func TestFetchInboxPageSuccess(t *testing.T) {
	t.Parallel()

	client := newHTTPTestClient(t, defaultConfig(), func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/gmail/v1/users/me/messages":
			return jsonResponse(`{"messages":[{"id":"msg-1"}],"nextPageToken":"page-2"}`), nil
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/gmail/v1/users/me/messages/"):
			return jsonResponse(`{
				"id":"msg-1",
				"threadId":"thread-1",
				"payload":{"headers":[
					{"name":"Subject","value":"Subject"},
					{"name":"From","value":"sender@example.com"}
				]}
			}`), nil
		}
		return nil, errors.New("unexpected request")
	})

	emails, nextToken, err := client.FetchInboxPage("in:inbox", 1, "")
	if err != nil {
		t.Fatalf("FetchInboxPage() error = %v", err)
	}
	if len(emails) != 1 || emails[0].ID != "msg-1" || nextToken != "page-2" {
		t.Fatalf("FetchInboxPage() = (%+v, %q)", emails, nextToken)
	}
}

func TestWriteAttachment(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writeAttachment(writer, filePath); err != nil {
		t.Fatalf("writeAttachment() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !strings.Contains(buf.String(), "filename=\"file.txt\"") {
		t.Fatalf("multipart body missing filename header: %q", buf.String())
	}

	largePath := filepath.Join(tempDir, "large.bin")
	f, err := os.Create(largePath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := f.Truncate(maxAttachmentSize + 1); err != nil {
		t.Fatalf("Truncate() error = %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	writer = multipart.NewWriter(io.Discard)
	if err := writeAttachment(writer, largePath); err == nil || !strings.Contains(err.Error(), "attachment too large") {
		t.Fatalf("expected attachment-too-large error, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newHTTPTestClient(t testing.TB, cfg *Config, fn roundTripFunc) *Client {
	t.Helper()

	srv, err := gmail.NewService(
		context.Background(),
		option.WithHTTPClient(&http.Client{Transport: fn}),
		option.WithEndpoint("https://gmail.test/"),
	)
	if err != nil {
		t.Fatalf("gmail.NewService() error = %v", err)
	}

	return &Client{
		srv:              srv,
		cache:            newLRUCache(cfg.CacheMaxSize),
		cfg:              cfg,
		logger:           NoopLogger(),
		sendScopeGranted: true,
	}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
