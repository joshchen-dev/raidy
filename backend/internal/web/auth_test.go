package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/joshchen-dev/raidy/internal/team"
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
	store := newSessionStore()
	token, err := store.put(session{User: user{ID: "user"}, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := store.get(token); !ok || value.User.ID != "user" {
		t.Fatalf("session = %+v, ok=%v", value, ok)
	}
	store.delete(token)
	if _, ok := store.get(token); ok {
		t.Fatal("deleted session remained valid")
	}
	store.values["expired"] = session{ExpiresAt: time.Now().Add(-time.Second)}
	if _, ok := store.get("expired"); ok {
		t.Fatal("expired session remained valid")
	}
}

func TestRequireSessionAndOrigin(t *testing.T) {
	h := &Handler{origin: "http://localhost:5173", sessions: newSessionStore()}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	protected := h.requireSession(next)

	request := httptest.NewRequest(http.MethodGet, "/api/teams", nil)
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", response.Code)
	}

	token, _ := h.sessions.put(session{ExpiresAt: time.Now().Add(time.Hour)})
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
