package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joshchen-dev/raidy/internal/postgres"
)

const (
	sessionCookie = "raidy_session"
	stateCookie   = "raidy_oauth_state"
)

type sessionKey struct{}

type user struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"globalName"`
	Avatar     string `json:"avatar"`
}

type guild struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

type session struct {
	User      user
	Guilds    []guild
	ExpiresAt time.Time
}

func (s session) hasGuild(id string) bool {
	for _, guild := range s.Guilds {
		if guild.ID == id {
			return true
		}
	}
	return false
}

func (s session) guildName(id string) string {
	for _, guild := range s.Guilds {
		if guild.ID == id {
			return guild.Name
		}
	}
	return ""
}

// sessionStore keeps signed-in web sessions keyed by an opaque cookie token.
type sessionStore interface {
	put(ctx context.Context, value session) (string, error)
	get(ctx context.Context, token string) (session, bool)
	delete(ctx context.Context, token string)
}

// memorySessions is the process-local store used by tests.
type memorySessions struct {
	mu     sync.Mutex
	values map[string]session
}

func newMemorySessions() *memorySessions { return &memorySessions{values: make(map[string]session)} }

func (s *memorySessions) put(_ context.Context, value session) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	for key, existing := range s.values {
		if time.Now().After(existing.ExpiresAt) {
			delete(s.values, key)
		}
	}
	s.values[token] = value
	s.mu.Unlock()
	return token, nil
}

func (s *memorySessions) get(_ context.Context, token string) (session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[token]
	if !ok || time.Now().After(value.ExpiresAt) {
		delete(s.values, token)
		return session{}, false
	}
	return value, true
}

func (s *memorySessions) delete(_ context.Context, token string) {
	s.mu.Lock()
	delete(s.values, token)
	s.mu.Unlock()
}

// dbSessions persists sessions in PostgreSQL so restarts and deploys keep
// people signed in. Only a SHA-256 hash of each token is stored.
type dbSessions struct {
	store *postgres.Store
	log   *slog.Logger
}

func tokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func (s dbSessions) put(ctx context.Context, value session) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return token, s.store.PutSession(ctx, tokenHash(token), data, value.ExpiresAt, time.Now())
}

func (s dbSessions) get(ctx context.Context, token string) (session, bool) {
	data, err := s.store.Session(ctx, tokenHash(token), time.Now())
	if err != nil {
		if !errors.Is(err, postgres.ErrNotFound) {
			s.log.Error("session lookup failed", "error", err)
		}
		return session{}, false
	}
	var value session
	if err := json.Unmarshal(data, &value); err != nil {
		s.log.Error("stored session is unreadable", "error", err)
		return session{}, false
	}
	return value, true
}

func (s dbSessions) delete(ctx context.Context, token string) {
	if err := s.store.DeleteSession(ctx, tokenHash(token)); err != nil {
		s.log.Error("session delete failed", "error", err)
	}
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	state, err := randomToken()
	if err != nil {
		h.fail(w, r, err)
		return
	}
	// The team to open after sign-in rides in the state, which the callback
	// already verifies against the cookie, so it cannot be tampered with.
	if teamID, err := strconv.ParseInt(r.URL.Query().Get("team"), 10, 64); err == nil && teamID > 0 {
		state += "." + strconv.FormatInt(teamID, 10)
	}
	h.setCookie(w, stateCookie, state, 10*time.Minute)
	query := url.Values{
		"client_id":     {h.config.ClientID},
		"redirect_uri":  {strings.TrimRight(h.config.BaseURL, "/") + "/api/auth/callback"},
		"response_type": {"code"},
		"scope":         {"identify guilds"},
		"state":         {state},
	}
	http.Redirect(w, r, "https://discord.com/oauth2/authorize?"+query.Encode(), http.StatusFound)
}

func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	state, err := r.Cookie(stateCookie)
	if err != nil || state.Value == "" || state.Value != r.URL.Query().Get("state") {
		writeError(w, http.StatusBadRequest, "invalid or expired OAuth state")
		return
	}
	h.clearCookie(w, stateCookie)
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "Discord authorization was not completed")
		return
	}
	token, err := h.exchangeCode(r, code)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	current, guilds, err := h.discordIdentity(r, token)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	sessionToken, err := h.sessions.put(r.Context(), session{User: current, Guilds: guilds, ExpiresAt: time.Now().Add(24 * time.Hour)})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.setCookie(w, sessionCookie, sessionToken, 24*time.Hour)
	http.Redirect(w, r, afterLogin(state.Value), http.StatusFound)
}

// afterLogin is the app page to open once the OAuth state checks out.
func afterLogin(state string) string {
	_, team, found := strings.Cut(state, ".")
	if teamID, err := strconv.ParseInt(team, 10, 64); found && err == nil && teamID > 0 {
		return "/app?team=" + strconv.FormatInt(teamID, 10)
	}
	return "/app"
}

func (h *Handler) exchangeCode(r *http.Request, code string) (string, error) {
	form := url.Values{
		"client_id":     {h.config.ClientID},
		"client_secret": {h.config.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {strings.TrimRight(h.config.BaseURL, "/") + "/api/auth/callback"},
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, h.discordAPI+"/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := h.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", errors.New("Discord rejected the OAuth exchange")
	}
	var value struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil || value.AccessToken == "" {
		return "", errors.New("invalid OAuth response from Discord")
	}
	return value.AccessToken, nil
}

func (h *Handler) discordIdentity(r *http.Request, token string) (user, []guild, error) {
	var identity struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		GlobalName string `json:"global_name"`
		Avatar     string `json:"avatar"`
	}
	if err := h.discordGet(r, token, "/users/@me", &identity); err != nil {
		return user{}, nil, err
	}
	current := user{ID: identity.ID, Username: identity.Username, GlobalName: identity.GlobalName, Avatar: identity.Avatar}
	var available []guild
	if err := h.discordGet(r, token, "/users/@me/guilds", &available); err != nil {
		return current, nil, err
	}
	botGuilds := make(map[string]bool)
	for _, value := range h.discord.State.Guilds {
		botGuilds[value.ID] = true
	}
	filtered := available[:0]
	for _, value := range available {
		if botGuilds[value.ID] {
			filtered = append(filtered, value)
		}
	}
	return current, filtered, nil
}

func (h *Handler) discordGet(r *http.Request, token, path string, value any) error {
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, h.discordAPI+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := h.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("Discord identity request failed")
	}
	return json.NewDecoder(response.Body).Decode(value)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sessionFrom(r.Context()).User)
}

func (h *Handler) guilds(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sessionFrom(r.Context()).Guilds)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		h.sessions.delete(r.Context(), cookie.Value)
	}
	h.clearCookie(w, sessionCookie)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setCookie(w http.ResponseWriter, name, value string, age time.Duration) {
	secure := strings.HasPrefix(h.config.BaseURL, "https://")
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: int(age.Seconds())})
}

func (h *Handler) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(h.config.BaseURL, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: -1})
}
