package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	raidydiscord "github.com/joshchen-dev/raidy/internal/discord"
	"github.com/joshchen-dev/raidy/internal/postgres"
)

type Config struct {
	ClientID     string
	ClientSecret string
	BaseURL      string
	StaticDir    string
}

type Handler struct {
	store      *postgres.Store
	discord    *discordgo.Session
	bot        *raidydiscord.Bot
	log        *slog.Logger
	config     Config
	origin     string
	sessions   *sessionStore
	httpClient *http.Client
	discordAPI string
	static     http.Handler
	index      string
}

type clientError struct{ message string }

func (e clientError) Error() string { return e.message }

func New(store *postgres.Store, session *discordgo.Session, bot *raidydiscord.Bot, log *slog.Logger, config Config) (*Handler, error) {
	if config.ClientID == "" || config.ClientSecret == "" || config.BaseURL == "" {
		return nil, errors.New("DISCORD_CLIENT_ID, DISCORD_CLIENT_SECRET, and APP_BASE_URL are required")
	}
	base, err := url.Parse(config.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, errors.New("APP_BASE_URL must be an absolute URL")
	}
	h := &Handler{
		store: store, discord: session, bot: bot, log: log, config: config,
		origin: base.Scheme + "://" + base.Host, sessions: newSessionStore(),
		httpClient: &http.Client{Timeout: 10 * time.Second}, discordAPI: "https://discord.com/api/v10",
	}
	if config.StaticDir != "" {
		index := filepath.Join(config.StaticDir, "index.html")
		if _, err := os.Stat(index); err == nil {
			h.static = http.FileServer(http.Dir(config.StaticDir))
			h.index = index
		}
	}
	return h, nil
}

func (h *Handler) Routes(health http.Handler) http.Handler {
	root := http.NewServeMux()
	root.Handle("/healthz", health)
	root.Handle("/readyz", health)
	root.HandleFunc("GET /api/auth/login", h.login)
	root.HandleFunc("GET /api/auth/callback", h.callback)

	api := http.NewServeMux()
	api.HandleFunc("GET /api/auth/me", h.me)
	api.HandleFunc("POST /api/auth/logout", h.logout)
	api.HandleFunc("GET /api/guilds", h.guilds)
	api.HandleFunc("GET /api/guilds/{guildID}/members", h.members)
	api.HandleFunc("GET /api/guilds/{guildID}/channels", h.channels)
	api.HandleFunc("GET /api/teams", h.teams)
	api.HandleFunc("POST /api/teams", h.createTeam)
	api.HandleFunc("PATCH /api/teams/{teamID}", h.updateTeam)
	api.HandleFunc("DELETE /api/teams/{teamID}", h.deleteTeam)
	api.HandleFunc("GET /api/teams/{teamID}/schedule", h.getSchedule)
	api.HandleFunc("PUT /api/teams/{teamID}/schedule", h.saveSchedule)
	api.HandleFunc("POST /api/teams/{teamID}/schedule/preview", h.previewSchedule)
	api.HandleFunc("POST /api/teams/{teamID}/schedule/enabled", h.setScheduleEnabled)
	api.HandleFunc("POST /api/teams/{teamID}/publish", h.publish)
	api.HandleFunc("POST /api/teams/{teamID}/republish", h.republish)
	api.HandleFunc("GET /api/teams/{teamID}/poll", h.currentPoll)
	root.Handle("/api/", h.requireSession(api))
	root.HandleFunc("/", h.serveApp)
	return root
}

func (h *Handler) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if unsafeMethod(r.Method) && r.Header.Get("Origin") != "" && r.Header.Get("Origin") != h.origin {
			writeError(w, http.StatusForbidden, "invalid request origin")
			return
		}
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "sign in with Discord")
			return
		}
		value, ok := h.sessions.get(cookie.Value)
		if !ok {
			writeError(w, http.StatusUnauthorized, "session expired; sign in again")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, value)))
	})
}

func (h *Handler) serveApp(w http.ResponseWriter, r *http.Request) {
	if h.static == nil {
		http.Error(w, "Raidy web assets are not built; run npm run dev in web/", http.StatusNotFound)
		return
	}
	if r.URL.Path != "/" && filepath.Ext(r.URL.Path) == "" {
		http.ServeFile(w, r, h.index)
		return
	}
	h.static.ServeHTTP(w, r)
}

func sessionFrom(ctx context.Context) session {
	return ctx.Value(sessionKey{}).(session)
}

func decode(r *http.Request, value any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return errors.New("Content-Type must be application/json")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return errors.New("invalid JSON request")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func unsafeMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	var inputError clientError
	var validation postgres.ValidationError
	switch {
	case errors.As(err, &inputError), errors.As(err, &validation):
		status = http.StatusBadRequest
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
	case errors.Is(err, postgres.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, postgres.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, postgres.ErrAlreadyPublished), errors.Is(err, postgres.ErrDuplicateTeam):
		status = http.StatusConflict
	}
	if status == http.StatusGatewayTimeout {
		h.log.Warn("web request timed out", "method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, status, "The request timed out; try again")
		return
	}
	if status == http.StatusInternalServerError {
		h.log.Error("web request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, status, "internal server error")
		return
	}
	writeError(w, status, err.Error())
}
