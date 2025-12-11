package main

import (
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"google.golang.org/api/gmail/v1"
)

const (
	defaultMaxResults = 10
	inboxQuery        = "in:inbox category:primary"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func run() error {
	srv, err := getGmailService()
	if err != nil {
		return fmt.Errorf("failed to initialize Gmail service: %w", err)
	}

	messages, err := fetchInboxMessages(srv)
	if err != nil {
		return fmt.Errorf("failed to fetch inbox messages: %w", err)
	}

	labels, err := fetchLabels(srv)
	if err != nil {
		log.Printf("Warning: could not fetch labels: %v", err)
		labels = []*gmail.Label{} // Continue with empty labels
	}

	p := tea.NewProgram(
		initialModel(messages, srv, labels),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// fetchInboxMessages retrieves messages from the primary inbox
func fetchInboxMessages(srv *gmail.Service) ([]*gmail.Message, error) {
	resp, err := srv.Users.Messages.
		List("me").
		Q(inboxQuery).
		MaxResults(defaultMaxResults).
		Do()
	if err != nil {
		return nil, err
	}

	if resp == nil || len(resp.Messages) == 0 {
		fmt.Println("No messages found in inbox")
		return []*gmail.Message{}, nil
	}

	return resp.Messages, nil
}

// fetchLabels retrieves all Gmail labels for the user
func fetchLabels(srv *gmail.Service) ([]*gmail.Label, error) {
	resp, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return nil, err
	}

	if resp == nil {
		return []*gmail.Label{}, nil
	}

	return resp.Labels, nil
}
