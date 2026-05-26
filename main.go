package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/rdx40/gmail-tui/api"
	"github.com/rdx40/gmail-tui/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cfg, err := api.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, ".config", "gmail-tui", "app.log")
	logger, _ := api.NewLogger(logPath, api.ParseLogLevel(cfg.LogLevel))
	defer logger.Close()

	client, err := api.NewClient(cfg, logger)
	if err != nil {
		log.Fatalf("Failed to initialize Gmail client: %v", err)
	}

	p := tea.NewProgram(ui.New(client, cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("TUI error: %v", err)
	}
}
