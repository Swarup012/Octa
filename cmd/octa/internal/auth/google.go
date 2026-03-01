package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	gmail "google.golang.org/api/gmail/v1"

	"github.com/Swarup012/solo/cmd/octa/internal"
	"github.com/Swarup012/solo/pkg/config"
)

func newGoogleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "google",
		Short: "Authenticate with Google (Calendar + Gmail)",
		Long: `Open a browser window to authenticate with Google.
Saves the OAuth token to ~/.octa/tokens/google.json.

Your config.json must have integrations.google with client_id and client_secret.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return authLoginGoogle()
		},
	}
	return cmd
}

func authLoginGoogle() error {
	cfg, err := internal.LoadConfig()
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	if cfg.Integrations.Google == nil {
		return fmt.Errorf(
			"integrations.google is not configured in your config.json\n" +
				"Add a section like:\n" +
				"  \"integrations\": {\n" +
				"    \"google\": {\n" +
				"      \"client_id\": \"YOUR_CLIENT_ID\",\n" +
				"      \"client_secret\": \"YOUR_CLIENT_SECRET\"\n" +
				"    }\n" +
				"  }",
		)
	}

	g := cfg.Integrations.Google
	if g.ClientID == "" || g.ClientSecret == "" {
		return fmt.Errorf("integrations.google: client_id and client_secret are required in config.json")
	}

	scopes := g.Scopes
	if len(scopes) == 0 {
		scopes = []string{
			calendar.CalendarScope,
			gmail.GmailModifyScope,
		}
	}

	redirectURI := fmt.Sprintf("http://127.0.0.1:9876/auth/callback")
	if g.RedirectURI != "" {
		redirectURI = g.RedirectURI
	}
	redirectPort := parseRedirectPort(redirectURI)
	redirectPath := parseRedirectPath(redirectURI)

	oauthCfg := &oauth2.Config{
		ClientID:     g.ClientID,
		ClientSecret: g.ClientSecret,
		RedirectURL:  redirectURI,
		Scopes:       scopes,
		Endpoint:     google.Endpoint,
	}

	// Generate state token for CSRF protection
	stateBuf := make([]byte, 16)
	if _, err := rand.Read(stateBuf); err != nil {
		return fmt.Errorf("generating state: %w", err)
	}
	state := hex.EncodeToString(stateBuf)

	authURL := oauthCfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
	)

	// Start local callback server
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(redirectPath, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("state mismatch — possible CSRF")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			errCh <- fmt.Errorf("no code received: %s", errMsg)
			http.Error(w, "No authorization code received", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h2>✅ Google authentication successful!</h2><p>You can close this window and return to the terminal.</p></body></html>")
		codeCh <- code
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", redirectPort))
	if err != nil {
		return fmt.Errorf("could not start callback server on port %d: %w", redirectPort, err)
	}

	server := &http.Server{Handler: mux}
	go server.Serve(listener) //nolint:errcheck
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		server.Shutdown(ctx) //nolint:errcheck
	}()

	fmt.Println("🔑 Opening browser for Google authentication...")
	fmt.Printf("\nIf the browser doesn't open, visit this URL:\n\n%s\n\n", authURL)

	// Try to open browser
	openBrowserLocal(authURL) //nolint:errcheck

	fmt.Println("Waiting for authentication...")

	var code string
	select {
	case code = <-codeCh:
		// got it from browser callback
	case err = <-errCh:
		return fmt.Errorf("authentication failed: %w", err)
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("authentication timed out after 5 minutes")
	}

	// Exchange code for token
	ctx := context.Background()
	token, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	// Save token to file
	tokenFile := g.TokenFile
	if tokenFile == "" {
		home, _ := os.UserHomeDir()
		tokenFile = filepath.Join(home, ".octa", "tokens", "google.json")
	}
	// Expand ~/
	if len(tokenFile) > 1 && tokenFile[:2] == "~/" {
		home, _ := os.UserHomeDir()
		tokenFile = filepath.Join(home, tokenFile[2:])
	}

	if err := saveGoogleToken(tokenFile, token); err != nil {
		return fmt.Errorf("could not save token: %w", err)
	}

	fmt.Printf("\n✅ Google authentication successful!\n")
	fmt.Printf("Token saved to: %s\n", tokenFile)
	fmt.Println("Google Calendar and Gmail are now available.")

	// Update config to ensure google integration is marked active
	cfg.Integrations.Google.TokenFile = tokenFile
	if saveErr := config.SaveConfig(internal.GetConfigPath(), cfg); saveErr != nil {
		fmt.Printf("Warning: could not update config: %v\n", saveErr)
	}

	return nil
}

func saveGoogleToken(path string, token *oauth2.Token) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}

func openBrowserLocal(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// parseRedirectPort extracts the port number from a redirect URI.
func parseRedirectPort(redirectURI string) int {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return 9876
	}
	port := u.Port()
	if port == "" {
		return 9876
	}
	var p int
	fmt.Sscanf(port, "%d", &p)
	if p == 0 {
		return 9876
	}
	return p
}

// parseRedirectPath extracts the path from a redirect URI.
func parseRedirectPath(redirectURI string) string {
	u, err := url.Parse(redirectURI)
	if err != nil || u.Path == "" {
		return "/auth/callback"
	}
	return u.Path
}
