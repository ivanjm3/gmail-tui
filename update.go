package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case emailLoadedMsg:
		return m.handleEmailLoaded(msg)
	case emailSentMsg:
		return m.handleEmailSent()
	case labelsLoadedMsg:
		return m.handleLabelsLoaded(msg)
	case searchResultMsg:
		return m.handleSearchResult(msg)
	case attachmentDownloadedMsg:
		return m, showNotification(fmt.Sprintf("Downloaded: %s", msg.filename))
	}

	return m.updateComponents(msg)
}

func (m model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.help.Width = msg.Width

	if m.state == stateInbox {
		m.list.SetSize(msg.Width, msg.Height-3)
	} else if m.state == stateViewing {
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 7
	}
	return m, nil
}

func (m model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle help toggle
	if !m.showHelp && key.Matches(msg, keys.ShowHelp) {
		m.showHelp = true
		return m, nil
	}
	if m.showHelp && key.Matches(msg, keys.CloseHelp) {
		m.showHelp = false
		return m, nil
	}

	// Handle attachment downloading state
	if m.state == stateViewing && m.attachmentDownloading {
		return m.handleAttachmentDownload(msg)
	}

	// Route to state-specific handlers
	switch m.state {
	case stateInbox:
		return updateInbox(msg, m)
	case stateViewing:
		return updateViewing(msg, m)
	case stateComposing:
		return updateComposing(msg, m)
	case stateReplying:
		return updateReplying(msg, m)
	case stateSearching:
		return updateSearching(msg, m)
	case stateManagingLabels:
		return updateLabelManagement(msg, m)
	}

	return m, nil
}

func (m model) handleAttachmentDownload(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Back) {
		m.attachmentDownloading = false
		return m, nil
	}

	if msg.Type == tea.KeyRunes {
		digit, err := strconv.Atoi(string(msg.Runes))
		if err == nil && digit > 0 && digit <= len(m.currentMsg.attachments) {
			m.attachmentDownloading = false
			attachment := m.currentMsg.attachments[digit-1]
			return m, tea.Batch(
				showNotification(fmt.Sprintf("Downloading %s...", attachment.Filename)),
				downloadAttachment(m.srv, m.currentMsg.id, attachment),
			)
		}
	}
	return m, nil
}

func (m model) handleEmailLoaded(msg emailLoadedMsg) (tea.Model, tea.Cmd) {
	m.state = stateViewing
	m.fullEmail = msg.content
	m.viewport.Width = m.width
	m.viewport.Height = m.height - 7
	m.viewport.SetContent(m.fullEmail)
	return m, nil
}

func (m model) handleEmailSent() (tea.Model, tea.Cmd) {
	m.state = stateInbox
	m.viewport.GotoTop()
	return m, showNotification("Email sent successfully!")
}

func (m model) handleLabelsLoaded(msg labelsLoadedMsg) (tea.Model, tea.Cmd) {
	items := make([]list.Item, len(msg.labels))
	for i, label := range msg.labels {
		items[i] = labelItem{label: label}
	}
	m.labels = msg.labels
	m.labelsList.SetItems(items)
	m.state = stateManagingLabels
	return m, nil
}

func (m model) handleSearchResult(msg searchResultMsg) (tea.Model, tea.Cmd) {
	items := make([]list.Item, 0, len(msg.messages))
	for _, message := range msg.messages {
		if item := createEmailItem(m.srv, message.Id, true); item != nil {
			items = append(items, *item)
		}
	}
	m.list.SetItems(items)
	m.state = stateInbox
	return m, nil
}

