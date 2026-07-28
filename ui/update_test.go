package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/rdx40/gmail-tui/api"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateErrMsgTransitionsToInbox(t *testing.T) {
	t.Parallel()

	model := newTestModel(&MockClient{hasSendScope: true})
	model.state = stateLoading

	updated, cmd := model.Update(errMsg{err: errors.New("boom")})
	if cmd == nil {
		t.Fatalf("expected clear-status command")
	}

	next := updated.(Model)
	if next.state != stateInbox {
		t.Fatalf("state = %v, want %v", next.state, stateInbox)
	}
	if next.statusMessage != "Error: boom" {
		t.Fatalf("statusMessage = %q, want %q", next.statusMessage, "Error: boom")
	}
}

func TestUpdateInboxLoadedTransitionsToInbox(t *testing.T) {
	t.Parallel()

	model := newTestModel(&MockClient{hasSendScope: true})
	model.state = stateLoading

	updated, _ := model.Update(inboxLoadedMsg{emails: []api.Email{
		{ID: "1", Subject: "First"},
		{ID: "2", Subject: "Second"},
	}})

	next := updated.(Model)
	if next.state != stateInbox {
		t.Fatalf("state = %v, want %v", next.state, stateInbox)
	}
	if len(next.emailList.Items()) != 2 {
		t.Fatalf("email count = %d, want 2", len(next.emailList.Items()))
	}
	if next.totalFetched != 2 {
		t.Fatalf("totalFetched = %d, want 2", next.totalFetched)
	}
}

func TestUpdateEmailOpenedTransitionsToViewing(t *testing.T) {
	t.Parallel()

	model := newTestModel(&MockClient{hasSendScope: true})
	model.state = stateLoading
	model.width = 80
	model.height = 24

	email := &api.Email{ID: "1", Body: "hello"}
	updated, _ := model.Update(emailOpenedMsg{email: email})
	next := updated.(Model)

	if next.state != stateViewing {
		t.Fatalf("state = %v, want %v", next.state, stateViewing)
	}
	if next.currentEmail == nil || next.currentEmail.ID != "1" {
		t.Fatalf("currentEmail = %+v, want ID 1", next.currentEmail)
	}
	if next.viewport.Width != 80 {
		t.Fatalf("viewport width = %d, want 80", next.viewport.Width)
	}
}

func TestComposeSendRequiresRecipient(t *testing.T) {
	t.Parallel()

	model := newTestModel(&MockClient{hasSendScope: true})
	model.state = stateComposing
	model.compose.reset("from@example.com")
	model.compose.subject.SetValue("Subject")

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatalf("expected validation command")
	}

	next := updated.(Model)
	msg := mustImmediateMsg(t, cmd)
	if _, ok := msg.(errMsg); !ok {
		t.Fatalf("cmd() = %#v, want errMsg", msg)
	}

	afterErr, _ := next.Update(msg)
	final := afterErr.(Model)
	if final.state != stateComposing {
		t.Fatalf("state = %v, want %v", final.state, stateComposing)
	}
	if final.statusMessage != "Error: recipient address is required" {
		t.Fatalf("statusMessage = %q", final.statusMessage)
	}
}

func TestComposeSendRejectsInvalidAddress(t *testing.T) {
	t.Parallel()

	model := newTestModel(&MockClient{hasSendScope: true})
	model.state = stateComposing
	model.compose.reset("from@example.com")
	model.compose.to.SetValue("not-an-email")
	model.compose.subject.SetValue("Subject")

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	msg := mustImmediateMsg(t, cmd)
	afterErr, _ := updated.(Model).Update(msg)
	final := afterErr.(Model)

	if final.statusMessage != "Error: invalid email address: not-an-email" {
		t.Fatalf("statusMessage = %q", final.statusMessage)
	}
}

