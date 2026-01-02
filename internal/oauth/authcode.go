package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

const (
	authEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
)

type AuthCodeRequest struct {
	ClientID string
	Scopes   []string
}

type AuthCodeSession struct {
	AuthURL      string
	RedirectURI  string
	ListenAddr   string
	State        string
	CodeVerifier string

	codeCh chan string
	errCh  chan error
	srv    *http.Server
}

type AuthCodeResult struct {
	Code         string
	State        string
	RedirectURI  string
	ListenAddr   string
	CodeVerifier string
}

// BeginAuthCodePKCE starts localhost callback server and returns the auth URL immediately.
func BeginAuthCodePKCE(req AuthCodeRequest) (*AuthCodeSession, error) {
	if req.ClientID == "" {
		return nil, apperr.New(apperr.AuthMissingClientID)
	}
	if len(req.Scopes) == 0 {
		return nil, apperr.New(apperr.AuthInvalidScopes).
			WithFix("Provide at least one OAuth scope.")
	}

	pkce, err := NewPKCE()
	if err != nil {
		return nil, apperr.New(apperr.AuthPKCEGen).WithCause(err)
	}

	state, err := randomState()
	if err != nil {
		return nil, apperr.New(apperr.AuthStateGen).WithCause(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, apperr.New(apperr.AuthListen).WithCause(err).
			WithFix("Check if localhost is available and not blocked by firewall.")
	}

	addr := ln.Addr().String()
	redirectURI := "http://" + addr + "/oauth/callback"

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		gotState := q.Get("state")

		if gotState != state {
			http.Error(w, "Invalid state. You can close this tab and retry.", http.StatusBadRequest)
			errCh <- apperr.New(apperr.AuthStateMismatch).
				WithMeta("expected_state", state).
				WithMeta("got_state", gotState)
			return
		}

		if e := q.Get("error"); e != "" {
			desc := q.Get("error_description")
			http.Error(w, "Authorization failed. You can close this tab and retry.", http.StatusBadRequest)
			errCh <- apperr.New(apperr.AuthDenied).
				WithMeta("oauth_error", e).
				WithMeta("oauth_error_description", desc)
			return
		}

		code := q.Get("code")
		if code == "" {
			http.Error(w, "Missing code. You can close this tab and retry.", http.StatusBadRequest)
			errCh <- apperr.New(apperr.AuthMissingCode)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<h2>Advncd: authentication complete</h2><p>You can close this tab and return to the terminal.</p>"))

		codeCh <- code
	})

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- apperr.New(apperr.AuthServe).WithCause(err)
		}
	}()

	authURL, err := buildAuthURL(req.ClientID, redirectURI, state, pkce.Challenge, pkce.Method, req.Scopes)
	if err != nil {
		_ = srv.Close()
		return nil, apperr.New(apperr.AuthAuthURL).WithCause(err)
	}

	return &AuthCodeSession{
		AuthURL:      authURL,
		RedirectURI:  redirectURI,
		ListenAddr:   addr,
		State:        state,
		CodeVerifier: pkce.Verifier,
		codeCh:       codeCh,
		errCh:        errCh,
		srv:          srv,
	}, nil
}

// Wait blocks until callback is received or ctx is done.
func (s *AuthCodeSession) Wait(ctx context.Context) (*AuthCodeResult, error) {
	select {
	case code := <-s.codeCh:
		// Give the browser a brief moment to finish loading the success page
		// (Safari sometimes makes an extra request; closing immediately causes "Can't connect" UI.)
		go func() {
			time.Sleep(2 * time.Second)
			_ = s.srv.Close()
		}()

		return &AuthCodeResult{
			Code:         code,
			State:        s.State,
			RedirectURI:  s.RedirectURI,
			ListenAddr:   s.ListenAddr,
			CodeVerifier: s.CodeVerifier,
		}, nil

	case e := <-s.errCh:
		_ = s.srv.Close()
		return nil, e

	case <-ctx.Done():
		_ = s.srv.Close()
		return nil, apperr.New(apperr.AuthAuthTimeout).
			WithCause(ctx.Err()).
			WithFix("Complete the browser login and consent, then try again.")
	}
}

func buildAuthURL(clientID, redirectURI, state, challenge, method string, scopes []string) (string, error) {
	u, err := url.Parse(authEndpoint)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", joinScopes(scopes))
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", method)

	q.Set("access_type", "offline")
	q.Set("prompt", "consent")

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func joinScopes(scopes []string) string {
	out := ""
	for i, s := range scopes {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}