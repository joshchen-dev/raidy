package discord

import (
	"context"
	"errors"
	"net"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	genericFailure = "Something went wrong. Try again in a moment."
	timeoutFailure = "That took too long. Try again in a moment."
)

// userMessage returns text that is safe to show in Discord. Errors created in
// this codebase are written for users; database, network, and Discord REST
// failures are logged instead of being echoed back.
func userMessage(err error) string {
	var pgErr *pgconn.PgError
	var connectErr *pgconn.ConnectError
	var restErr *discordgo.RESTError
	var netErr net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return timeoutFailure
	case errors.As(err, &pgErr), errors.As(err, &connectErr), errors.As(err, &restErr), errors.As(err, &netErr):
		return genericFailure
	}
	return err.Error()
}
