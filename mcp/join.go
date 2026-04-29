package mcp

import (
	"context"

	"github.com/teslashibe/mcptool"
	telegram "github.com/teslashibe/telegram-go"
)

type JoinLeaveInput struct {
	PeerID  string `json:"peer_id" jsonschema:"description=channel/supergroup id or @username (1:1 user peers are not joinable)"`
	Confirm bool   `json:"confirm" jsonschema:"description=must be true (write operation)"`
}

func joinChat(ctx context.Context, c *telegram.Client, in JoinLeaveInput) (any, error) {
	return nil, c.JoinChat(ctx, telegram.JoinLeaveParams{PeerID: in.PeerID, Confirm: in.Confirm})
}

func leaveChat(ctx context.Context, c *telegram.Client, in JoinLeaveInput) (any, error) {
	return nil, c.LeaveChat(ctx, telegram.JoinLeaveParams{PeerID: in.PeerID, Confirm: in.Confirm})
}

var joinTools = []mcptool.Tool{
	mcptool.Define[*telegram.Client, JoinLeaveInput](
		"telegram_join_chat",
		"Join a public channel/supergroup by @username or canonical channel id; confirm-gated",
		"JoinChat",
		joinChat,
	),
	mcptool.Define[*telegram.Client, JoinLeaveInput](
		"telegram_leave_chat",
		"Leave a channel/supergroup; confirm-gated",
		"LeaveChat",
		leaveChat,
	),
}
