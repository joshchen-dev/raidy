package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"

	raidydiscord "github.com/joshchen-dev/raidy/internal/discord"
	"github.com/joshchen-dev/raidy/internal/postgres"
	raidyweb "github.com/joshchen-dev/raidy/internal/web"
	"github.com/joshchen-dev/raidy/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("raidy stopped", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	token, databaseURL := os.Getenv("DISCORD_TOKEN"), os.Getenv("DATABASE_URL")
	if token == "" || databaseURL == "" {
		return errors.New("DISCORD_TOKEN and DATABASE_URL are required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := migrations.Up(ctx, databaseURL); err != nil {
		return err
	}
	store, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return err
	}
	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers
	bot := raidydiscord.New(session, store, log)
	session.AddHandler(bot.Handle)
	if err := session.Open(); err != nil {
		return err
	}
	defer session.Close()

	if _, err := session.ApplicationCommandBulkOverwrite(session.State.User.ID, os.Getenv("DISCORD_GUILD_ID"), raidydiscord.Commands()); err != nil {
		return err
	}

	address := os.Getenv("HTTP_ADDR")
	if address == "" {
		address = ":8080"
	}
	webHandler, err := raidyweb.New(store, session, bot, log, raidyweb.Config{
		ClientID: os.Getenv("DISCORD_CLIENT_ID"), ClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
		BaseURL: os.Getenv("APP_BASE_URL"), StaticDir: os.Getenv("WEB_DIST_DIR"),
	})
	if err != nil {
		return err
	}
	server := &http.Server{Addr: address, Handler: webHandler.Routes(healthHandler(store)), ReadHeaderTimeout: 5 * time.Second}
	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	go bot.RunScheduler(ctx)
	log.Info("raidy started", "http_addr", address)

	select {
	case <-ctx.Done():
	case err := <-serverErr:
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

type pinger interface {
	Ping(context.Context) error
}

func healthHandler(store pinger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
	})
	return mux
}