func (m model) updateComponents(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch m.state {
	case stateLoading:
		m.loading, cmd = m.loading.Update(msg)
		cmds = append(cmds, cmd)
	case stateViewing:
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	case stateReplying:
		m.replyBody, cmd = m.replyBody.Update(msg)
		cmds = append(cmds, cmd)
	case stateSearching:
		m.searchInput, cmd = m.searchInput.Update(msg)
		cmds = append(cmds, cmd)
	case stateManagingLabels:
		m.labelsList, cmd = m.labelsList.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func updateInbox(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Compose):
		m.state = stateComposing
		m.composeFrom.SetValue("me")
		return m, nil

	case key.Matches(msg, keys.Search):
		m.state = stateSearching
		m.searchInput.Focus()
		return m, nil

	case key.Matches(msg, keys.Labels):
		return m, loadLabels(m.srv)

	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Select):
		selected, ok := m.list.SelectedItem().(emailItem)
		if ok {
			m.currentMsg = &selected
			m.state = stateLoading
			return m, tea.Batch(m.loading.Tick, loadEmail(m.srv, selected.id))
		}

	case key.Matches(msg, keys.Delete):
		if selected, ok := m.list.SelectedItem().(emailItem); ok {
			return m, deleteEmail(m.srv, selected.id)
		}

	case key.Matches(msg, keys.ToggleRead):
		if selected, ok := m.list.SelectedItem().(emailItem); ok {
			return m, toggleReadStatus(m.srv, selected.id, selected.isUnread)
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func updateViewing(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.state = stateInbox
		m.viewport.GotoTop()
		return m, nil

	case key.Matches(msg, keys.Reply):
		m.state = stateReplying
		m.replyToMsg = m.currentMsg
		m.replyBody.Focus()
		return m, nil

	case key.Matches(msg, keys.Delete):
		return m, deleteEmail(m.srv, m.currentMsg.id)

	case key.Matches(msg, keys.ToggleRead):
		return m, toggleReadStatus(m.srv, m.currentMsg.id, m.currentMsg.isUnread)

	case key.Matches(msg, keys.Labels):
		return m, loadLabels(m.srv)

	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.DownloadAttachment):
		if len(m.currentMsg.attachments) == 0 {
			return m, showNotification("No attachments available")
		}
		m.attachmentDownloading = true
		return m, showNotification(fmt.Sprintf(
			"Select attachment to download (1-%d) [esc] cancel",
			len(m.currentMsg.attachments),
		))
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func updateComposing(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		if m.addingAttachment {
			m.addingAttachment = false
			m.attachmentInput.Reset()
			return m, m.focusComposeField()
		}
		m.state = stateInbox
		return m, nil

	case key.Matches(msg, keys.Send):
		return m, sendEmail(
			m.srv,
			m.composeTo.Value(),
			m.composeCc.Value(),
			m.composeBcc.Value(),
			m.composeSubj.Value(),
			m.composeBody.Value(),
			m.composeAttachments,
		)

	case key.Matches(msg, keys.AddAttachment):
		if !m.addingAttachment {
			m.addingAttachment = true
			m.attachmentInput.Focus()
		}
		return m, nil

	case key.Matches(msg, keys.RemoveAttachment):
		if !m.addingAttachment && len(m.composeAttachments) > 0 {
			m.composeAttachments = m.composeAttachments[:len(m.composeAttachments)-1]
			return m, showNotification("Removed last attachment")
		}

	case msg.Type == tea.KeyEnter && m.addingAttachment:
		return m.handleAttachmentAdd()

	case key.Matches(msg, keys.NextInput):
		if !m.addingAttachment {
			m.focused = (m.focused + 1) % 6
			return m, m.focusComposeField()
		}

	case key.Matches(msg, keys.PrevInput):
		if !m.addingAttachment {
			m.focused = (m.focused - 1 + 6) % 6
			return m, m.focusComposeField()
		}
	}

	return m.updateComposeFields(msg)
}

func (m model) handleAttachmentAdd() (tea.Model, tea.Cmd) {
	path := strings.TrimSpace(m.attachmentInput.Value())
	if path == "" {
		return m, nil
	}

	if _, err := os.Stat(path); err == nil {
		m.composeAttachments = append(m.composeAttachments, path)
		m.addingAttachment = false
		m.attachmentInput.Reset()
		return m, tea.Batch(
			m.focusComposeField(),
			showNotification(fmt.Sprintf("Added: %s", filepath.Base(path))),
		)
	}

	return m, showNotification(fmt.Sprintf("File not found: %s", path))
}

func (m model) updateComposeFields(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.addingAttachment {
		m.attachmentInput, cmd = m.attachmentInput.Update(msg)
		return m, cmd
	}

	switch m.focused {
	case 0:
		m.composeFrom, cmd = m.composeFrom.Update(msg)
	case 1:
		m.composeTo, cmd = m.composeTo.Update(msg)
	case 2:
		m.composeCc, cmd = m.composeCc.Update(msg)
	case 3:
		m.composeBcc, cmd = m.composeBcc.Update(msg)
	case 4:
		m.composeSubj, cmd = m.composeSubj.Update(msg)
	case 5:
		m.composeBody, cmd = m.composeBody.Update(msg)
	}
	return m, cmd
}

func updateReplying(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.state = stateViewing
		m.addingAttachment = false
		return m, nil

	case key.Matches(msg, keys.Send):
		quoted := fmt.Sprintf(
			"\n\n--- Original Message ---\nFrom: %s\nDate: %s\n\n%s",
			m.replyToMsg.from,
			m.replyToMsg.date,
			indentText(m.currentMsg.body),
		)
		fullBody := m.replyBody.Value() + quoted
		return m, sendEmail(
			m.srv,
			m.replyToMsg.from,
			"",
			"",
			"Re: "+m.replyToMsg.subject,
			fullBody,
			m.replyAttachments,
		)

	case key.Matches(msg, keys.AddAttachment):
		m.addingAttachment = true
		m.attachmentInput.Focus()
		return m, nil

	case key.Matches(msg, keys.RemoveAttachment):
		if len(m.replyAttachments) > 0 {
			m.replyAttachments = m.replyAttachments[:len(m.replyAttachments)-1]
		}
		return m, nil

	case msg.Type == tea.KeyEnter && m.addingAttachment:
		path := strings.TrimSpace(m.attachmentInput.Value())
		if path != "" {
			if _, err := os.Stat(path); err == nil {
				m.replyAttachments = append(m.replyAttachments, path)
				m.addingAttachment = false
				m.attachmentInput.Reset()
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	if m.addingAttachment {
		m.attachmentInput, cmd = m.attachmentInput.Update(msg)
	} else {
		m.replyBody, cmd = m.replyBody.Update(msg)
	}
	return m, cmd
}

func updateSearching(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.state = stateInbox
		return m, nil

	case msg.Type == tea.KeyEnter:
		m.state = stateLoading
		m.searchQuery = m.searchInput.Value()
		return m, tea.Batch(m.loading.Tick, performSearch(m.srv, m.searchQuery))
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func updateLabelManagement(msg tea.KeyMsg, m model) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		m.state = stateInbox
		return m, nil

	case key.Matches(msg, keys.Select):
		if selected, ok := m.labelsList.SelectedItem().(labelItem); ok {
			m.state = stateLoading
			return m, tea.Batch(m.loading.Tick, loadEmailsByLabel(m.srv, selected.label.Id))
		}
	}

	var cmd tea.Cmd
	m.labelsList, cmd = m.labelsList.Update(msg)
	return m, cmd
}
