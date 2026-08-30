package gmail

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

const ReadonlyScope = "https://www.googleapis.com/auth/gmail.readonly"

type credentialSection struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	AuthURI      string `json:"auth_uri"`
	TokenURI     string `json:"token_uri"`
}

type credentialFile struct {
	Installed *credentialSection `json:"installed"`
	Web       *credentialSection `json:"web"`
}

// Authenticate returns an HTTP client authorized only for Gmail read access.
func Authenticate(ctx context.Context, credentialsPath, tokenPath string) (*http.Client, error) {
	section, err := readCredentials(credentialsPath)
	if err != nil {
		return nil, err
	}
	config := &oauth2.Config{
		ClientID:     section.ClientID,
		ClientSecret: section.ClientSecret,
		Scopes:       []string{ReadonlyScope},
		Endpoint: oauth2.Endpoint{
			AuthURL:  section.AuthURI,
			TokenURL: section.TokenURI,
		},
	}

	token, err := readToken(tokenPath)
	if errors.Is(err, os.ErrNotExist) {
		token, err = authorize(ctx, config)
		if err == nil {
			err = writeToken(tokenPath, token)
		}
	}
	if err != nil {
		return nil, err
	}

	source := &savingTokenSource{
		source: oauth2.ReuseTokenSource(token, config.TokenSource(ctx, token)),
		path:   tokenPath,
		last:   token,
	}
	return oauth2.NewClient(ctx, source), nil
}

func readCredentials(path string) (*credentialSection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read OAuth credentials %q: %w", path, err)
	}
	var file credentialFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse OAuth credentials: %w", err)
	}
	section := file.Installed
	if section == nil {
		section = file.Web
	}
	if section == nil || section.ClientID == "" || section.AuthURI == "" || section.TokenURI == "" {
		return nil, fmt.Errorf("OAuth credentials must contain an installed or web client configuration")
	}
	return section, nil
}

func readToken(path string) (*oauth2.Token, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, fmt.Errorf("parse OAuth token: %w", err)
	}
	return &token, nil
}

func writeToken(path string, token *oauth2.Token) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open OAuth token file: %w", err)
	}
	encoder := json.NewEncoder(file)
	err = encoder.Encode(token)
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("write OAuth token: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close OAuth token file: %w", closeErr)
	}
	return nil
}

func authorize(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start OAuth callback listener: %w", err)
	}
	defer listener.Close()

	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("generate OAuth state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)
	config.RedirectURL = "http://" + listener.Addr().String() + "/oauth/callback"
	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	tokenChannel := make(chan *oauth2.Token, 1)
	errorChannel := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "invalid OAuth state", http.StatusBadRequest)
			errorChannel <- fmt.Errorf("OAuth callback state did not match")
			return
		}
		if oauthError := r.URL.Query().Get("error"); oauthError != "" {
			http.Error(w, "authorization was denied", http.StatusBadRequest)
			errorChannel <- fmt.Errorf("OAuth authorization failed: %s", oauthError)
			return
		}
		token, err := config.Exchange(r.Context(), r.URL.Query().Get("code"))
		if err != nil {
			http.Error(w, "could not exchange authorization code", http.StatusBadRequest)
			errorChannel <- fmt.Errorf("exchange OAuth code: %w", err)
			return
		}
		_, _ = fmt.Fprintln(w, "Inbox Cleaner is authorized. You can close this tab.")
		tokenChannel <- token
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errorChannel <- serveErr
		}
	}()

	fmt.Fprintf(os.Stderr, "Open this URL to authorize Inbox Cleaner:\n%s\n", authURL)
	_ = openBrowser(authURL)

	var token *oauth2.Token
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-errorChannel:
	case token = <-tokenChannel:
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func openBrowser(rawURL string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		command, args = "open", []string{rawURL}
	case "windows":
		command, args = "rundll32", []string{"url.dll,FileProtocolHandler", rawURL}
	default:
		command, args = "xdg-open", []string{rawURL}
	}
	return exec.Command(command, args...).Start()
}

type savingTokenSource struct {
	mu     sync.Mutex
	source oauth2.TokenSource
	path   string
	last   *oauth2.Token
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := s.source.Token()
	if err != nil {
		return nil, err
	}
	if s.last == nil || token.AccessToken != s.last.AccessToken || token.RefreshToken != s.last.RefreshToken || !token.Expiry.Equal(s.last.Expiry) {
		if err := writeToken(s.path, token); err != nil {
			return nil, err
		}
		s.last = token
	}
	return token, nil
}
