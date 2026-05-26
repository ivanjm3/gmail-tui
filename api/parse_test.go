package api

import (
	"encoding/base64"
	"strings"
	"testing"

	"google.golang.org/api/gmail/v1"
)

func encodeBody(s string) string {
	return base64.URLEncoding.EncodeToString([]byte(s))
}

func TestStripHTML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "plain text", input: "hello world", want: "hello world"},
		{name: "simple tag", input: "<b>bold</b>", want: "bold"},
		{name: "nested tags", input: "<div><p>text</p></div>", want: "text"},
		{name: "named entity", input: "&amp;", want: "&"},
		{name: "encoded tag removed", input: "&lt;b&gt;", want: ""},
		{name: "named nbsp trimmed", input: "&nbsp;", want: ""},
		{name: "numeric entity decimal", input: "&#160;", want: ""},
		{name: "numeric entity hex", input: "&#x00A0;", want: ""},
		{name: "whitespace collapse", input: "a  b\t\tc\n\n", want: "a b c"},
		{name: "whitespace after stripping", input: "<br/>   <br/>", want: ""},
		{name: "unicode", input: "<p>héllo wörld</p>", want: "héllo wörld"},
		{name: "script content", input: "<script>alert('xss')</script>", want: "alert('xss')"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := stripHTML(tt.input); got != tt.want {
				t.Fatalf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripHTML_NoAngleBrackets(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"<b>bold</b>",
		"&lt;script&gt;",
		"2 < 3",
		"",
		"héllo",
	}

	for _, input := range inputs {
		if got := stripHTML(input); strings.ContainsAny(got, "<>") {
			t.Fatalf("stripHTML(%q) = %q contains angle brackets", input, got)
		}
	}
}

func TestDecodeBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "valid base64url", input: encodeBody("hello"), want: "hello"},
		{name: "missing padding", input: strings.TrimRight(encodeBody("hello"), "="), want: "hello"},
		{name: "invalid", input: "!!!invalid!!!", want: "(failed to decode body)"},
		{name: "unicode", input: encodeBody("Hello Wörld!"), want: "Hello Wörld!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := decodeBody(tt.input); got != tt.want {
				t.Fatalf("decodeBody(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantSame bool
		want     string
	}{
		{name: "rfc1123z", input: "Mon, 02 Jan 2006 15:04:05 -0700"},
		{name: "rfc1123", input: "Mon, 02 Jan 2006 15:04:05 MST"},
		{name: "day without leading zero", input: "Mon, 2 Jan 2006 15:04:05 -0700"},
		{name: "without day name", input: "2 Jan 2006 15:04:05 -0700"},
		{name: "rfc822z", input: "02 Jan 06 15:04 -0700"},
		{name: "rfc822", input: "02 Jan 06 15:04 MST"},
		{name: "invalid returns raw", input: "not a date", wantSame: true},
		{name: "empty returns placeholder", input: "", want: "Unknown date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := FormatDate(tt.input)
			if tt.want != "" {
				if got != tt.want {
					t.Fatalf("FormatDate(%q) = %q, want %q", tt.input, got, tt.want)
				}
				return
			}
			if tt.wantSame {
				if got != tt.input {
					t.Fatalf("FormatDate(%q) = %q, want raw %q", tt.input, got, tt.input)
				}
				return
			}
			if got == "" {
				t.Fatalf("FormatDate(%q) returned empty string", tt.input)
			}
		})
	}
}

func TestIndentText(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"hello",
		"line1\nline2\nline3",
		"héllo\nwörld",
	}

	for _, input := range tests {
		got := IndentText(input)
		for i, line := range strings.Split(got, "\n") {
			if !strings.HasPrefix(line, "> ") {
				t.Fatalf("IndentText(%q) line %d = %q, want prefix \"> \"", input, i, line)
			}
		}
	}
}

