//go:build integration

package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func TestIntegrationFetchInboxPreservesOrdering(t *testing.T) {
	t.Parallel()

	server := newMockGmailServer(t)
	server.listMessages = []*gmail.Message{{Id: "2"}, {Id: "1"}}
	server.metadataByID["1"] = metadataMessage("1", "First")
	server.metadataByID["2"] = metadataMessage("2", "Second")
	server.delays["1"] = 50 * time.Millisecond
	server.delays["2"] = 5 * time.Millisecond
	ts := httptest.NewServer(server)
	defer ts.Close()

	client := newIntegrationClient(t, ts, t.TempDir())
	emails, err := client.FetchInbox("in:inbox", 10)
	if err != nil {
		t.Fatalf("FetchInbox() error = %v", err)
	}

	if len(emails) != 2 {
		t.Fatalf("FetchInbox() len = %d, want 2", len(emails))
	}
	if emails[0].ID != "2" || emails[1].ID != "1" {
		t.Fatalf("FetchInbox() order = [%s %s], want [2 1]", emails[0].ID, emails[1].ID)
	}
}

func TestIntegrationEmptyInbox(t *testing.T) {
	t.Parallel()

	server := newMockGmailServer(t)
	ts := httptest.NewServer(server)
	defer ts.Close()

	client := newIntegrationClient(t, ts, t.TempDir())
	emails, err := client.FetchInbox("in:inbox", 10)
	if err != nil {
		t.Fatalf("FetchInbox() error = %v", err)
	}
	if len(emails) != 0 {
		t.Fatalf("FetchInbox() len = %d, want 0", len(emails))
	}
}

func TestIntegrationFetchEmail(t *testing.T) {
	t.Parallel()

	server := newMockGmailServer(t)
	server.fullByID["msg-1"] = &gmail.Message{
		Id:       "msg-1",
		ThreadId: "thread-1",
		LabelIds: []string{"INBOX"},
		Payload: &gmail.MessagePart{
			MimeType: "multipart/mixed",
			Headers: []*gmail.MessagePartHeader{
				{Name: "Subject", Value: "Hello"},
				{Name: "From", Value: "sender@example.com"},
				{Name: "Message-ID", Value: "<msg-1@example.com>"},
			},
			Parts: []*gmail.MessagePart{
				{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: base64.URLEncoding.EncodeToString([]byte("body"))}},
				{MimeType: "application/pdf", Filename: "report.pdf", Body: &gmail.MessagePartBody{AttachmentId: "att-1", Size: 7}},
			},
		},
	}
	ts := httptest.NewServer(server)
	defer ts.Close()

	client := newIntegrationClient(t, ts, t.TempDir())
	email, err := client.FetchEmail("msg-1")
	if err != nil {
		t.Fatalf("FetchEmail() error = %v", err)
	}

	if email.Subject != "Hello" || email.Body != "body" || len(email.Attachments) != 1 {
		t.Fatalf("FetchEmail() = %+v", email)
	}
}

func TestIntegrationSendEmail(t *testing.T) {
	t.Parallel()

	server := newMockGmailServer(t)
	ts := httptest.NewServer(server)
	defer ts.Close()

	client := newIntegrationClient(t, ts, t.TempDir())
	if err := client.SendEmail("to@example.com", "", "", "Subject", "Body", nil); err != nil {
		t.Fatalf("SendEmail() error = %v", err)
	}
	if len(server.sentRawMessages) != 1 || server.sentRawMessages[0] == "" {
		t.Fatalf("expected one raw message, got %v", server.sentRawMessages)
	}
}

func TestIntegrationDeleteEmail(t *testing.T) {
	t.Parallel()

	server := newMockGmailServer(t)
	ts := httptest.NewServer(server)
	defer ts.Close()

	client := newIntegrationClient(t, ts, t.TempDir())
	client.cacheStore(&Email{ID: "msg-1", FullLoaded: true, Body: "cached"})
	if err := client.DeleteEmail("msg-1"); err != nil {
		t.Fatalf("DeleteEmail() error = %v", err)
	}
	if len(server.trashedIDs) != 1 || server.trashedIDs[0] != "msg-1" {
		t.Fatalf("trashed IDs = %v", server.trashedIDs)
	}
	if _, ok := client.cache.Load("msg-1"); ok {
		t.Fatal("expected cache entry to be deleted")
	}
}

func TestIntegrationToggleRead(t *testing.T) {
	t.Parallel()

	server := newMockGmailServer(t)
	ts := httptest.NewServer(server)
	defer ts.Close()

	client := newIntegrationClient(t, ts, t.TempDir())
	client.cacheStore(&Email{ID: "msg-1", IsUnread: true, FullLoaded: true})

	newUnread, err := client.ToggleRead("msg-1", true)
	if err != nil {
		t.Fatalf("ToggleRead() error = %v", err)
	}
	if newUnread {
		t.Fatal("expected email to become read")
	}
	if len(server.modifiedIDs) != 1 || server.modifiedIDs[0] != "msg-1" {
		t.Fatalf("modified IDs = %v", server.modifiedIDs)
	}
}

