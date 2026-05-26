package api

// ClientInterface defines all operations the ui package needs from the API layer.
// Using an interface here enables mock injection in tests and decouples the ui
// package from the concrete *Client implementation.
type ClientInterface interface {
	FetchInbox(query string, max int64) ([]Email, error)
	FetchInboxPage(query string, max int64, pageToken string) ([]Email, string, error)
	Search(query string, max int64) ([]Email, error)
	FetchByLabel(labelID string, max int64) ([]Email, error)
	FetchEmail(id string) (*Email, error)
	FetchLabels() ([]Label, error)
	GetUserProfile() (string, error)
	DeleteEmail(id string) error
	ToggleRead(id string, currentlyUnread bool) (bool, error)
	SendEmail(to, cc, bcc, subject, body string, attachments []string) error
	SendReply(to, subject, body, inReplyTo, references string, attachments []string) error
	DownloadAttachment(msgID, attachmentID, filename string) (string, error)
	CacheSize() int
	HasSendScope() bool
}
