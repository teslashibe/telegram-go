package mcp

import (
	"context"

	"github.com/teslashibe/mcptool"
	telegram "github.com/teslashibe/telegram-go"
)

type ListChatsInput struct {
	Limit        int  `json:"limit,omitempty" jsonschema:"description=number of chats to return (default 20\\, max 200),minimum=1,maximum=200"`
	OnlyUnread   bool `json:"only_unread,omitempty"`
	OnlyDirect   bool `json:"only_direct,omitempty" jsonschema:"description=only 1:1 user chats"`
	OnlyGroups   bool `json:"only_groups,omitempty" jsonschema:"description=only groups + supergroups"`
	OnlyChannels bool `json:"only_channels,omitempty" jsonschema:"description=only broadcast channels"`
}

func listChats(ctx context.Context, c *telegram.Client, in ListChatsInput) (any, error) {
	return c.ListChats(ctx, telegram.ChatListParams{
		Limit:        in.Limit,
		OnlyUnread:   in.OnlyUnread,
		OnlyDirect:   in.OnlyDirect,
		OnlyGroups:   in.OnlyGroups,
		OnlyChannels: in.OnlyChannels,
	})
}

var chatTools = []mcptool.Tool{
	mcptool.Define[*telegram.Client, ListChatsInput](
		"telegram_list_chats",
		"List Telegram dialogs (DMs\\, groups\\, channels) most-recently-active first; supports unread/direct/group/channel filters",
		"ListChats",
		listChats,
	),
}
