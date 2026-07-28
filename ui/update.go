package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rdx40/gmail-tui/api"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles all messages. Never performs blocking work.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		return m.handleResize(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)

	// --- async results ---

	case inboxLoadedMsg:
		m.emailList.SetItems(emailsToItems(msg.emails))
		m.emailList.Title = "Inbox"
		m.totalFetched = len(msg.emails)
		m.pageToken = msg.nextToken
		m.currentToken = ""
		m.pageHistory = nil
		m.currentPage = 1
		m.state = stateInbox
		return m, nil

	case nextPageLoadedMsg:
		m.emailList.SetItems(emailsToItems(msg.emails))
		m.pageToken = msg.nextToken
		m.totalFetched = len(msg.emails)
		m.state = stateInbox
		return m, nil

	case userProfileLoadedMsg:
		m.userEmail = msg.email
		return m, nil

	case emailOpenedMsg:
		m.currentEmail = msg.email
		m.state = stateViewing
		m.viewport.Width = m.width
		vpHeight := m.height - 7
		if vpHeight < 1 {
			vpHeight = 1
		}
		m.viewport.Height = vpHeight
		m.viewport.SetContent(msg.email.Body)
		m.viewport.GotoTop()
		return m, nil

	case emailSentMsg:
		m.state = stateLoading
		m.statusMessage = "Email sent!"
		return m, tea.Batch(m.spinner.Tick, fetchInbox(m.client, int64(m.config.MaxResults)), clearStatusAfter(3*time.Second))

	case emailDeletedMsg:
		m = m.removeListedEmail(msg.id)
		m.statusMessage = "Email moved to trash"
		return m, clearStatusAfter(3 * time.Second)

	case emailArchivedMsg:
		m = m.removeListedEmail(msg.id)
		m.statusMessage = "Email archived"
		return m, clearStatusAfter(3 * time.Second)

	case readToggledMsg:
		items := m.emailList.Items()
		for i, item := range items {
			if ei, ok := item.(emailItem); ok && ei.email.ID == msg.id {
				ei.email.IsUnread = msg.isUnread
				m.emailList.SetItem(i, ei)
				break
			}
		}
		if m.currentEmail != nil && m.currentEmail.ID == msg.id {
			m.currentEmail.IsUnread = msg.isUnread
		}
		action := "marked as read"
		if msg.isUnread {
			action = "marked as unread"
		}
		m.statusMessage = "Email " + action
		return m, clearStatusAfter(3 * time.Second)

	case labelsLoadedMsg:
		m.labelList.SetItems(labelsToItems(msg.labels))
		m.state = stateLabels
		return m, nil

	case searchResultMsg:
		m.emailList.SetItems(emailsToItems(msg.emails))
		if msg.title != "" {
			m.emailList.Title = msg.title
		}
		// Results replace the inbox listing; inbox page tokens no longer apply.
		m.pageToken = ""
		m.currentToken = ""
		m.pageHistory = nil
		m.currentPage = 1
		m.totalFetched = len(msg.emails)
		m.state = stateInbox
		if len(msg.emails) == 0 && msg.query != "" {
			m.statusMessage = fmt.Sprintf("No results found for: %s", msg.query)
			return m, clearStatusAfter(5 * time.Second)
		}
		return m, nil

	case attachmentSavedMsg:
		m.statusMessage = fmt.Sprintf("Downloaded: %s", msg.path)
		return m, clearStatusAfter(3 * time.Second)

	case errMsg:
		m.statusMessage = "Error: " + msg.err.Error()
		if m.state == stateLoading {
			m.state = stateInbox
		}
		return m, clearStatusAfter(5 * time.Second)

	case statusMsg:
		m.statusMessage = msg.message
		return m, clearStatusAfter(3 * time.Second)

	case clearStatusMsg:
		m.statusMessage = ""
		return m, nil
	}

	return m.updateSubComponents(msg)
}

