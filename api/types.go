package api

// Email represents a Gmail message.
type Email struct {
	ID          string
	ThreadID    string
	Subject     string
	From        string
	To          string
	CC          string
	BCC         string
	Date        string
	Snippet     string
	Body        string
	Labels      []string
	IsUnread    bool
	Attachments []Attachment
	FullLoaded  bool   // true when body+attachments have been fetched
	MessageID   string // value of Message-ID header (for reply threading)
	References  string // value of References header (for reply threading)
	InReplyTo   string // value of In-Reply-To header
}

// Attachment represents an email attachment.
type Attachment struct {
	ID       string
	Filename string
	MimeType string
	Size     int64
}

// Label represents a Gmail label.
type Label struct {
	ID   string
	Name string
}
