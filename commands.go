package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"google.golang.org/api/gmail/v1"
)

const (
	maxAttachmentSize = 25 * 1024 * 1024 // 25MB Gmail limit
	downloadsDir      = "downloads"
)

func loadEmail(srv *gmail.Service, msgID string) tea.Cmd {
	return func() tea.Msg {
		content, err := fetchFullEmailBody(srv, msgID)
		if err != nil {
			return emailLoadErrorMsg{err: err}
		}
		return emailLoadedMsg{content: content}
	}
}

func loadEmailsByLabel(srv *gmail.Service, labelID string) tea.Cmd {
	return func() tea.Msg {
		msgs, err := srv.Users.Messages.List("me").LabelIds(labelID).MaxResults(10).Do()
		if err != nil {
			return emailLoadErrorMsg{err: err}
		}
		return searchResultMsg{messages: msgs.Messages}
	}
}

func sendEmail(srv *gmail.Service, to, cc, bcc, subject, body string, attachments []string) tea.Cmd {
	return func() tea.Msg {
		tmpFile, err := os.CreateTemp("", "gmail-msg-")
		if err != nil {
			return emailLoadErrorMsg{err: fmt.Errorf("failed to create temp file: %w", err)}
		}
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		writer := multipart.NewWriter(tmpFile)
		boundary := writer.Boundary()

		// Write headers
		headers := buildEmailHeaders(to, cc, bcc, subject, boundary)
		if _, err := tmpFile.WriteString(headers); err != nil {
			return emailLoadErrorMsg{err: fmt.Errorf("failed to write headers: %w", err)}
		}

		// Write text body
		textPart := fmt.Sprintf("--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", boundary, body)
		if _, err := tmpFile.WriteString(textPart); err != nil {
			return emailLoadErrorMsg{err: fmt.Errorf("failed to write body: %w", err)}
		}

		// Process attachments
		for _, filePath := range attachments {
			if err := addAttachment(writer, filePath); err != nil {
				return emailLoadErrorMsg{err: err}
			}
		}

		// Finalize
		if _, err := tmpFile.WriteString(fmt.Sprintf("\r\n--%s--\r\n", boundary)); err != nil {
			return emailLoadErrorMsg{err: fmt.Errorf("failed to write final boundary: %w", err)}
		}

		if err := writer.Close(); err != nil {
			return emailLoadErrorMsg{err: fmt.Errorf("failed to close writer: %w", err)}
		}

		// Read back and send
		if _, err := tmpFile.Seek(0, 0); err != nil {
			return emailLoadErrorMsg{err: fmt.Errorf("failed to seek temp file: %w", err)}
		}

		content, err := io.ReadAll(tmpFile)
		if err != nil {
			return emailLoadErrorMsg{err: fmt.Errorf("failed to read temp file: %w", err)}
		}

		raw := base64.URLEncoding.EncodeToString(content)
		_, err = srv.Users.Messages.Send("me", &gmail.Message{Raw: raw}).Do()
		if err != nil {
			return emailLoadErrorMsg{err: err}
		}

		return emailSentMsg{}
	}
}

func buildEmailHeaders(to, cc, bcc, subject, boundary string) string {
	headers := fmt.Sprintf("To: %s\r\n", to)
	if cc != "" {
		headers += fmt.Sprintf("Cc: %s\r\n", cc)
	}
	if bcc != "" {
		headers += fmt.Sprintf("Bcc: %s\r\n", bcc)
	}
	headers += fmt.Sprintf("Subject: %s\r\n", subject)
	headers += fmt.Sprintf("MIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%s\r\n\r\n", boundary)
	return headers
}

func addAttachment(writer *multipart.Writer, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open attachment: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if fileInfo.Size() > maxAttachmentSize {
		return fmt.Errorf("attachment too large: %s (max 25MB)", filepath.Base(filePath))
	}

	partHeader := textproto.MIMEHeader{}
	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	partHeader.Set("Content-Type", mimeType)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(filePath)))
	partHeader.Set("Content-Transfer-Encoding", "base64")

	partWriter, err := writer.CreatePart(partHeader)
	if err != nil {
		return fmt.Errorf("failed to create attachment part: %w", err)
	}

	encoder := base64.NewEncoder(base64.StdEncoding, partWriter)
	if _, err := io.Copy(encoder, file); err != nil {
		return fmt.Errorf("failed to write attachment: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return fmt.Errorf("failed to close encoder: %w", err)
	}

	return nil
}

func downloadAttachment(srv *gmail.Service, msgID string, attachment *gmail.MessagePart) tea.Cmd {
	return func() tea.Msg {
		att, err := srv.Users.Messages.Attachments.Get("me", msgID, attachment.Body.AttachmentId).Do()
		if err != nil {
			return notificationMsg{message: fmt.Sprintf("Download failed: %v", err)}
		}

		data, err := base64.RawURLEncoding.DecodeString(att.Data)
		if err != nil {
			return notificationMsg{message: fmt.Sprintf("Failed to decode: %v", err)}
		}

		if err := os.MkdirAll(downloadsDir, 0755); err != nil {
			return notificationMsg{message: fmt.Sprintf("Couldn't create downloads directory: %v", err)}
		}

		filename := filepath.Join(downloadsDir, sanitizeFilename(attachment.Filename))
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return notificationMsg{message: fmt.Sprintf("Save failed: %v", err)}
		}

		return attachmentDownloadedMsg{filename: filename}
	}
}

func deleteEmail(srv *gmail.Service, msgID string) tea.Cmd {
	return func() tea.Msg {
		_, err := srv.Users.Messages.Trash("me", msgID).Do()
		if err != nil {
			return emailLoadErrorMsg{err: err}
		}
		return notificationMsg{message: "Email moved to trash"}
	}
}

func toggleReadStatus(srv *gmail.Service, msgID string, isUnread bool) tea.Cmd {
	return func() tea.Msg {
		mod := gmail.ModifyMessageRequest{}
		if isUnread {
			mod.RemoveLabelIds = []string{"UNREAD"}
		} else {
			mod.AddLabelIds = []string{"UNREAD"}
		}

		_, err := srv.Users.Messages.Modify("me", msgID, &mod).Do()
		if err != nil {
			return emailLoadErrorMsg{err: err}
		}

		action := "marked as read"
		if !isUnread {
			action = "marked as unread"
		}
		return notificationMsg{message: "Email " + action}
	}
}

func performSearch(srv *gmail.Service, query string) tea.Cmd {
	return func() tea.Msg {
		msgs, err := srv.Users.Messages.List("me").Q(query).MaxResults(30).Do()
		if err != nil {
			return emailLoadErrorMsg{err: err}
		}
		return searchResultMsg{messages: msgs.Messages}
	}
}

func loadLabels(srv *gmail.Service) tea.Cmd {
	return func() tea.Msg {
		labels, err := srv.Users.Labels.List("me").Do()
		if err != nil {
			return emailLoadErrorMsg{err: err}
		}
		return labelsLoadedMsg{labels: labels.Labels}
	}
}

func showNotification(msg string) tea.Cmd {
	return func() tea.Msg {
		return notificationMsg{message: msg}
	}
}

func sanitizeFilename(name string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsLetter(r) || unicode.IsNumber(r) ||
			r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, name)
}
