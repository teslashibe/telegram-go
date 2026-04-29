package mcp

import (
	"context"

	"github.com/teslashibe/mcptool"
	telegram "github.com/teslashibe/telegram-go"
)

type ResolveContactInput struct {
	Ref string `json:"ref" jsonschema:"description=@username\\, t.me/<name>\\, or canonical user:|chat:|channel:<id>"`
}

func resolveContact(ctx context.Context, c *telegram.Client, in ResolveContactInput) (any, error) {
	return c.ResolveContact(ctx, in.Ref)
}

var contactTools = []mcptool.Tool{
	mcptool.Define[*telegram.Client, ResolveContactInput](
		"telegram_resolve_contact",
		"Resolve @username / t.me link / kind:id to a Peer (id, kind, display name, username, phone)",
		"ResolveContact",
		resolveContact,
	),
}
