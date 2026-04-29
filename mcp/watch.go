package mcp

import (
	"context"

	"github.com/teslashibe/mcptool"
	telegram "github.com/teslashibe/telegram-go"
)

type WatchInput struct {
	SinceRowID int64  `json:"since_row_id,omitempty" jsonschema:"description=local-log cursor returned by the previous Watch call"`
	PeerID     string `json:"peer_id,omitempty" jsonschema:"description=optional peer filter"`
	Limit      int    `json:"limit,omitempty" jsonschema:"minimum=1,maximum=500"`
}

func watch(ctx context.Context, c *telegram.Client, in WatchInput) (any, error) {
	return c.Watch(ctx, telegram.WatchParams{
		SinceRowID: in.SinceRowID, PeerID: in.PeerID, Limit: in.Limit,
	})
}

var watchTools = []mcptool.Tool{
	mcptool.Define[*telegram.Client, WatchInput](
		"telegram_watch",
		"Poll the local message log for messages newer than since_row_id; returns a cursor for the next call (never blocks)",
		"Watch",
		watch,
	),
}
