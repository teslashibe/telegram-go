package mcp

import (
	"context"

	"github.com/teslashibe/mcptool"
	telegram "github.com/teslashibe/telegram-go"
)

type StatusInput struct{}

func status(ctx context.Context, c *telegram.Client, _ StatusInput) (any, error) {
	return c.Status(ctx)
}

var statusTools = []mcptool.Tool{
	mcptool.Define[*telegram.Client, StatusInput](
		"telegram_status",
		"Report client state: storage path, session presence, connection + auth status, self user, stored message count",
		"Status",
		status,
	),
}