func TestComposeSendPromptsForEmptySubject(t *testing.T) {
	t.Parallel()

	mock := &MockClient{hasSendScope: true}
	model := newTestModel(mock)
	model.state = stateComposing
	model.compose.reset("from@example.com")
	model.compose.to.SetValue("to@example.com")

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd != nil {
		t.Fatalf("expected no send command before confirmation")
	}

	next := updated.(Model)
	if !next.confirmNoSubject {
		t.Fatal("expected no-subject confirmation state")
	}
	if next.statusMessage != "Send with no subject? [y/n]" {
		t.Fatalf("statusMessage = %q", next.statusMessage)
	}

	confirmed, cmd := next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	msg := mustImmediateMsg(t, cmd)
	if _, ok := msg.(emailSentMsg); !ok {
		t.Fatalf("cmd() = %#v, want emailSentMsg", msg)
	}
	if mock.lastSentSubject != "" {
		t.Fatalf("sent subject = %q, want empty", mock.lastSentSubject)
	}
	if confirmed.(Model).confirmNoSubject {
		t.Fatal("expected confirmation flag to clear after send")
	}
}

func TestReplySendNormalizesSubjectPrefix(t *testing.T) {
	t.Parallel()

	mock := &MockClient{hasSendScope: true}
	model := newTestModel(mock)
	model.state = stateReplying
	model.currentEmail = &api.Email{Body: "body"}
	model.replyTo = &api.Email{
		From:       "Sender <sender@example.com>",
		Date:       "Jan 02, 2006 15:04",
		Subject:    "re: follow up",
		MessageID:  "<msg@example.com>",
		References: "<ref@example.com>",
	}
	model.replyBody.SetValue("Reply text")

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	msg := mustImmediateMsg(t, cmd)
	if _, ok := msg.(emailSentMsg); !ok {
		t.Fatalf("cmd() = %#v, want emailSentMsg", msg)
	}
	if mock.lastReplySubject != "Re: follow up" {
		t.Fatalf("reply subject = %q, want %q", mock.lastReplySubject, "Re: follow up")
	}
}

func TestHandleComposeAttachRejectsMissingFile(t *testing.T) {
	t.Parallel()

	model := newTestModel(&MockClient{hasSendScope: true})
	model.state = stateComposing
	model.compose.addingAttach = true
	model.compose.attachInput.SetValue("missing-file.txt")

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := mustImmediateMsg(t, cmd)
	afterErr, _ := updated.(Model).Update(msg)
	final := afterErr.(Model)

	if len(final.compose.attachments) != 0 {
		t.Fatalf("attachments = %v, want none", final.compose.attachments)
	}
	if final.statusMessage != "Error: file not found: missing-file.txt" {
		t.Fatalf("statusMessage = %q", final.statusMessage)
	}
}

func TestUpdateResizeSetsComponentSizes(t *testing.T) {
	t.Parallel()

	model := newTestModel(&MockClient{})
	model.state = stateLoading

	// Send resize message
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	next := updated.(Model)

	if next.width != 100 || next.height != 40 {
		t.Fatalf("model dimensions = %dx%d, want 100x40", next.width, next.height)
	}

	// Verify emailList size was set even if we were in loading state
	if next.emailList.Width() != 100 {
		t.Fatalf("emailList width = %d, want 100", next.emailList.Width())
	}
	if next.emailList.Height() != 37 { // 40 - 3
		t.Fatalf("emailList height = %d, want 37", next.emailList.Height())
	}

	// Verify labelList size
	if next.labelList.Height() != 37 {
		t.Fatalf("labelList height = %d, want 37", next.labelList.Height())
	}

	// Verify viewport size
	if next.viewport.Width != 100 {
		t.Fatalf("viewport width = %d, want 100", next.viewport.Width)
	}
	if next.viewport.Height != 33 { // 40 - 7
		t.Fatalf("viewport height = %d, want 33", next.viewport.Height)
	}
}

func mustImmediateMsg(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()

	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("expected command to return a message")
	}
	return msg
}