func (m Model) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	w := msg.Width
	h := msg.Height
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	m.width = w
	m.height = h
	m.help.Width = w

	// Set sizes for all components so they are ready when state changes
	m.emailList.SetSize(w, h-3)
	m.labelList.SetSize(w, h-3)

	vpHeight := h - 7
	if vpHeight < 1 {
		vpHeight = 1
	}
	m.viewport.Width = w
	m.viewport.Height = vpHeight

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.showHelp && key.Matches(msg, keys.ShowHelp) {
		m.showHelp = true
		return m, nil
	}
	if m.showHelp && key.Matches(msg, keys.CloseHelp) {
		m.showHelp = false
		return m, nil
	}

	if m.confirmDelete {
		return m.handleDeleteConfirm(msg)
	}

	if m.state == stateViewing && m.downloading {
		return m.handleAttachmentPick(msg)
	}

	switch m.state {
	case stateLoading:
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
	case stateInbox:
		return m.updateInbox(msg)
	case stateViewing:
		return m.updateViewing(msg)
	case stateComposing:
		return m.updateComposing(msg)
	case stateReplying:
		return m.updateReplying(msg)
	case stateSearching:
		return m.updateSearching(msg)
	case stateLabels:
		return m.updateLabels(msg)
	}
	return m, nil
}

// ---------- state handlers ----------

func (m Model) updateInbox(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Compose):
		if !m.client.HasSendScope() {
			return m, notify("Compose disabled: Gmail send scope not granted")
		}
		m.state = stateComposing
		m.isReply = false
		m.confirmNoSubject = false
		m.compose.reset(m.userEmail)
		return m, m.compose.focusField()

	case key.Matches(msg, keys.Search):
		m.state = stateSearching
		m.searchInput.SetValue(m.lastQuery)
		m.searchInput.Focus()
		return m, nil

	case key.Matches(msg, keys.Labels):
		m.state = stateLoading
		return m, tea.Batch(m.spinner.Tick, fetchLabelsCmd(m.client))

	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Select):
		if ei, ok := m.emailList.SelectedItem().(emailItem); ok {
			m.currentEmail = &ei.email
			m.state = stateLoading
			return m, tea.Batch(m.spinner.Tick, openEmail(m.client, ei.email.ID))
		}

	case key.Matches(msg, keys.Delete):
		if ei, ok := m.emailList.SelectedItem().(emailItem); ok {
			m.confirmDelete = true
			m.deleteTargetID = ei.email.ID
			m.deleteSubject = ei.email.Subject
			m.statusMessage = fmt.Sprintf("Delete \"%s\"? [y/n]", truncate(ei.email.Subject, 40))
			return m, nil
		}

	case key.Matches(msg, keys.ToggleRead):
		if ei, ok := m.emailList.SelectedItem().(emailItem); ok {
			return m, toggleReadCmd(m.client, ei.email.ID, ei.email.IsUnread)
		}

	case key.Matches(msg, keys.Archive):
		if ei, ok := m.emailList.SelectedItem().(emailItem); ok {
			return m, archiveEmailCmd(m.client, ei.email.ID)
		}

	case key.Matches(msg, keys.Refresh):
		m.state = stateLoading
		return m, tea.Batch(m.spinner.Tick, fetchInbox(m.client, int64(m.config.MaxResults)))

	case key.Matches(msg, keys.NextPage):
		if m.pageToken == "" {
			m.statusMessage = "No more emails"
			return m, clearStatusAfter(2 * time.Second)
		}
		m.pageHistory = append(m.pageHistory, m.currentToken)
		m.currentToken = m.pageToken
		m.currentPage++
		m.state = stateLoading
		return m, tea.Batch(m.spinner.Tick, fetchNextPageCmd(m.client, inboxQuery, int64(m.config.MaxResults), m.pageToken))

	case key.Matches(msg, keys.PrevPage):
		if len(m.pageHistory) == 0 {
			return m, nil
		}
		last := m.pageHistory[len(m.pageHistory)-1]
		m.pageHistory = m.pageHistory[:len(m.pageHistory)-1]
		m.currentToken = last
		m.currentPage--
		m.state = stateLoading
		return m, tea.Batch(m.spinner.Tick, fetchNextPageCmd(m.client, inboxQuery, int64(m.config.MaxResults), last))
	}

	var cmd tea.Cmd
	m.emailList, cmd = m.emailList.Update(msg)
	return m, cmd
}

