package api

import (
	"html"
	"regexp"
	"strings"
	"time"

	"google.golang.org/api/gmail/v1"
)

var (
	htmlTagRe    = regexp.MustCompile(`<[^>]*>`)
	whitespaceRe = regexp.MustCompile(`\s+`)
)

// parseMessage converts a raw Gmail API message into a domain Email.
// If full is true, body and attachments are extracted.
func parseMessage(msg *gmail.Message, full bool) *Email {
	email := &Email{
		ID:       msg.Id,
		ThreadID: msg.ThreadId,
		Snippet:  msg.Snippet,
	}

	if msg.Payload != nil {
		for _, h := range msg.Payload.Headers {
			switch h.Name {
			case "Subject":
				email.Subject = h.Value
			case "From":
				email.From = h.Value
			case "Date":
				email.Date = FormatDate(h.Value)
			case "To":
				email.To = h.Value
			case "Cc":
				email.CC = h.Value
			case "Bcc":
				email.BCC = h.Value
			case "Message-ID":
				email.MessageID = h.Value
			case "References":
				email.References = h.Value
			case "In-Reply-To":
				email.InReplyTo = h.Value
			}
		}

		if full {
			email.Body = extractPlainText(msg.Payload)
			if email.Body == "" {
				email.Body = "(no text content found)"
			}
			email.Attachments = findAttachments(msg.Payload)
			email.FullLoaded = true
		}
	}

	for _, labelID := range msg.LabelIds {
		if labelID == "UNREAD" {
			email.IsUnread = true
		}
		email.Labels = append(email.Labels, labelID)
	}

	if len(email.Snippet) > 80 {
		email.Snippet = email.Snippet[:77] + "..."
	}

	return email
}

func findAttachments(part *gmail.MessagePart) []Attachment {
	var out []Attachment
	if strings.HasPrefix(part.MimeType, "multipart/") {
		for _, p := range part.Parts {
			out = append(out, findAttachments(p)...)
		}
	} else if part.Filename != "" {
		att := Attachment{
			Filename: part.Filename,
			MimeType: part.MimeType,
		}
		if part.Body != nil {
			att.ID = part.Body.AttachmentId
			att.Size = part.Body.Size
		}
		out = append(out, att)
	}
	return out
}

func extractPlainText(payload *gmail.MessagePart) string {
	return extractPlainTextDepth(payload, 0)
}

func extractPlainTextDepth(payload *gmail.MessagePart, depth int) string {
	if depth > 20 {
		return ""
	}
	if payload.MimeType == "text/plain" && payload.Body != nil && payload.Body.Data != "" {
		return decodeBody(payload.Body.Data)
	}
	if strings.HasPrefix(payload.MimeType, "multipart/") && len(payload.Parts) > 0 {
		// First pass: plain text (recurse into nested multipart)
		for _, p := range payload.Parts {
			if p.MimeType == "text/plain" || strings.HasPrefix(p.MimeType, "multipart/") {
				if text := extractPlainTextDepth(p, depth+1); text != "" {
					return text
				}
			}
		}
		// Second pass: fall back to HTML
		for _, p := range payload.Parts {
			if p.MimeType == "text/html" || strings.HasPrefix(p.MimeType, "multipart/") {
				if text := extractPlainTextDepth(p, depth+1); text != "" {
					return text
				}
			}
		}
	}
	if payload.MimeType == "text/html" && payload.Body != nil && payload.Body.Data != "" {
		return stripHTML(decodeBody(payload.Body.Data))
	}
	return ""
}

func decodeBody(body string) string {
	decoded, err := decodeBase64Robust(body)
	if err != nil {
		return "(failed to decode body)"
	}
	return string(decoded)
}

func stripHTML(input string) string {
	input = htmlTagRe.ReplaceAllString(input, " ")
	input = html.UnescapeString(input)
	input = htmlTagRe.ReplaceAllString(input, " ")
	input = strings.NewReplacer("<", " ", ">", " ").Replace(input)
	input = whitespaceRe.ReplaceAllString(input, " ")
	return strings.TrimSpace(input)
}

// FormatDate parses and formats email dates for display.
func FormatDate(dateStr string) string {
	if strings.TrimSpace(dateStr) == "" {
		return "Unknown date"
	}
	for _, format := range []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 -0700",
		time.RFC822Z,
		time.RFC822,
	} {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t.Format("Jan 02, 2006 15:04")
		}
	}
	return dateStr
}

// IndentText adds "> " prefix to each line for quoted text.
func IndentText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = "> " + line
	}
	return strings.Join(lines, "\n")
}