func TestExtractPlainText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload *gmail.MessagePart
		want    string
	}{
		{
			name: "text plain only",
			payload: &gmail.MessagePart{
				MimeType: "text/plain",
				Body:     &gmail.MessagePartBody{Data: encodeBody("hello world")},
			},
			want: "hello world",
		},
		{
			name: "text html only",
			payload: &gmail.MessagePart{
				MimeType: "text/html",
				Body:     &gmail.MessagePartBody{Data: encodeBody("<b>hello</b>")},
			},
			want: "hello",
		},
		{
			name: "multipart alternative prefers plain",
			payload: &gmail.MessagePart{
				MimeType: "multipart/alternative",
				Parts: []*gmail.MessagePart{
					{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: encodeBody("plain")}},
					{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: encodeBody("<p>html</p>")}},
				},
			},
			want: "plain",
		},
		{
			name: "multipart alternative falls back to html",
			payload: &gmail.MessagePart{
				MimeType: "multipart/alternative",
				Parts: []*gmail.MessagePart{
					{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: encodeBody("<p>html only</p>")}},
				},
			},
			want: "html only",
		},
		{
			name: "nested multipart",
			payload: &gmail.MessagePart{
				MimeType: "multipart/mixed",
				Parts: []*gmail.MessagePart{
					{
						MimeType: "multipart/alternative",
						Parts: []*gmail.MessagePart{
							{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: encodeBody("nested")}},
						},
					},
				},
			},
			want: "nested",
		},
		{
			name: "empty payload",
			payload: &gmail.MessagePart{
				MimeType: "text/plain",
				Body:     &gmail.MessagePartBody{Data: ""},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := extractPlainText(tt.payload); got != tt.want {
				t.Fatalf("extractPlainText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractPlainText_DeepNesting(t *testing.T) {
	t.Parallel()

	leaf := &gmail.MessagePart{
		MimeType: "text/plain",
		Body:     &gmail.MessagePartBody{Data: encodeBody("too deep")},
	}
	current := leaf
	for range 30 {
		current = &gmail.MessagePart{
			MimeType: "multipart/mixed",
			Parts:    []*gmail.MessagePart{current},
		}
	}

	if got := extractPlainText(current); got != "" {
		t.Fatalf("extractPlainText() = %q, want empty string after depth guard", got)
	}
}

func TestParseMessage(t *testing.T) {
	t.Parallel()

	t.Run("missing payload", func(t *testing.T) {
		t.Parallel()

		email := parseMessage(&gmail.Message{
			Id:       "1",
			ThreadId: "thread-1",
			Snippet:  "snippet",
		}, false)

		if email.ID != "1" || email.ThreadID != "thread-1" || email.Snippet != "snippet" {
			t.Fatalf("unexpected parsed email: %+v", email)
		}
		if email.Subject != "" || email.Body != "" || email.IsUnread {
			t.Fatalf("expected zero-value fields for missing payload, got %+v", email)
		}
	})

	t.Run("missing headers", func(t *testing.T) {
		t.Parallel()

		email := parseMessage(&gmail.Message{
			Id: "2",
			Payload: &gmail.MessagePart{
				MimeType: "text/plain",
				Body:     &gmail.MessagePartBody{Data: encodeBody("body")},
			},
		}, false)

		if email.Subject != "" || email.From != "" || email.Date != "" {
			t.Fatalf("expected missing headers to stay empty, got %+v", email)
		}
	})

	t.Run("unread label present", func(t *testing.T) {
		t.Parallel()

		email := parseMessage(&gmail.Message{
			LabelIds: []string{"INBOX", "UNREAD"},
			Payload:  &gmail.MessagePart{},
		}, false)

		if !email.IsUnread {
			t.Fatal("expected unread label to set IsUnread")
		}
	})

	t.Run("unread label absent", func(t *testing.T) {
		t.Parallel()

		email := parseMessage(&gmail.Message{
			LabelIds: []string{"INBOX"},
			Payload:  &gmail.MessagePart{},
		}, false)

		if email.IsUnread {
			t.Fatal("expected IsUnread to stay false")
		}
	})

	t.Run("snippet truncation at boundary", func(t *testing.T) {
		t.Parallel()

		snippet := strings.Repeat("a", 80)
		email := parseMessage(&gmail.Message{
			Snippet: snippet,
			Payload: &gmail.MessagePart{},
		}, false)

		if email.Snippet != snippet {
			t.Fatalf("snippet should not be truncated, got %q", email.Snippet)
		}
	})

	t.Run("snippet truncation above boundary", func(t *testing.T) {
		t.Parallel()

		email := parseMessage(&gmail.Message{
			Snippet: strings.Repeat("a", 100),
			Payload: &gmail.MessagePart{},
		}, false)

		if len(email.Snippet) != 80 || !strings.HasSuffix(email.Snippet, "...") {
			t.Fatalf("unexpected truncated snippet %q", email.Snippet)
		}
	})

	t.Run("full payload extracts body attachments and threading headers", func(t *testing.T) {
		t.Parallel()

		email := parseMessage(&gmail.Message{
			Id:       "6",
			ThreadId: "thread-6",
			LabelIds: []string{"INBOX", "UNREAD"},
			Snippet:  "héllo world",
			Payload: &gmail.MessagePart{
				MimeType: "multipart/mixed",
				Headers: []*gmail.MessagePartHeader{
					{Name: "Subject", Value: "Test Subject"},
					{Name: "From", Value: "Sender <sender@example.com>"},
					{Name: "To", Value: "recipient@example.com"},
					{Name: "Cc", Value: "copy@example.com"},
					{Name: "Bcc", Value: "blind@example.com"},
					{Name: "Date", Value: "Mon, 02 Jan 2006 15:04:05 -0700"},
					{Name: "Message-ID", Value: "<msg@example.com>"},
					{Name: "References", Value: "<ref1@example.com> <ref2@example.com>"},
					{Name: "In-Reply-To", Value: "<ref2@example.com>"},
				},
				Parts: []*gmail.MessagePart{
					{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: encodeBody("héllo body")}},
					{MimeType: "application/pdf", Filename: "report.pdf", Body: &gmail.MessagePartBody{AttachmentId: "att-1", Size: 1234}},
				},
			},
		}, true)

		if email.Subject != "Test Subject" || email.From != "Sender <sender@example.com>" || email.To != "recipient@example.com" {
			t.Fatalf("unexpected header extraction: %+v", email)
		}
		if email.CC != "copy@example.com" || email.BCC != "blind@example.com" {
			t.Fatalf("unexpected cc/bcc extraction: %+v", email)
		}
		if email.MessageID != "<msg@example.com>" || email.References != "<ref1@example.com> <ref2@example.com>" || email.InReplyTo != "<ref2@example.com>" {
			t.Fatalf("unexpected threading headers: %+v", email)
		}
		if email.Body != "héllo body" || !email.FullLoaded {
			t.Fatalf("expected full body load, got %+v", email)
		}
		if len(email.Attachments) != 1 || email.Attachments[0].Filename != "report.pdf" {
			t.Fatalf("unexpected attachments: %+v", email.Attachments)
		}
	})

	t.Run("full payload without text uses fallback", func(t *testing.T) {
		t.Parallel()

		email := parseMessage(&gmail.Message{
			Payload: &gmail.MessagePart{
				MimeType: "multipart/mixed",
				Parts: []*gmail.MessagePart{
					{MimeType: "application/octet-stream", Filename: "blob.bin"},
				},
			},
		}, true)

		if email.Body != "(no text content found)" {
			t.Fatalf("unexpected fallback body %q", email.Body)
		}
	})
}

func TestFindAttachments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		part *gmail.MessagePart
		want []Attachment
	}{
		{
			name: "no attachments",
			part: &gmail.MessagePart{
				MimeType: "text/plain",
				Body:     &gmail.MessagePartBody{Data: encodeBody("body")},
			},
			want: nil,
		},
		{
			name: "single attachment",
			part: &gmail.MessagePart{
				MimeType: "application/pdf",
				Filename: "report.pdf",
				Body:     &gmail.MessagePartBody{AttachmentId: "att-1", Size: 1024},
			},
			want: []Attachment{{ID: "att-1", Filename: "report.pdf", MimeType: "application/pdf", Size: 1024}},
		},
		{
			name: "nested attachments",
			part: &gmail.MessagePart{
				MimeType: "multipart/mixed",
				Parts: []*gmail.MessagePart{
					{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: encodeBody("body")}},
					{
						MimeType: "multipart/alternative",
						Parts: []*gmail.MessagePart{
							{MimeType: "image/png", Filename: "chart.png", Body: &gmail.MessagePartBody{AttachmentId: "att-2", Size: 2048}},
						},
					},
				},
			},
			want: []Attachment{{ID: "att-2", Filename: "chart.png", MimeType: "image/png", Size: 2048}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := findAttachments(tt.part)
			if len(got) != len(tt.want) {
				t.Fatalf("findAttachments() len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("findAttachments()[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