func (m Model) updateViewing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.state = stateInbox
		m.viewport.GotoTop()
		return m, nil

	case key.Matches(msg, keys.Reply):
		if !m.client.HasSendScope() {
			return m, notify("Reply disabled: Gmail send scope not granted")
		}
		m.state = stateReplying
		m.replyTo = m.currentEmail
		m.replyBody.Reset()
		m.replyAttachments = nil
		m.addingAttach = false
		m.replyBody.Focus()
		return m, nil

	case key.Matches(msg, keys.Delete):
		m.confirmDelete = true
		m.deleteTargetID = m.currentEmail.ID
		m.deleteSubject = m.currentEmail.Subject
		m.statusMessage = fmt.Sprintf("Delete \"%s\"? [y/n]", truncate(m.currentEmail.Subject, 40))
		return m, nil

	case key.Matches(msg, keys.ToggleRead):
		return m, toggleReadCmd(m.client, m.currentEmail.ID, m.currentEmail.IsUnread)

	case key.Matches(msg, keys.Archive):
		return m, archiveEmailCmd(m.client, m.currentEmail.ID)

	case key.Matches(msg, keys.Labels):
		m.state = stateLoading
		return m, tea.Batch(m.spinner.Tick, fetchLabelsCmd(m.client))

	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.DownloadAttachment):
		if m.currentEmail == nil || len(m.currentEmail.Attachments) == 0 {
			return m, notify("No attachments available")
		}
		m.downloading = true
		m.statusMessage = fmt.Sprintf("Select attachment (1-%d) [esc] cancel", len(m.currentEmail.Attachments))
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) updateComposing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle discard confirmation first
	if m.confirmDiscard {
		switch msg.String() {
		case "y", "Y":
			m.confirmDiscard = false
			m.state = stateInbox
			return m, nil
		case "n", "N":
			m.confirmDiscard = false
			m.statusMessage = ""
			return m, nil
		}
		return m, nil
	}

	if m.confirmNoSubject {
		switch msg.String() {
		case "y", "Y":
			m.confirmNoSubject = false
			m.statusMessage = ""
			return m, m.composeSendCmd()
		case "n", "N":
			m.confirmNoSubject = false
			m.statusMessage = ""
			return m, nil
		}
		return m, nil
	}

	switch {
	case msg.Type == tea.KeyEsc:
		if m.compose.addingAttach {
			m.compose.addingAttach = false
			m.compose.attachInput.Reset()
			return m, m.compose.focusField()
		}
		// Check if any field has content before discarding
		hasContent := m.compose.to.Value() != "" ||
			m.compose.subject.Value() != "" ||
			m.compose.body.Value() != "" ||
			len(m.compose.attachments) > 0
		if hasContent {
			m.confirmDiscard = true
			m.statusMessage = "Discard draft? [y/n]"
			return m, nil
		}
		m.state = stateInbox
		return m, nil

	case key.Matches(msg, keys.Send):
		if err := validateComposeRecipients(
			m.compose.to.Value(),
			m.compose.cc.Value(),
			m.compose.bcc.Value(),
		); err != nil {
			return m, errorCmd(err)
		}
		if strings.TrimSpace(m.compose.subject.Value()) == "" {
			m.confirmNoSubject = true
			m.statusMessage = "Send with no subject? [y/n]"
			return m, nil
		}
		return m, m.composeSendCmd()

	case key.Matches(msg, keys.AddAttachment):
		if !m.compose.addingAttach {
			m.compose.addingAttach = true
			m.compose.attachInput.SetValue("")
			m.compose.attachInput.Focus()
		}
		return m, nil

	case key.Matches(msg, keys.RemoveAttachment):
		if !m.compose.addingAttach && len(m.compose.attachments) > 0 {
			m.compose.attachments = m.compose.attachments[:len(m.compose.attachments)-1]
			return m, notify("Removed last attachment")
		}

	case msg.Type == tea.KeyEnter && m.compose.addingAttach:
		return m.handleComposeAttach()

	case key.Matches(msg, keys.NextInput):
		if !m.compose.addingAttach {
			m.compose.focused = (m.compose.focused + 1) % 6
			return m, m.compose.focusField()
		}

	case key.Matches(msg, keys.PrevInput):
		if !m.compose.addingAttach {
			m.compose.focused = (m.compose.focused - 1 + 6) % 6
			return m, m.compose.focusField()
		}
	}

	return m.updateComposeInputs(msg)
}

