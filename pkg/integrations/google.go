// Octa - Personal AI Agent
// License: MIT

package integrations

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	gmail "google.golang.org/api/gmail/v1"

	"github.com/Swarup012/solo/pkg/config"
)

// GoogleClients holds the authenticated HTTP client for Google APIs.
type GoogleClients struct {
	HTTPClient *http.Client
}

// InitGoogle initialises Google OAuth and returns an authenticated HTTP client.
// Returns nil without error if Google integration is not configured.
func InitGoogle(cfg *config.Config) (*GoogleClients, error) {
	if cfg.Integrations.Google == nil {
		return nil, nil
	}

	g := cfg.Integrations.Google
	if g.ClientID == "" || g.ClientSecret == "" {
		return nil, fmt.Errorf("integrations.google: client_id and client_secret are required")
	}

	scopes := g.Scopes
	if len(scopes) == 0 {
		scopes = []string{
			calendar.CalendarScope,
			gmail.GmailModifyScope,
		}
	}

	oauthCfg := &oauth2.Config{
		ClientID:     g.ClientID,
		ClientSecret: g.ClientSecret,
		RedirectURL:  g.RedirectURI,
		Scopes:       scopes,
		Endpoint:     google.Endpoint,
	}

	tokenFile := g.TokenFile
	if tokenFile == "" {
		home, _ := os.UserHomeDir()
		tokenFile = filepath.Join(home, ".octa", "tokens", "google.json")
	}

	// Expand ~ in path
	if len(tokenFile) > 1 && tokenFile[:2] == "~/" {
		home, _ := os.UserHomeDir()
		tokenFile = filepath.Join(home, tokenFile[2:])
	}

	token, err := loadGoogleToken(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("integrations.google: no token at %s — run 'octa auth google': %w", tokenFile, err)
	}

	//nolint:staticcheck // oauth2.NoContext is fine for token refresh
	client := oauthCfg.Client(oauth2.NoContext, token)
	return &GoogleClients{HTTPClient: client}, nil
}

// loadGoogleToken loads an OAuth2 token from a JSON file.
func loadGoogleToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	token := &oauth2.Token{}
	if err := json.NewDecoder(f).Decode(token); err != nil {
		return nil, err
	}
	return token, nil
}
