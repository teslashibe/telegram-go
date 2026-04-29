package mcp

import (
	"context"

	"github.com/teslashibe/mcptool"
	telegram "github.com/teslashibe/telegram-go"
)

type SendMessageInput struct {
	PeerID    string `json:"peer_id" jsonschema:"description=canonical peer id (@username\\, t.me/<name>\\, or user:|chat:|channel:<id>)"`
	Body      string `json:"body" jsonschema:"description=plain-text message body"`
	ReplyToID int    `json:"reply_to_id,omitempty" jsonschema:"description=optional message id to reply to"`
	Silent    bool   `json:"silent,omitempty" jsonschema:"description=suppress notification on the recipient"`
	Confirm   bool   `json:"confirm" jsonschema:"description=must be true (write operation)"`
}

func sendMessage(ctx context.Context, c *telegram.Client, in SendMessageInput) (any, error) {
	return nil, c.SendMessage(ctx, telegram.SendParams{
		PeerID: in.PeerID, Body: in.Body, ReplyToID: in.ReplyToID,
		Silent: in.Silent, Confirm: in.Confirm,
	})
}

type SendMediaInput struct {
	PeerID   string `json:"peer_id" jsonschema:"description=canonical peer id"`
	FilePath string `json:"file_path" jsonschema:"description=absolute path to a local file to upload"`
	Kind     string `json:"kind,omitempty" jsonschema:"description=force a media kind; auto-detected when empty,enum=photo,enum=video,enum=audio,enum=document,enum=sticker,enum=voice"`
	Caption  string `json:"caption,omitempty" jsonschema:"description=optional caption sent as a follow-up message"`
	MIMEType string `json:"mime_type,omitempty"`
	Silent   bool   `json:"silent,omitempty"`
	Confirm  bool   `json:"confirm" jsonschema:"description=must be true (write operation)"`
}

func sendMedia(ctx context.Context, c *telegram.Client, in SendMediaInput) (any, error) {
	return nil, c.SendMedia(ctx, telegram.SendMediaParams{
		PeerID: in.PeerID, FilePath: in.FilePath, Kind: telegram.MediaKind(in.Kind),
		Caption: in.Caption, MIMEType: in.MIMEType,
		Silent: in.Silent, Confirm: in.Confirm,
	})
}

var sendTools = []mcptool.Tool{
	mcptool.Define[*telegram.Client, SendMessageInput](
		"telegram_send_message",
		"Send a text message; supports reply_to_id and silent delivery; confirm-gated and allowlist-checked",
		"SendMessage",
		sendMessage,
	),
	mcptool.Define[*telegram.Client, SendMediaInput](
		"telegram_send_media",
		"Upload + send a local file (photo/video/audio/document/sticker/voice); confirm-gated and allowlist-checked",
		"SendMedia",
		sendMedia,
	),
}
