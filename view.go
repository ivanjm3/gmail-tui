package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if m.showHelp {
		return m.help.View(keys)
	}

	switch m.state {
	case stateInbox:
		return m.inboxView()
	case stateViewing:
		return m.emailView()
	case stateLoading:
		return m.loadingView()
	case stateComposing:
		return m.composeView()
	case stateReplying:
		return m.replyView()
	case stateSearching:
		return m.searchView()
	case stateManagingLabels:
		return m.labelsView()
	}
	return ""
}

func (m model) inboxView() string {
	help := "\n[c] compose • [r] reply • [d] delete • [m] mark read/unread • [l] labels • [/] search • [?] help • [q] quit\n"
	return m.list.View() + help
}

func (m model) emailView() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\nFrom: %s\n", m.currentMsg.from))
	b.WriteString(fmt.Sprintf("To: %s\n", m.currentMsg.recipient))

	if m.currentMsg.cc != "" {
		b.WriteString(fmt.Sprintf("CC: %s\n", m.currentMsg.cc))
	}
	if m.currentMsg.bcc != "" {
		b.WriteString(fmt.Sprintf("BCC: %s\n", m.currentMsg.bcc))
	}

	b.WriteString(fmt.Sprintf("Subject: %s\n", m.currentMsg.subject))
	b.WriteString(fmt.Sprintf("Date: %s\n\n", m.currentMsg.date))
	b.WriteString(m.viewport.View() + "\n\n")

	if m.attachmentDownloading {
		b.WriteString("\nDownload which attachment? (1-9) [esc] cancel\n")
		for i, att := range m.currentMsg.attachments {
			if i < 9 {
				b.WriteString(fmt.Sprintf("  [%d] %s (%s)\n", i+1, att.Filename, humanSize(att.Body.Size)))
			}
		}
	} else if len(m.currentMsg.attachments) > 0 {
		b.WriteString("\nAttachments:\n")
		for i, att := range m.currentMsg.attachments {
			b.WriteString(fmt.Sprintf("  [%d] %s (%s)\n", i+1, att.Filename, humanSize(att.Body.Size)))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n[b] back • [r] reply • [d] delete • [m] mark read/unread • [ctrl+d] download attachment • [q] quit\n")
	return b.String()
}

func (m model) loadingView() string {
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Center,
			m.loading.View(),
			"Loading...",
		),
	)
}

func (m model) composeView() string {
	var b strings.Builder

	b.WriteString("\n  Compose New Email\n\n")
	b.WriteString(fmt.Sprintf("  From: %s\n", m.composeFrom.View()))
	b.WriteString(fmt.Sprintf("  To:   %s\n", m.composeTo.View()))
	b.WriteString(fmt.Sprintf("  CC:   %s\n", m.composeCc.View()))
	b.WriteString(fmt.Sprintf("  BCC:  %s\n", m.composeBcc.View()))
	b.WriteString(fmt.Sprintf("  Subj: %s\n\n", m.composeSubj.View()))
	b.WriteString("  Body:\n" + m.composeBody.View() + "\n")

	if len(m.composeAttachments) > 0 {
		b.WriteString("\nAttachments:\n")
		for i, f := range m.composeAttachments {
			b.WriteString(fmt.Sprintf("  [%d] %s\n", i+1, filepath.Base(f)))
		}
	}

	if m.addingAttachment {
		b.WriteString("\nAttachment Path: " + m.attachmentInput.View())
	}

	b.WriteString("\n[ctrl+s] send • [ctrl+a] add attachment • [ctrl+x] remove attachment • [esc] back")
	return b.String()
}

func (m model) replyView() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\n  Reply to: %s\n", m.replyToMsg.from))
	b.WriteString(fmt.Sprintf("  Subject: Re: %s\n\n", m.replyToMsg.subject))
	b.WriteString(m.replyBody.View() + "\n")

	if len(m.replyAttachments) > 0 {
		b.WriteString("\nAttachments:\n")
		for i, f := range m.replyAttachments {
			b.WriteString(fmt.Sprintf("  [%d] %s\n", i+1, filepath.Base(f)))
		}
	}

	if m.addingAttachment {
		b.WriteString("\nAttachment Path: " + m.attachmentInput.View())
	}

	b.WriteString("\n[ctrl+s] send • [ctrl+a] add attachment • [ctrl+x] remove attachment • [esc] back")
	return b.String()
}

func (m model) searchView() string {
	return "\n  Search: " + m.searchInput.View() + "\n\n[enter] search • [esc] cancel\n"
}

func (m model) labelsView() string {
	help := "\n[↑/↓] navigate • [enter] select • [b] back\n"
	return m.labelsList.View() + help
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
