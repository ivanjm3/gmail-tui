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

func newGmailService() (*gmail.Service, string, error) {
	checkGitignore()

	ctx := context.Background()

	config, err := loadOAuthConfig()
	if err != nil {
		return nil, "", fmt.Errorf("failed to load OAuth config: %w", err)
	}

	client, grantedScope, err := getAuthenticatedClient(ctx, config)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get authenticated client: %w", err)
	}

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create Gmail service: %w", err)
	}

	return srv, grantedScope, nil
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

// getAuthenticatedClient returns an authenticated HTTP client along with the
// space-separated set of scopes Google actually granted for the token in use.
// Google's consent screen lets a user deselect individual scope checkboxes, so
// the granted set can be a subset of the scopes requested in loadOAuthConfig.
func getAuthenticatedClient(ctx context.Context, config *oauth2.Config) (*http.Client, string, error) {
	tokenPath, err := getTokenPath()
	if err != nil {
		return nil, "", fmt.Errorf("failed to determine token path: %w", err)
	}

	token, scope, err := loadToken(tokenPath)
	if err != nil {
		token, scope, err = performOAuthFlow(ctx, config)
		if err != nil {
			return nil, "", fmt.Errorf("OAuth flow failed: %w", err)
		}
		if err := saveToken(tokenPath, token, scope); err != nil {
			return nil, "", fmt.Errorf("failed to save token: %w", err)
		}
	}

	ts := config.TokenSource(ctx, token)
	refreshedToken, err := ts.Token()
	if err != nil {
		token, scope, err = performOAuthFlow(ctx, config)
		if err != nil {
			return nil, "", fmt.Errorf("OAuth flow failed after refresh error: %w", err)
		}
		if err := saveToken(tokenPath, token, scope); err != nil {
			return nil, "", fmt.Errorf("failed to save token: %w", err)
		}
		return config.Client(ctx, token), scope, nil
	}

	if refreshedToken.AccessToken != token.AccessToken ||
		refreshedToken.RefreshToken != token.RefreshToken ||
		!refreshedToken.Expiry.Equal(token.Expiry) {
		_ = saveToken(tokenPath, refreshedToken, scope)
	}

	return oauth2.NewClient(ctx, ts), scope, nil
}

func getTokenPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, tokenFileName), nil
}

// storedToken persists the oauth2.Token fields plus the granted scope string,
// since encoding/json on a bare oauth2.Token would otherwise drop it.
type storedToken struct {
	*oauth2.Token
	Scope string `json:"scope,omitempty"`
}

func loadToken(path string) (*oauth2.Token, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}

	stored := storedToken{Token: &oauth2.Token{}}
	decodeErr := json.NewDecoder(f).Decode(&stored)
	f.Close()
	if decodeErr != nil {
		os.Remove(path)
		return nil, "", fmt.Errorf("failed to decode token (corrupted file removed): %w", decodeErr)
	}
	return stored.Token, stored.Scope, nil
}

func saveToken(path string, token *oauth2.Token, scope string) error {
	tmpPath := path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temp token file: %w", err)
	}
	if err := json.NewEncoder(f).Encode(storedToken{Token: token, Scope: scope}); err != nil {
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

func performOAuthFlow(ctx context.Context, config *oauth2.Config) (*oauth2.Token, string, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	fmt.Printf("\nAuthorization required. Please visit:\n%s\n\n", authURL)
	fmt.Print("Enter authorization code: ")

	var code string
	if _, err := fmt.Scanln(&code); err != nil {
		return nil, "", fmt.Errorf("failed to read authorization code: %w", err)
	}

	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, "", fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Google returns the actually-granted scopes on the token response; the
	// user may have deselected some via the consent screen's granular toggles.
	// Fall back to the requested scopes if the provider omits it.
	scope, ok := token.Extra("scope").(string)
	if !ok || scope == "" {
		scope = strings.Join(config.Scopes, " ")
	}

	return token, scope, nil
}
