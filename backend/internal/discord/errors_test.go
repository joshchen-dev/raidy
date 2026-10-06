package discord

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/joshchen-dev/raidy/internal/postgres"
)

func TestUserMessageHidesInternalErrors(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{errors.New("choose a subcommand"), "choose a subcommand"},
		{postgres.ErrDuplicateTeam, postgres.ErrDuplicateTeam.Error()},
		{postgres.ValidationError{Message: "select at least one weekday"}, "select at least one weekday"},
		{fmt.Errorf("create: %w", &pgconn.PgError{Code: "23503", Message: "violates foreign key"}), genericFailure},
		{&pgconn.ConnectError{Config: &pgconn.Config{}}, genericFailure},
		{&discordgo.RESTError{Response: &http.Response{StatusCode: 500}}, genericFailure},
		{context.DeadlineExceeded, timeoutFailure},
	}
	for _, test := range tests {
		if got := userMessage(test.err); got != test.want {
			t.Errorf("userMessage(%v) = %q, want %q", test.err, got, test.want)
		}
	}
}
