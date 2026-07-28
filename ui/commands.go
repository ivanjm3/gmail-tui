package ui

import (
	"time"

	"github.com/rdx40/gmail-tui/api"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	inboxQuery = "in:inbox category:primary"
)

// ---------- message types ----------

type inboxLoadedMsg struct {
	emails    []api.Email
	nextToken string
}
type emailOpenedMsg struct{ email *api.Email }
type emailSentMsg struct{}
type emailDeletedMsg struct{ id string }
type emailArchivedMsg struct{ id string }
type readToggledMsg struct {
	id       string
	isUnread bool
}
type labelsLoadedMsg struct{ labels []api.Label }
type searchResultMsg struct {
	emails []api.Email
	query  string
	title  string // list title to show for these results
}
type attachmentSavedMsg struct{ path string }
type errMsg struct{ err error }
type statusMsg struct{ message string }
type clearStatusMsg struct{}
type nextPageLoadedMsg struct {
	emails    []api.Email
	nextToken string
}
type userProfileLoadedMsg struct{ email string }

// ---------- commands ----------

// fetchInbox fetches the first page of the primary inbox, keeping the
// next-page token so pagination works from the initial load.
func fetchInbox(client api.ClientInterface, maxResults int64) tea.Cmd {
	return func() tea.Msg {
		emails, nextToken, err := client.FetchInboxPage(inboxQuery, maxResults, "")
		if err != nil {
			return errMsg{err: err}
		}
		return inboxLoadedMsg{emails: emails, nextToken: nextToken}
	}
}

// openEmail fetches a full email by ID.
func openEmail(client api.ClientInterface, id string) tea.Cmd {
	return func() tea.Msg {
		email, err := client.FetchEmail(id)
		if err != nil {
			return errMsg{err: err}
		}
		return emailOpenedMsg{email: email}
	}
}

// sendEmailCmd sends a new email.
func sendEmailCmd(client api.ClientInterface, to, cc, bcc, subject, body string, attachments []string) tea.Cmd {
	return func() tea.Msg {
		if err := client.SendEmail(to, cc, bcc, subject, body, attachments); err != nil {
			return errMsg{err: err}
		}
		return emailSentMsg{}
	}
}

// sendReplyCmd sends a reply with proper threading headers.
func sendReplyCmd(client api.ClientInterface, to, subject, body, inReplyTo, references string, attachments []string) tea.Cmd {
	return func() tea.Msg {
		if err := client.SendReply(to, subject, body, inReplyTo, references, attachments); err != nil {
			return errMsg{err: err}
		}
		return emailSentMsg{}
	}
}

// deleteEmailCmd moves an email to trash.
func deleteEmailCmd(client api.ClientInterface, id string) tea.Cmd {
	return func() tea.Msg {
		if err := client.DeleteEmail(id); err != nil {
			return errMsg{err: err}
		}
		return emailDeletedMsg{id: id}
	}
}

// archiveEmailCmd removes an email from the inbox (removes the INBOX label).
func archiveEmailCmd(client api.ClientInterface, id string) tea.Cmd {
	return func() tea.Msg {
		if err := client.ArchiveEmail(id); err != nil {
			return errMsg{err: err}
		}
		return emailArchivedMsg{id: id}
	}
}

// toggleReadCmd flips the read/unread state of an email.
func toggleReadCmd(client api.ClientInterface, id string, currentlyUnread bool) tea.Cmd {
	return func() tea.Msg {
		newUnread, err := client.ToggleRead(id, currentlyUnread)
		if err != nil {
			return errMsg{err: err}
		}
		return readToggledMsg{id: id, isUnread: newUnread}
	}
}

// searchCmd searches emails matching query.
func searchCmd(client api.ClientInterface, query string, maxResults int64) tea.Cmd {
	return func() tea.Msg {
		emails, err := client.Search(query, maxResults)
		if err != nil {
			return errMsg{err: err}
		}
		return searchResultMsg{emails: emails, query: query, title: "Search: " + query}
	}
}

// fetchByLabelCmd fetches emails with the given label.
func fetchByLabelCmd(client api.ClientInterface, labelID, labelName string, maxResults int64) tea.Cmd {
	return func() tea.Msg {
		emails, err := client.FetchByLabel(labelID, maxResults)
		if err != nil {
			return errMsg{err: err}
		}
		return searchResultMsg{emails: emails, query: "", title: labelName}
	}
}

// fetchLabelsCmd fetches all Gmail labels.
func fetchLabelsCmd(client api.ClientInterface) tea.Cmd {
	return func() tea.Msg {
		labels, err := client.FetchLabels()
		if err != nil {
			return errMsg{err: err}
		}
		return labelsLoadedMsg{labels: labels}
	}
}

// downloadAttachmentCmd downloads an attachment to the downloads directory.
func downloadAttachmentCmd(client api.ClientInterface, msgID string, att api.Attachment) tea.Cmd {
	return func() tea.Msg {
		path, err := client.DownloadAttachment(msgID, att.ID, att.Filename)
		if err != nil {
			return errMsg{err: err}
		}
		return attachmentSavedMsg{path: path}
	}
}

// fetchNextPageCmd fetches a page of inbox emails using a page token.
func fetchNextPageCmd(client api.ClientInterface, query string, maxResults int64, pageToken string) tea.Cmd {
	return func() tea.Msg {
		emails, nextToken, err := client.FetchInboxPage(query, maxResults, pageToken)
		if err != nil {
			return errMsg{err: err}
		}
		return nextPageLoadedMsg{emails: emails, nextToken: nextToken}
	}
}

// fetchUserProfileCmd fetches the authenticated user's email address.
func fetchUserProfileCmd(client api.ClientInterface) tea.Cmd {
	return func() tea.Msg {
		email, err := client.GetUserProfile()
		if err != nil {
			// Non-fatal: just return empty profile
			return userProfileLoadedMsg{email: ""}
		}
		return userProfileLoadedMsg{email: email}
	}
}

// notify sends a one-shot status message; the Update handler auto-clears it.
func notify(msg string) tea.Cmd {
	return func() tea.Msg { return statusMsg{message: msg} }
}

// clearStatusAfter clears the status bar after the given duration.
func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return clearStatusMsg{} })
}
