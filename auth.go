package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const (
	credentialsFile = "credentials.json"
	tokenFileName   = ".gmail-tui-token.json"
)

// getGmailService initializes and returns an authenticated Gmail API service
func getGmailService() (*gmail.Service, error) {
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

// loadOAuthConfig reads and parses the OAuth2 credentials file
func loadOAuthConfig() (*oauth2.Config, error) {
	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %w", err)
	}

	config, err := google.ConfigFromJSON(data, gmail.MailGoogleComScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	return config, nil
}

// getAuthenticatedClient returns an authenticated HTTP client
func getAuthenticatedClient(ctx context.Context, config *oauth2.Config) (*http.Client, error) {
	tokenPath, err := getTokenPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine token path: %w", err)
	}

	token, err := loadToken(tokenPath)
	if err != nil {
		// Token doesn't exist or is invalid, perform OAuth flow
		token, err = performOAuthFlow(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("OAuth flow failed: %w", err)
		}

		if err := saveToken(tokenPath, token); err != nil {
			return nil, fmt.Errorf("failed to save token: %w", err)
		}
	}

	// Create a TokenSource from the loaded token so we can attempt a refresh now.
	ts := config.TokenSource(ctx, token)

	// Try to retrieve a valid token from the source (this will refresh if needed).
	refreshedToken, err := ts.Token()
	if err != nil {
		// Refresh failed (invalid_grant, revoked refresh token, etc.).
		// Fall back to interactive OAuth flow to obtain a new token.
		token, err = performOAuthFlow(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("OAuth flow failed after refresh error: %w", err)
		}

		if err := saveToken(tokenPath, token); err != nil {
			return nil, fmt.Errorf("failed to save token: %w", err)
		}

		return config.Client(ctx, token), nil
	}

	// If the token has been refreshed, persist it.
	if refreshedToken.AccessToken != token.AccessToken || refreshedToken.RefreshToken != token.RefreshToken || !refreshedToken.Expiry.Equal(token.Expiry) {
		_ = saveToken(tokenPath, refreshedToken) // best-effort save; ignore error here
	}

	// Return an HTTP client that uses the token source (it will handle refreshes).
	return oauth2.NewClient(ctx, ts), nil
}

// getTokenPath returns the full path to the token file
func getTokenPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, tokenFileName), nil
}

// loadToken reads and deserializes a token from disk
func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	token := &oauth2.Token{}
	if err := json.NewDecoder(f).Decode(token); err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	return token, nil
}

// saveToken serializes and saves a token to disk
func saveToken(path string, token *oauth2.Token) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create token file: %w", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(token); err != nil {
		return fmt.Errorf("failed to encode token: %w", err)
	}

	return nil
}

// performOAuthFlow executes the OAuth2 authorization code flow
func performOAuthFlow(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	// Request offline access and force consent so Google returns a refresh token.
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