func (m Model) updateReplying(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		if m.addingAttach {
			m.addingAttach = false
			m.attachInput.Reset()
			return m, nil
		}
		m.state = stateViewing
		return m, nil

	case key.Matches(msg, keys.Send):
		if m.replyTo == nil {
			return m, errorCmd(fmt.Errorf("reply target is missing"))
		}
		if invalid, ok := api.ValidateEmailAddresses(m.replyTo.From); !ok {
			return m, errorCmd(fmt.Errorf("invalid email address: %s", invalid))
		}
		return m, m.replySendCmd()

	case key.Matches(msg, keys.AddAttachment):
		m.addingAttach = true
		m.attachInput.SetValue("")
		m.attachInput.Focus()
		return m, nil

	case key.Matches(msg, keys.RemoveAttachment):
		if len(m.replyAttachments) > 0 {
			m.replyAttachments = m.replyAttachments[:len(m.replyAttachments)-1]
			return m, notify("Removed last attachment")
		}

	case msg.Type == tea.KeyEnter && m.addingAttach:
		path := strings.TrimSpace(m.attachInput.Value())
		if path == "" {
			return m, nil
		}
		if err := validateAttachment(path); err != nil {
			return m, errorCmd(err)
		}
		m.replyAttachments = append(m.replyAttachments, path)
		m.addingAttach = false
		m.attachInput.Reset()
		return m, notify(fmt.Sprintf("Added: %s", filepath.Base(path)))
	}

	var cmd tea.Cmd
	if m.addingAttach {
		m.attachInput, cmd = m.attachInput.Update(msg)
	} else {
		m.replyBody, cmd = m.replyBody.Update(msg)
	}
	return m, cmd
}

func (m Model) updateSearching(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		m.state = stateInbox
		return m, nil

	case msg.Type == tea.KeyEnter:
		q := strings.TrimSpace(m.searchInput.Value())
		if q == "" {
			return m, nil
		}
		m.lastQuery = q
		m.state = stateLoading
		return m, tea.Batch(m.spinner.Tick, searchCmd(m.client, q, int64(m.config.SearchMaxResults)))
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m Model) updateLabels(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.state = stateInbox
		return m, nil

	case key.Matches(msg, keys.Select):
		if li, ok := m.labelList.SelectedItem().(labelItem); ok {
			m.state = stateLoading
			return m, tea.Batch(m.spinner.Tick, fetchByLabelCmd(m.client, li.label.ID, li.label.Name, int64(m.config.MaxResults)))
		}
	}

	var cmd tea.Cmd
	m.labelList, cmd = m.labelList.Update(msg)
	return m, cmd
}

// ---------- sub-handlers ----------

// removeListedEmail drops an email from the list (after delete/archive) and
// returns to the inbox if that email was being viewed.
func (m Model) removeListedEmail(id string) Model {
	for i, item := range m.emailList.Items() {
		if ei, ok := item.(emailItem); ok && ei.email.ID == id {
			m.emailList.RemoveItem(i)
			if m.totalFetched > 0 {
				m.totalFetched--
			}
			break
		}
	}
	if m.state == stateViewing {
		m.state = stateInbox
	}
	return m
}

func (m Model) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.confirmDelete = false
		id := m.deleteTargetID
		m.deleteTargetID = ""
		return m, deleteEmailCmd(m.client, id)
	default:
		m.confirmDelete = false
		m.deleteTargetID = ""
		m.statusMessage = "Delete cancelled"
		return m, clearStatusAfter(2 * time.Second)
	}
}

func (m Model) handleAttachmentPick(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		m.downloading = false
		m.statusMessage = ""
		return m, nil
	}
	if msg.Type == tea.KeyRunes {
		digit, err := strconv.Atoi(string(msg.Runes))
		if err == nil && digit > 0 && digit <= len(m.currentEmail.Attachments) {
			m.downloading = false
			att := m.currentEmail.Attachments[digit-1]
			m.statusMessage = fmt.Sprintf("Downloading %s...", att.Filename)
			return m, downloadAttachmentCmd(m.client, m.currentEmail.ID, att)
		}
	}
	return m, nil
}

