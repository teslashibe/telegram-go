package mcp

import (
	"context"

	"github.com/teslashibe/mcptool"
	telegram "github.com/teslashibe/telegram-go"
)

type GetMessagesInput struct {
	PeerID    string `json:"peer_id" jsonschema:"description=canonical peer id (@username\\, t.me/<name>\\, or user:|chat:|channel:<numericId>)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=default 50\\, max 200,minimum=1,maximum=200"`
	OffsetID  int    `json:"offset_id,omitempty" jsonschema:"description=only return messages with id < offset_id (pagination)"`
	MinID     int    `json:"min_id,omitempty" jsonschema:"description=only return messages with id > min_id (newer than)"`
	FromLocal bool   `json:"from_local,omitempty" jsonschema:"description=true = serve from local SQLite log only (no network); false = MTProto"`
}

func getMessages(ctx context.Context, c *telegram.Client, in GetMessagesInput) (any, error) {
	return c.GetMessages(ctx, telegram.MessageListParams{
		PeerID:    in.PeerID,
		Limit:     in.Limit,
		OffsetID:  in.OffsetID,
		MinID:     in.MinID,
		FromLocal: in.FromLocal,
	})
}

type SearchInput struct {
	Query  string `json:"query" jsonschema:"description=full-text search query"`
	PeerID string `json:"peer_id,omitempty" jsonschema:"description=optional peer to scope the search (omit for global)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"minimum=1,maximum=200"`
}

func searchMessages(ctx context.Context, c *telegram.Client, in SearchInput) (any, error) {
	return c.Search(ctx, telegram.SearchParams{
		Query:  in.Query,
		PeerID: in.PeerID,
		Limit:  in.Limit,
	})
}

type MarkReadInput struct {
	PeerID       string `json:"peer_id" jsonschema:"description=canonical peer id"`
	MaxMessageID int    `json:"max_message_id,omitempty" jsonschema:"description=mark messages with id <= max_message_id as read; omit/0 = mark all"`
	Confirm      bool   `json:"confirm" jsonschema:"description=must be true (write operation)"`
}

func markRead(ctx context.Context, c *telegram.Client, in MarkReadInput) (any, error) {
	return nil, c.MarkRead(ctx, telegram.MarkReadParams{
		PeerID:       in.PeerID,
		MaxMessageID: in.MaxMessageID,
		Confirm:      in.Confirm,
	})
}

var messageTools = []mcptool.Tool{
	mcptool.Define[*telegram.Client, GetMessagesInput](
		"telegram_get_messages",
		"Fetch a peer's recent messages (default MTProto; set from_local=true for cached log only)",
		"GetMessages",
		getMessages,
	),
	mcptool.Define[*telegram.Client, SearchInput](
		"telegram_search",
		"Server-side message search; pass peer_id to scope or leave empty for a global search across all dialogs",
		"Search",
		searchMessages,
	),
	mcptool.Define[*telegram.Client, MarkReadInput](
		"telegram_mark_read",
		"Mark messages as read in a peer up to max_message_id (0 = all); confirm-gated",
		"MarkRead",
		markRead,
	),
}