func newTestModel(client api.ClientInterface) Model {
	model := New(client, &api.Config{
		MaxResults:       10,
		SearchMaxResults: 30,
		DownloadsDir:     "downloads",
		MaxConcurrent:    5,
		CacheMaxSize:     100,
		LogLevel:         "INFO",
	})
	model.noStyle = true
	model.width = 80
	model.height = 24
	return model
}

type MockClient struct {
	inboxEmails      []api.Email
	inboxErr         error
	email            *api.Email
	emailErr         error
	labels           []api.Label
	labelsErr        error
	profileEmail     string
	profileErr       error
	deleteErr        error
	archiveErr       error
	toggleUnread     bool
	toggleErr        error
	sendErr          error
	replyErr         error
	downloadPath     string
	downloadErr      error
	pageEmails       []api.Email
	pageNextToken    string
	pageErr          error
	cacheSize        int
	hasSendScope     bool
	lastSentSubject  string
	lastReplySubject string
	lastReplyBody    string
	lastReplyTo      string
	lastReplyRefs    string
	lastReplyMessage string
}

func (m *MockClient) FetchInbox(string, int64) ([]api.Email, error) {
	return m.inboxEmails, m.inboxErr
}

func (m *MockClient) FetchInboxPage(string, int64, string) ([]api.Email, string, error) {
	return m.pageEmails, m.pageNextToken, m.pageErr
}

func (m *MockClient) Search(string, int64) ([]api.Email, error) {
	return nil, nil
}

func (m *MockClient) FetchByLabel(string, int64) ([]api.Email, error) {
	return nil, nil
}

func (m *MockClient) FetchEmail(string) (*api.Email, error) {
	return m.email, m.emailErr
}

func (m *MockClient) FetchLabels() ([]api.Label, error) {
	return m.labels, m.labelsErr
}

func (m *MockClient) GetUserProfile() (string, error) {
	return m.profileEmail, m.profileErr
}

func (m *MockClient) DeleteEmail(string) error {
	return m.deleteErr
}

func (m *MockClient) ArchiveEmail(string) error {
	return m.archiveErr
}

func (m *MockClient) ToggleRead(string, bool) (bool, error) {
	return m.toggleUnread, m.toggleErr
}

func (m *MockClient) SendEmail(to, cc, bcc, subject, body string, attachments []string) error {
	m.lastReplyTo = to
	m.lastSentSubject = subject
	return m.sendErr
}

func (m *MockClient) SendReply(to, subject, body, inReplyTo, references string, attachments []string) error {
	m.lastReplyTo = to
	m.lastReplySubject = subject
	m.lastReplyBody = body
	m.lastReplyMessage = inReplyTo
	m.lastReplyRefs = references
	return m.replyErr
}

func (m *MockClient) DownloadAttachment(string, string, string) (string, error) {
	return m.downloadPath, m.downloadErr
}

func (m *MockClient) CacheSize() int {
	return m.cacheSize
}

func (m *MockClient) HasSendScope() bool {
	return m.hasSendScope
}

func TestPaginationTokenFlow(t *testing.T) {
	t.Parallel()

	model := newTestModel(&MockClient{})
	model.state = stateLoading

	// Initial inbox load carries the next-page token.
	updated, _ := model.Update(inboxLoadedMsg{emails: []api.Email{{ID: "1"}}, nextToken: "T1"})
	m := updated.(Model)
	if m.pageToken != "T1" || m.currentToken != "" || m.currentPage != 1 {
		t.Fatalf("after load: pageToken=%q currentToken=%q page=%d", m.pageToken, m.currentToken, m.currentPage)
	}

	// Next page: history records the token that fetched the current page.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(Model)
	if m.currentToken != "T1" || len(m.pageHistory) != 1 || m.pageHistory[0] != "" || m.currentPage != 2 {
		t.Fatalf("after next: currentToken=%q history=%v page=%d", m.currentToken, m.pageHistory, m.currentPage)
	}

	updated, _ = m.Update(nextPageLoadedMsg{emails: []api.Email{{ID: "2"}}, nextToken: "T2"})
	m = updated.(Model)
	if m.pageToken != "T2" {
		t.Fatalf("after page load: pageToken=%q, want T2", m.pageToken)
	}

	// Prev page: pops the token that fetched page 1 (empty = first page).
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updated.(Model)
	if m.currentToken != "" || len(m.pageHistory) != 0 || m.currentPage != 1 {
		t.Fatalf("after prev: currentToken=%q history=%v page=%d", m.currentToken, m.pageHistory, m.currentPage)
	}
}

