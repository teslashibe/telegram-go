package mcp

import (
	"context"

	"github.com/teslashibe/mcptool"
	telegram "github.com/teslashibe/telegram-go"
)

type ReactInput struct {
	PeerID    string `json:"peer_id" jsonschema:"description=canonical peer id"`
	MessageID int    `json:"message_id" jsonschema:"description=Telegram-side message id (per-peer)"`
	Emoji     string `json:"emoji,omitempty" jsonschema:"description=reaction emoji (e.g. \\\"👍\\\"); empty = remove existing reaction"`
	Big       bool   `json:"big,omitempty" jsonschema:"description=play the big reaction animation (premium-only)"`
	Confirm   bool   `json:"confirm" jsonschema:"description=must be true (write operation)"`
}

func react(ctx context.Context, c *telegram.Client, in ReactInput) (any, error) {
	return nil, c.React(ctx, telegram.ReactParams{
		PeerID: in.PeerID, MessageID: in.MessageID,
		Emoji: in.Emoji, Big: in.Big, Confirm: in.Confirm,
	})
}

var reactTools = []mcptool.Tool{
	mcptool.Define[*telegram.Client, ReactInput](
		"telegram_react",
		"Add or remove an emoji reaction on a message; confirm-gated",
		"React",
		react,
	),
}
