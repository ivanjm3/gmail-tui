package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"google.golang.org/api/gmail/v1"
)

// createEmailItem fetches and converts a Gmail message to an emailItem
func createEmailItem(srv *gmail.Service, msgID string, minimal bool) *emailItem {
	if srv == nil {
		log.Println("Gmail service is not initialized")
		return nil
	}

	format := "full"
	if minimal {
		format = "minimal"
	}

	msg, err := srv.Users.Messages.Get("me", msgID).Format(format).Do()
	if err != nil {
		log.Printf("Error fetching message %s: %v\n", msgID, err)
		return nil
	}

	if msg == nil {
		log.Printf("Received nil message for ID %s\n", msgID)
		return nil
	}

	item := &emailItem{
		id:       msg.Id,
		threadId: msg.ThreadId,
		snippet:  msg.Snippet,
	}

	// Extract headers
	if msg.Payload != nil {
		for _, h := range msg.Payload.Headers {
			switch h.Name {
			case "Subject":
				item.subject = h.Value
			case "From":
				item.from = h.Value
			case "Date":
				item.date = formatDate(h.Value)
			case "To":
				item.recipient = h.Value
			case "Cc":
				item.cc = h.Value
			case "Bcc":
				item.bcc = h.Value
			}
		}

		// Extract body and attachments for full format
		if !minimal {
			item.body = extractPlainText(msg.Payload)
			item.attachments = findAttachments(msg.Payload)
		}
	}

	// Process labels
	for _, labelID := range msg.LabelIds {
		if labelID == "UNREAD" {
			item.isUnread = true
		}
		item.labels = append(item.labels, labelID)
	}

	// Truncate snippet if needed
	if len(item.snippet) > 80 {
		item.snippet = item.snippet[:77] + "..."
	}

	return item
}

// findAttachments recursively finds all attachments in a message part
func findAttachments(part *gmail.MessagePart) []*gmail.MessagePart {
	var attachments []*gmail.MessagePart

	if strings.HasPrefix(part.MimeType, "multipart/") {
		for _, p := range part.Parts {
			attachments = append(attachments, findAttachments(p)...)
		}
	} else if part.Filename != "" {
		attachments = append(attachments, part)
	}

	return attachments
}

// fetchFullEmailBody retrieves the complete email content for viewing
func fetchFullEmailBody(srv *gmail.Service, msgID string) (string, error) {
	msg, err := srv.Users.Messages.Get("me", msgID).Format("full").Do()
	if err != nil {
		return "", fmt.Errorf("failed to fetch message: %w", err)
	}

	var from, subject, date string
	for _, h := range msg.Payload.Headers {
		switch h.Name {
		case "From":
			from = h.Value
		case "Subject":
			subject = h.Value
		case "Date":
			date = formatDate(h.Value)
		}
	}

	body := extractPlainText(msg.Payload)
	if body == "" {
		body = "(no text content found)"
	}

	return fmt.Sprintf("From: %s\nSubject: %s\nDate: %s\n\n%s",
		from, subject, date, body), nil
}

// extractPlainText recursively extracts plain text from a message part
func extractPlainText(payload *gmail.MessagePart) string {
	if payload.MimeType == "text/plain" && payload.Body != nil && payload.Body.Data != "" {
		return decodeBody(payload.Body.Data)
	}

	// Handle multipart messages
	if strings.HasPrefix(payload.MimeType, "multipart/") && len(payload.Parts) > 0 {
		// First try to find plain text
		for _, p := range payload.Parts {
			if p.MimeType == "text/plain" {
				if text := extractPlainText(p); text != "" {
					return text
				}
			}
		}

		// Fall back to HTML if no plain text found
		for _, p := range payload.Parts {
			if p.MimeType == "text/html" {
				if text := extractPlainText(p); text != "" {
					return text
				}
			}
		}
	}

	// Handle HTML content
	if payload.MimeType == "text/html" && payload.Body != nil && payload.Body.Data != "" {
		htmlContent := decodeBody(payload.Body.Data)
		return stripHTML(htmlContent)
	}

	return ""
}

// decodeBody decodes base64-encoded email body
func decodeBody(body string) string {
	// Add padding if needed
	if len(body)%4 != 0 {
		body += strings.Repeat("=", (4-len(body)%4)%4)
	}

	decoded, err := base64.URLEncoding.DecodeString(body)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(body)
		if err != nil {
			return "Failed to decode body."
		}
	}
	return string(decoded)
}

// stripHTML removes HTML tags and decodes entities
func stripHTML(input string) string {
	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	input = re.ReplaceAllString(input, "")

	// Decode HTML entities
	entities := map[string]string{
		"&nbsp;":  " ",
		"&lt;":    "<",
		"&gt;":    ">",
		"&amp;":   "&",
		"&quot;":  "\"",
		"&apos;":  "'",
		"&#39;":   "'",
		"&ndash;": "-",
		"&mdash;": "—",
	}

	for entity, replacement := range entities {
		input = strings.ReplaceAll(input, entity, replacement)
	}

	// Collapse whitespace
	input = regexp.MustCompile(`\s+`).ReplaceAllString(input, " ")
	return strings.TrimSpace(input)
}

// formatDate parses and formats email dates
func formatDate(dateStr string) string {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 -0700",
		time.RFC822Z,
		time.RFC822,
	}

	for _, format := range formats {
		t, err := time.Parse(format, dateStr)
		if err == nil {
			return t.Format("Jan 02, 2006 15:04")
		}
	}

	return dateStr
}

// indentText adds "> " prefix to each line for quoted text
func indentText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = "> " + line
	}
	return strings.Join(lines, "\n")
}
