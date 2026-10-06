package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/joshchen-dev/raidy/internal/postgres"
	"github.com/joshchen-dev/raidy/internal/team"
	"github.com/joshchen-dev/raidy/migrations"
)

func TestLoginSetsOAuthState(t *testing.T) {
	h := &Handler{config: Config{ClientID: "client", BaseURL: "http://localhost:5173"}}
	request := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	response := httptest.NewRecorder()
	h.login(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("login status = %d", response.Code)
	}
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Host != "discord.com" || location.Query().Get("scope") != "identify guilds" {
		t.Fatalf("unexpected OAuth redirect: %s", location)
	}
	var state string
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == stateCookie {
			state = cookie.Value
		}
	}
	if state == "" || state != location.Query().Get("state") {
		t.Fatal("OAuth state cookie and query did not match")
	}
}

func TestSessionLifecycle(t *testing.T) {
	testSessionStore(t, newMemorySessions())
}

func TestDatabaseSessionLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	if err := migrations.Up(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	store, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	sessions := dbSessions{store: store, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	testSessionStore(t, sessions)
	// A session must survive a new store instance, as after a restart.
	token, err := sessions.put(ctx, session{User: user{ID: "restart"}, Guilds: []guild{{ID: "g", Name: "Guild"}}, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	restarted := dbSessions{store: store, log: sessions.log}
	if value, ok := restarted.get(ctx, token); !ok || value.User.ID != "restart" || value.guildName("g") != "Guild" {
		t.Fatalf("session after restart = %+v, ok=%v", value, ok)
	}
}

func testSessionStore(t *testing.T, store sessionStore) {
	t.Helper()
	ctx := context.Background()
	token, err := store.put(ctx, session{User: user{ID: "user"}, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := store.get(ctx, token); !ok || value.User.ID != "user" {
		t.Fatalf("session = %+v, ok=%v", value, ok)
	}
	store.delete(ctx, token)
	if _, ok := store.get(ctx, token); ok {
		t.Fatal("deleted session remained valid")
	}
	expired, err := store.put(ctx, session{ExpiresAt: time.Now().Add(-time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.get(ctx, expired); ok {
		t.Fatal("expired session remained valid")
	}
	if _, ok := store.get(ctx, "unknown-token"); ok {
		t.Fatal("unknown token was accepted")
	}
}

func TestRequireSessionAndOrigin(t *testing.T) {
	h := &Handler{origin: "http://localhost:5173", sessions: newMemorySessions()}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	protected := h.requireSession(next)

	request := httptest.NewRequest(http.MethodGet, "/api/teams", nil)
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", response.Code)
	}

	token, _ := h.sessions.put(context.Background(), session{ExpiresAt: time.Now().Add(time.Hour)})
	request = httptest.NewRequest(http.MethodPost, "/api/teams", nil)
	request.Header.Set("Origin", "https://attacker.example")
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	response = httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/teams", nil)
	request.Header.Set("Origin", h.origin)
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	response = httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("same-origin status = %d", response.Code)
	}
}

func TestParseClock(t *testing.T) {
	minutes, err := parseClock("21:15")
	if err != nil || minutes != 21*60+15 {
		t.Fatalf("parseClock() = %d, %v", minutes, err)
	}
	if _, err := parseClock("9:15"); err == nil {
		t.Fatal("non-canonical clock value was accepted")
	}
}

func TestScheduleDraftUsesDomainGenerator(t *testing.T) {
	h := &Handler{}
	draft, occurrences, err := h.scheduleDraft(team.Team{ID: 7, GuildID: "guild", Name: "The Echo", Timezone: "Asia/Tokyo"}, "leader", scheduleInput{
		Timezone: "Asia/Tokyo", CadenceDays: 14, Weekdays: []int{1}, StartTime: "21:00", EndTime: "23:00",
		PublishLeadDays: 3, FirstPeriodStart: "2099-01-05", ChannelID: "channel",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(occurrences) != 2 || draft.FirstPublishAt.IsZero() {
		t.Fatalf("draft=%+v occurrences=%+v", draft, occurrences)
	}
}

func TestBotChannelPermissionsUsesCachedGuildState(t *testing.T) {
	session, err := discordgo.New("Bot test")
	if err != nil {
		t.Fatal(err)
	}
	session.State.User = &discordgo.User{ID: "bot"}
	needed := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)
	guild := &discordgo.Guild{
		ID: "guild", OwnerID: "owner",
		Roles:   []*discordgo.Role{{ID: "guild", Permissions: needed}},
		Members: []*discordgo.Member{{GuildID: "guild", User: session.State.User}},
	}
	if err := session.State.GuildAdd(guild); err != nil {
		t.Fatal(err)
	}
	allowed := &discordgo.Channel{ID: "allowed", GuildID: "guild", Type: discordgo.ChannelTypeGuildText}
	denied := &discordgo.Channel{ID: "denied", GuildID: "guild", Type: discordgo.ChannelTypeGuildText, PermissionOverwrites: []*discordgo.PermissionOverwrite{{ID: "guild", Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionSendMessages}}}
	h := &Handler{discord: session}
	permissions, err := h.botChannelPermissions(context.Background(), "guild", []*discordgo.Channel{allowed, denied})
	if err != nil {
		t.Fatal(err)
	}
	if permissions[allowed.ID]&needed != needed {
		t.Fatal("writable channel was rejected")
	}
	if permissions[denied.ID]&needed == needed {
		t.Fatal("denied channel was accepted")
	}
}

// Regression: Discord sends the display name as "global_name"; reading the
// wrong key left every signed-in user shown by username only.
func TestDiscordIdentityReadsGlobalNameAndBotGuilds(t *testing.T) {
	discord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/users/@me":
			_, _ = w.Write([]byte(`{"id":"u1","username":"alisaie","global_name":"Alisaie","avatar":"abc"}`))
		case "/users/@me/guilds":
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Scions"},{"id":"g2","name":"Without bot"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discord.Close()
	session, err := discordgo.New("Bot test")
	if err != nil {
		t.Fatal(err)
	}
	if err := session.State.GuildAdd(&discordgo.Guild{ID: "g1"}); err != nil {
		t.Fatal(err)
	}
	h := &Handler{discord: session, discordAPI: discord.URL, httpClient: discord.Client()}
	current, guilds, err := h.discordIdentity(httptest.NewRequest(http.MethodGet, "/api/auth/callback", nil), "token")
	if err != nil {
		t.Fatal(err)
	}
	if current.GlobalName != "Alisaie" || current.Username != "alisaie" || current.Avatar != "abc" {
		t.Fatalf("user = %+v, want global_name decoded", current)
	}
	if len(guilds) != 1 || guilds[0].ID != "g1" {
		t.Fatalf("guilds = %+v, want only the guild the bot is in", guilds)
	}
}
