package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestCheckGitignore(t *testing.T) {
	tempDir := t.TempDir()
	logs := captureSlog(t)

	withWorkingDir(t, tempDir, func() {
		checkGitignore()
	})
	if !strings.Contains(logs.String(), "credentials.json is not in .gitignore") {
		t.Fatalf("expected warning for missing .gitignore entry, got %q", logs.String())
	}

	logs.Reset()
	if err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte("credentials.json\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	withWorkingDir(t, tempDir, func() {
		checkGitignore()
	})
	if logs.Len() != 0 {
		t.Fatalf("expected no warning when .gitignore contains credentials.json, got %q", logs.String())
	}
}

func TestLoadOAuthConfig(t *testing.T) {
	credentialsPath := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(credentialsPath, []byte(`{
		"installed": {
			"client_id": "client-id",
			"client_secret": "client-secret",
			"redirect_uris": ["http://127.0.0.1"],
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": "https://oauth2.googleapis.com/token"
		}
	}`), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("GMAIL_TUI_CREDENTIALS", credentialsPath)

	cfg, err := loadOAuthConfig()
	if err != nil {
		t.Fatalf("loadOAuthConfig() error = %v", err)
	}
	if cfg.ClientID != "client-id" || len(cfg.Scopes) != 3 {
		t.Fatalf("unexpected OAuth config: %+v", cfg)
	}
}

func TestLoadOAuthConfigMissingFile(t *testing.T) {
	t.Setenv("GMAIL_TUI_CREDENTIALS", filepath.Join(t.TempDir(), "missing.json"))

	_, err := loadOAuthConfig()
	if err == nil || !strings.Contains(err.Error(), "credentials file not found") {
		t.Fatalf("expected missing credentials error, got %v", err)
	}
}

func TestLoadOAuthConfigInvalidJSON(t *testing.T) {
	credentialsPath := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(credentialsPath, []byte(`REPLACE_WITH_THINE`), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("GMAIL_TUI_CREDENTIALS", credentialsPath)

	_, err := loadOAuthConfig()
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
	if !strings.Contains(err.Error(), "REPLACE_WITH_THINE") {
		t.Fatalf("expected error to contain the invalid content, got %v", err)
	}
}

func TestGetTokenPathUsesHomeDir(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	tokenPath, err := getTokenPath()
	if err != nil {
		t.Fatalf("getTokenPath() error = %v", err)
	}
	if tokenPath != filepath.Join(home, tokenFileName) {
		t.Fatalf("getTokenPath() = %q", tokenPath)
	}
}

func TestSaveTokenAndLoadToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.json")
	want := &oauth2.Token{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour).Round(time.Second),
	}

	if err := saveToken(path, want, "https://www.googleapis.com/auth/gmail.send"); err != nil {
		t.Fatalf("saveToken() error = %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected temp token file to be removed, err = %v", err)
	}

	got, scope, err := loadToken(path)
	if err != nil {
		t.Fatalf("loadToken() error = %v", err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || got.TokenType != want.TokenType {
		t.Fatalf("loadToken() = %+v, want %+v", got, want)
	}
	if scope != "https://www.googleapis.com/auth/gmail.send" {
		t.Fatalf("loadToken() scope = %q", scope)
	}
}

func TestLoadTokenCorruptedFileRemovesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.json")
	if err := os.WriteFile(path, []byte("not-json"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, _, err := loadToken(path)
	if err == nil || !strings.Contains(err.Error(), "corrupted file removed") {
		t.Fatalf("expected corruption error, got %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("expected corrupted token file to be removed, err = %v", statErr)
	}
}

func TestGetAuthenticatedClientUsesSavedToken(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	tokenPath, err := getTokenPath()
	if err != nil {
		t.Fatalf("getTokenPath() error = %v", err)
	}
	if err := saveToken(tokenPath, &oauth2.Token{
		AccessToken: "access-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}, "https://www.googleapis.com/auth/gmail.readonly"); err != nil {
		t.Fatalf("saveToken() error = %v", err)
	}

	client, scope, err := getAuthenticatedClient(context.Background(), &oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "http://127.0.0.1",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://example.com/auth",
			TokenURL: "https://example.com/token",
		},
	})
	if err != nil {
		t.Fatalf("getAuthenticatedClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("expected authenticated client")
	}
	if scope != "https://www.googleapis.com/auth/gmail.readonly" {
		t.Fatalf("getAuthenticatedClient() scope = %q", scope)
	}
}

func TestPerformOAuthFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access-token",
			"refresh_token": "refresh-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	cfg := &oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "http://127.0.0.1",
		Endpoint: oauth2.Endpoint{
			AuthURL:  server.URL + "/auth",
			TokenURL: server.URL,
		},
	}

	inputPath := filepath.Join(t.TempDir(), "stdin.txt")
	if err := os.WriteFile(inputPath, []byte("auth-code\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	withStdinFile(t, inputPath, func() {
		token, _, err := performOAuthFlow(context.Background(), cfg)
		if err != nil {
			t.Fatalf("performOAuthFlow() error = %v", err)
		}
		if token.AccessToken != "access-token" || token.RefreshToken != "refresh-token" {
			t.Fatalf("unexpected token: %+v", token)
		}
	})
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	}()

	fn()
}

func withStdinFile(t *testing.T, path string, fn func()) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	oldStdin := os.Stdin
	os.Stdin = file
	defer func() {
		os.Stdin = oldStdin
	}()

	fn()
}
