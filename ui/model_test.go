package ui

import (
	"math"
	"strings"
	"testing"

	"github.com/rdx40/gmail-tui/api"
)

func TestTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{name: "empty", input: "", max: 5, want: ""},
		{name: "shorter than max", input: "hello", max: 10, want: "hello"},
		{name: "equal to max", input: "hello", max: 5, want: "hello"},
		{name: "longer than max", input: "abcdefghij", max: 7, want: "abcd..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := truncate(tt.input, tt.max); got != tt.want {
				t.Fatalf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestHumanSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		bytes int64
		want  string
	}{
		{bytes: 0, want: "0 B"},
		{bytes: 1, want: "1 B"},
		{bytes: 1023, want: "1023 B"},
		{bytes: 1024, want: "1.0 KB"},
		{bytes: 1024 * 1024, want: "1.0 MB"},
		{bytes: 1024 * 1024 * 1024, want: "1.0 GB"},
		{bytes: math.MaxInt64, want: "8.0 EB"},
	}

	for _, tt := range tests {
		if got := humanSize(tt.bytes); got != tt.want {
			t.Fatalf("humanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestEmailsToItems(t *testing.T) {
	t.Parallel()

	emails := []api.Email{
		{ID: "1", Subject: "First", From: "first@example.com"},
		{ID: "2", Subject: "Second", From: "second@example.com"},
	}

	items := emailsToItems(emails)
	if len(items) != len(emails) {
		t.Fatalf("emailsToItems() len = %d, want %d", len(items), len(emails))
	}

	item, ok := items[0].(emailItem)
	if !ok || item.email.ID != "1" {
		t.Fatalf("emailsToItems()[0] = %#v, want emailItem for ID 1", items[0])
	}
}

func TestLabelsToItems(t *testing.T) {
	t.Parallel()

	labels := []api.Label{{ID: "INBOX", Name: "Inbox"}, {ID: "STARRED", Name: "Starred"}}
	items := labelsToItems(labels)

	if len(items) != len(labels) {
		t.Fatalf("labelsToItems() len = %d, want %d", len(items), len(labels))
	}

	item, ok := items[1].(labelItem)
	if !ok || item.label.Name != "Starred" {
		t.Fatalf("labelsToItems()[1] = %#v, want labelItem for Starred", items[1])
	}
}

func TestEmailItemMethods(t *testing.T) {
	t.Parallel()

	email := api.Email{
		Subject:  "Subject",
		From:     "sender@example.com",
		Snippet:  "preview text",
		IsUnread: true,
	}
	item := emailItem{email: email}

	title := item.Title()
	if !strings.Contains(title, "Subject") {
		t.Fatalf("Title() = %q, want subject included", title)
	}
	if title == "Subject" {
		t.Fatalf("Title() = %q, want unread marker/padding", title)
	}
	if got := item.Description(); got != "sender@example.com - preview text" {
		t.Fatalf("Description() = %q", got)
	}
	if got := item.FilterValue(); got != "Subject sender@example.com" {
		t.Fatalf("FilterValue() = %q", got)
	}

	item.email.IsUnread = false
	readTitle := item.Title()
	if !strings.HasSuffix(readTitle, "Subject") {
		t.Fatalf("read Title() = %q, want subject suffix", readTitle)
	}
	if title == readTitle {
		t.Fatalf("expected read and unread titles to differ, both were %q", title)
	}
}

func TestComposeFormReset(t *testing.T) {
	t.Parallel()

	form := newComposeForm()
	form.from.SetValue("old-from@example.com")
	form.to.SetValue("to@example.com")
	form.cc.SetValue("cc@example.com")
	form.bcc.SetValue("bcc@example.com")
	form.subject.SetValue("Subject")
	form.body.SetValue("Body")
	form.attachments = []string{"file.txt"}
	form.addingAttach = true
	form.focused = 5

	form.reset("from@example.com")

	if form.from.Value() != "from@example.com" {
		t.Fatalf("from = %q, want %q", form.from.Value(), "from@example.com")
	}
	if form.to.Value() != "" || form.cc.Value() != "" || form.bcc.Value() != "" || form.subject.Value() != "" || form.body.Value() != "" {
		t.Fatalf("reset did not clear fields: %+v", form)
	}
	if len(form.attachments) != 0 || form.addingAttach {
		t.Fatalf("reset did not clear attachment state: %+v", form)
	}
	if form.focused != 1 {
		t.Fatalf("focused = %d, want 1", form.focused)
	}
}