func TestSearchResultsResetPagination(t *testing.T) {
	t.Parallel()

	model := newTestModel(&MockClient{})
	model.pageToken = "T9"
	model.currentToken = "T8"
	model.pageHistory = []string{"", "T8"}
	model.currentPage = 3

	updated, _ := model.Update(searchResultMsg{emails: []api.Email{{ID: "1"}}, query: "q", title: "Search: q"})
	m := updated.(Model)
	if m.pageToken != "" || m.currentToken != "" || len(m.pageHistory) != 0 || m.currentPage != 1 {
		t.Fatalf("pagination not reset: %+v", m)
	}
	if m.emailList.Title != "Search: q" {
		t.Fatalf("title = %q, want %q", m.emailList.Title, "Search: q")
	}
}

func TestReplyReferencesIncludeMessageID(t *testing.T) {
	t.Parallel()

	mock := &MockClient{hasSendScope: true}
	model := newTestModel(mock)
	model.state = stateReplying
	model.currentEmail = &api.Email{Body: "body"}
	model.replyTo = &api.Email{
		From:       "sender@example.com",
		Subject:    "hi",
		MessageID:  "<msg@example.com>",
		References: "<ref@example.com>",
	}

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if _, ok := mustImmediateMsg(t, cmd).(emailSentMsg); !ok {
		t.Fatal("expected emailSentMsg")
	}
	if mock.lastReplyRefs != "<ref@example.com> <msg@example.com>" {
		t.Fatalf("references = %q, want original refs + message id", mock.lastReplyRefs)
	}
	if mock.lastReplyMessage != "<msg@example.com>" {
		t.Fatalf("in-reply-to = %q", mock.lastReplyMessage)
	}
}

func TestArchiveRemovesEmailFromList(t *testing.T) {
	t.Parallel()

	model := newTestModel(&MockClient{})
	model.state = stateInbox
	model.emailList.SetItems(emailsToItems([]api.Email{{ID: "1"}, {ID: "2"}}))
	model.totalFetched = 2

	updated, _ := model.Update(emailArchivedMsg{id: "1"})
	m := updated.(Model)
	if len(m.emailList.Items()) != 1 {
		t.Fatalf("items = %d, want 1", len(m.emailList.Items()))
	}
	if m.totalFetched != 1 {
		t.Fatalf("totalFetched = %d, want 1", m.totalFetched)
	}
	if m.statusMessage != "Email archived" {
		t.Fatalf("statusMessage = %q", m.statusMessage)
	}
}

func TestReadToggleUpdatesCurrentEmail(t *testing.T) {
	t.Parallel()

	model := newTestModel(&MockClient{})
	model.state = stateViewing
	model.currentEmail = &api.Email{ID: "1", IsUnread: true}
	model.emailList.SetItems(emailsToItems([]api.Email{{ID: "1", IsUnread: true}}))

	updated, _ := model.Update(readToggledMsg{id: "1", isUnread: false})
	m := updated.(Model)
	if m.currentEmail.IsUnread {
		t.Fatal("currentEmail.IsUnread not updated")
	}
}

func TestTruncateMultibyteSafe(t *testing.T) {
	t.Parallel()

	got := truncate("héllo wörld émails über", 10)
	if !strings.HasSuffix(got, "...") || len([]rune(got)) != 10 {
		t.Fatalf("truncate = %q, want 10 runes ending in ...", got)
	}
}
