package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const (
	credentialsFile = "credentials.json"
	tokenFileName   = ".gmail-tui-token.json" // #nosec G101
)

// checkGitignore warns if credentials.json is not listed in .gitignore.
func checkGitignore() {
	f, err := os.Open(".gitignore")
	if err != nil {
		slog.Warn("credentials.json is not in .gitignore — risk of committing secrets")
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "credentials.json") {
			return
		}
	}
	slog.Warn("credentials.json is not in .gitignore — risk of committing secrets")
}

func newGmailService() (*gmail.Service, error) {
	checkGitignore()

	ctx := context.Background()

	config, err := loadOAuthConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load OAuth config: %w", err)
	}

	client, err := getAuthenticatedClient(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to get authenticated client: %w", err)
	}

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gmail service: %w", err)
	}

	return srv, nil
}

func loadOAuthConfig() (*oauth2.Config, error) {
	credPath := os.Getenv("GMAIL_TUI_CREDENTIALS")
	if credPath == "" {
		credPath = credentialsFile
	}

	data, err := os.ReadFile(credPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("credentials file not found at %s — see README for setup instructions: %w", credPath, err)
		}
		return nil, fmt.Errorf("unable to read credentials file: %w", err)
	}

	if !json.Valid(data) {
		return nil, fmt.Errorf("credentials file at %s is not valid JSON — ensure you have downloaded your OAuth client credentials correctly (see README): %s", credPath, strings.TrimSpace(string(data)))
	}

	config, err := google.ConfigFromJSON(data, gmail.GmailReadonlyScope, gmail.GmailSendScope, gmail.GmailModifyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	return config, nil
}

func getAuthenticatedClient(ctx context.Context, config *oauth2.Config) (*http.Client, error) {
	tokenPath, err := getTokenPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine token path: %w", err)
	}

	token, err := loadToken(tokenPath)
	if err != nil {
		token, err = performOAuthFlow(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("OAuth flow failed: %w", err)
		}
		if err := saveToken(tokenPath, token); err != nil {
			return nil, fmt.Errorf("failed to save token: %w", err)
		}
	}

	ts := config.TokenSource(ctx, token)
	refreshedToken, err := ts.Token()
	if err != nil {
		token, err = performOAuthFlow(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("OAuth flow failed after refresh error: %w", err)
		}
		if err := saveToken(tokenPath, token); err != nil {
			return nil, fmt.Errorf("failed to save token: %w", err)
		}
		return config.Client(ctx, token), nil
	}

	if refreshedToken.AccessToken != token.AccessToken ||
		refreshedToken.RefreshToken != token.RefreshToken ||
		!refreshedToken.Expiry.Equal(token.Expiry) {
		_ = saveToken(tokenPath, refreshedToken)
	}

	return oauth2.NewClient(ctx, ts), nil
}

func getTokenPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, tokenFileName), nil
}

func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	token := &oauth2.Token{}
	decodeErr := json.NewDecoder(f).Decode(token)
	f.Close()
	if decodeErr != nil {
		os.Remove(path)
		return nil, fmt.Errorf("failed to decode token (corrupted file removed): %w", decodeErr)
	}
	return token, nil
}

func saveToken(path string, token *oauth2.Token) error {
	tmpPath := path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temp token file: %w", err)
	}
	if err := json.NewEncoder(f).Encode(token); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to encode token: %w", err)
	}
	f.Close()
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename token file: %w", err)
	}
	return nil
}

func performOAuthFlow(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	fmt.Printf("\nAuthorization required. Please visit:\n%s\n\n", authURL)
	fmt.Print("Enter authorization code: ")

	var code string
	if _, err := fmt.Scanln(&code); err != nil {
		return nil, fmt.Errorf("failed to read authorization code: %w", err)
	}

	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	return token, nil
}