func TestIntegrationFetchLabels(t *testing.T) {
	t.Parallel()

	server := newMockGmailServer(t)
	server.labels = []*gmail.Label{
		{Id: "INBOX", Name: "Inbox"},
		{Id: "STARRED", Name: "Starred"},
	}
	ts := httptest.NewServer(server)
	defer ts.Close()

	client := newIntegrationClient(t, ts, t.TempDir())
	labels, err := client.FetchLabels()
	if err != nil {
		t.Fatalf("FetchLabels() error = %v", err)
	}
	if len(labels) != 2 || labels[0].ID != "INBOX" || labels[1].Name != "Starred" {
		t.Fatalf("FetchLabels() = %+v", labels)
	}
}

func TestIntegrationDownloadAttachment(t *testing.T) {
	t.Parallel()

	downloadsDir := t.TempDir()
	server := newMockGmailServer(t)
	server.attachments["msg-1/att-1"] = []byte("attachment")
	ts := httptest.NewServer(server)
	defer ts.Close()

	client := newIntegrationClient(t, ts, downloadsDir)
	path, err := client.DownloadAttachment("msg-1", "att-1", "report.pdf")
	if err != nil {
		t.Fatalf("DownloadAttachment() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "attachment" {
		t.Fatalf("downloaded data = %q", string(data))
	}
}

type mockGmailServer struct {
	t *testing.T

	mu              sync.Mutex
	listMessages    []*gmail.Message
	metadataByID    map[string]*gmail.Message
	fullByID        map[string]*gmail.Message
	labels          []*gmail.Label
	attachments     map[string][]byte
	delays          map[string]time.Duration
	sentRawMessages []string
	trashedIDs      []string
	modifiedIDs     []string
}

func newMockGmailServer(t *testing.T) *mockGmailServer {
	t.Helper()

	return &mockGmailServer{
		t:            t,
		metadataByID: map[string]*gmail.Message{},
		fullByID:     map[string]*gmail.Message{},
		attachments:  map[string][]byte{},
		delays:       map[string]time.Duration{},
	}
}

func (s *mockGmailServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/gmail/v1/users/me/messages":
		s.writeJSON(w, &gmail.ListMessagesResponse{Messages: s.listMessages})
	case r.Method == http.MethodGet && r.URL.Path == "/gmail/v1/users/me/labels":
		s.writeJSON(w, &gmail.ListLabelsResponse{Labels: s.labels})
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages/send"):
		var req gmail.Message
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.t.Fatalf("decode send request: %v", err)
		}
		s.mu.Lock()
		s.sentRawMessages = append(s.sentRawMessages, req.Raw)
		s.mu.Unlock()
		s.writeJSON(w, &gmail.Message{Id: "sent"})
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/trash"):
		id := pathSegmentBeforeSuffix(r.URL.Path, "/trash")
		s.mu.Lock()
		s.trashedIDs = append(s.trashedIDs, id)
		s.mu.Unlock()
		s.writeJSON(w, &gmail.Message{Id: id})
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/modify"):
		id := pathSegmentBeforeSuffix(r.URL.Path, "/modify")
		s.mu.Lock()
		s.modifiedIDs = append(s.modifiedIDs, id)
		s.mu.Unlock()
		s.writeJSON(w, &gmail.Message{Id: id})
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/attachments/"):
		key := attachmentKey(r.URL.Path)
		data := s.attachments[key]
		s.writeJSON(w, &gmail.MessagePartBody{Data: base64.RawURLEncoding.EncodeToString(data), Size: int64(len(data))})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/gmail/v1/users/me/messages/"):
		id := path.Base(r.URL.Path)
		if delay := s.delays[id]; delay > 0 {
			time.Sleep(delay)
		}
		format := r.URL.Query().Get("format")
		if format == "full" {
			s.writeJSON(w, s.fullByID[id])
			return
		}
		s.writeJSON(w, s.metadataByID[id])
	default:
		http.NotFound(w, r)
	}
}

func (s *mockGmailServer) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.t.Fatalf("encode response: %v", err)
	}
}

func newIntegrationClient(t *testing.T, ts *httptest.Server, downloadsDir string) *Client {
	t.Helper()

	srv, err := gmail.NewService(
		context.Background(),
		option.WithHTTPClient(ts.Client()),
		option.WithEndpoint(ts.URL+"/"),
	)
	if err != nil {
		t.Fatalf("gmail.NewService() error = %v", err)
	}

	cfg := defaultConfig()
	cfg.DownloadsDir = downloadsDir
	cfg.MaxConcurrent = 4

	return &Client{
		srv:    srv,
		cache:  newLRUCache(cfg.CacheMaxSize),
		cfg:    cfg,
		logger: NoopLogger(),
	}
}

func metadataMessage(id, subject string) *gmail.Message {
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

func pathSegmentBeforeSuffix(path, suffix string) string {
	trimmed := strings.TrimSuffix(path, suffix)
	parts := strings.Split(trimmed, "/")
	return parts[len(parts)-1]
}

func attachmentKey(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-3] + "/" + parts[len(parts)-1]
}