func (m Model) handleComposeAttach() (tea.Model, tea.Cmd) {
	path := strings.TrimSpace(m.compose.attachInput.Value())
	if path == "" {
		return m, nil
	}
	if err := validateAttachment(path); err != nil {
		return m, errorCmd(err)
	}
	m.compose.attachments = append(m.compose.attachments, path)
	m.compose.addingAttach = false
	m.compose.attachInput.Reset()
	return m, tea.Batch(
		m.compose.focusField(),
		notify(fmt.Sprintf("Added: %s", filepath.Base(path))),
	)
}

func (m Model) updateComposeInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.compose.addingAttach {
		m.compose.attachInput, cmd = m.compose.attachInput.Update(msg)
		return m, cmd
	}
	switch m.compose.focused {
	case 0:
		m.compose.from, cmd = m.compose.from.Update(msg)
	case 1:
		m.compose.to, cmd = m.compose.to.Update(msg)
	case 2:
		m.compose.cc, cmd = m.compose.cc.Update(msg)
	case 3:
		m.compose.bcc, cmd = m.compose.bcc.Update(msg)
	case 4:
		m.compose.subject, cmd = m.compose.subject.Update(msg)
	case 5:
		m.compose.body, cmd = m.compose.body.Update(msg)
	}
	return m, cmd
}

func (m Model) updateSubComponents(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.state {
	case stateLoading:
		m.spinner, cmd = m.spinner.Update(msg)
	case stateViewing:
		m.viewport, cmd = m.viewport.Update(msg)
	case stateReplying:
		if m.addingAttach {
			m.attachInput, cmd = m.attachInput.Update(msg)
		} else {
			m.replyBody, cmd = m.replyBody.Update(msg)
		}
	case stateSearching:
		m.searchInput, cmd = m.searchInput.Update(msg)
	case stateLabels:
		m.labelList, cmd = m.labelList.Update(msg)
	}
	return m, cmd
}

// truncate shortens s to at most max characters (runes), appending "..." when
// truncated. Rune-based so multi-byte characters are never split.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func (m Model) composeSendCmd() tea.Cmd {
	return sendEmailCmd(
		m.client,
		m.compose.to.Value(),
		m.compose.cc.Value(),
		m.compose.bcc.Value(),
		m.compose.subject.Value(),
		m.compose.body.Value(),
		m.compose.attachments,
	)
}

func (m Model) replySendCmd() tea.Cmd {
	quoted := fmt.Sprintf(
		"\n\n--- Original Message ---\nFrom: %s\nDate: %s\n\n%s",
		m.replyTo.From,
		m.replyTo.Date,
		api.IndentText(m.currentEmail.Body),
	)
	// RFC 5322: References of a reply = original References + original Message-ID.
	references := strings.TrimSpace(m.replyTo.References + " " + m.replyTo.MessageID)
	return sendReplyCmd(
		m.client,
		m.replyTo.From,
		formatReplySubject(m.replyTo.Subject),
		m.replyBody.Value()+quoted,
		m.replyTo.MessageID,
		references,
		m.replyAttachments,
	)
}

func validateComposeRecipients(to, cc, bcc string) error {
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("recipient address is required")
	}
	for _, field := range []string{to, cc, bcc} {
		if invalid, ok := api.ValidateEmailAddresses(field); !ok {
			return fmt.Errorf("invalid email address: %s", invalid)
		}
	}
	return nil
}

func validateAttachment(path string) error {
	info, err := api.ValidateAttachmentPath(path)
	if err != nil {
		return err
	}
	return api.ValidateAttachmentSize(info)
}

func formatReplySubject(subject string) string {
	trimmed := strings.TrimSpace(subject)
	if strings.HasPrefix(strings.ToLower(trimmed), "re:") {
		return "Re: " + strings.TrimSpace(trimmed[3:])
	}
	return "Re: " + trimmed
}

func errorCmd(err error) tea.Cmd {
	return func() tea.Msg {
		return errMsg{err: err}
	}
}
