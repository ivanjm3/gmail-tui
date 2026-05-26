package ui

import (
	"errors"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Feature: gmail-tui-optimization, Property 2: Status bar error prefix
func TestPropertyStatusBarErrorPrefix(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		model := newTestModel(&MockClient{hasSendScope: true})
		model.state = stateLoading

		updated, _ := model.Update(errMsg{err: errors.New(t.StringN(40))})
		next := updated.(Model)

		if !strings.HasPrefix(next.statusMessage, "Error: ") {
			t.Fatalf("statusMessage = %q, want Error prefix", next.statusMessage)
		}
		if next.state != stateInbox {
			t.Fatalf("state = %v, want %v", next.state, stateInbox)
		}
	})
}

// Feature: gmail-tui-optimization, Property 13: Reply subject prefix
func TestPropertyReplySubjectPrefix(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		subject := rapid.Sample(t,
			t.StringN(40),
			"re: "+t.StringN(20),
			"RE:"+t.StringN(20),
			"Re: "+t.StringN(20),
		)
		if got := formatReplySubject(subject); !strings.HasPrefix(got, "Re: ") {
			t.Fatalf("formatReplySubject(%q) = %q", subject, got)
		}
	})
}
