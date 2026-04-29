package telegram

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/gotd/td/tg"
)

// ListChats returns the dialog list (DMs, groups, channels) ordered by
// most recent activity. Telegram's dialog API is paginated; this method
// hides the cursor by fetching up to params.Limit records (capped at 200).
func (c *Client) ListChats(ctx context.Context, params ChatListParams) ([]Chat, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	var out []Chat
	err := c.withAPI(func(api *tg.Client) error {
		resp, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			OffsetPeer: &tg.InputPeerEmpty{},
			Limit:      limit,
		})
		if err != nil {
			return fmt.Errorf("MessagesGetDialogs: %w", err)
		}
		var dialogs []tg.DialogClass
		var msgs []tg.MessageClass
		var chats []tg.ChatClass
		var users []tg.UserClass
		switch d := resp.(type) {
		case *tg.MessagesDialogs:
			dialogs, msgs, chats, users = d.Dialogs, d.Messages, d.Chats, d.Users
		case *tg.MessagesDialogsSlice:
			dialogs, msgs, chats, users = d.Dialogs, d.Messages, d.Chats, d.Users
		default:
			return fmt.Errorf("%w: unexpected dialogs response %T", ErrParseFailed, resp)
		}
		c.cache().ingestUsers(users)
		c.cache().ingestChats(chats)

		userByID := map[int64]*tg.User{}
		for _, u := range users {
			if uu, ok := u.(*tg.User); ok {
				userByID[uu.ID] = uu
			}
		}
		chatByID := map[int64]tg.ChatClass{}
		for _, ch := range chats {
			switch v := ch.(type) {
			case *tg.Chat:
				chatByID[v.ID] = v
			case *tg.Channel:
				chatByID[v.ID] = v
			}
		}
		topMsg := map[string]tg.MessageClass{}
		for _, m := range msgs {
			peer := messagePeer(m)
			if peer != "" {
				topMsg[peer] = m
			}
		}

		for _, dlg := range dialogs {
			d, ok := dlg.(*tg.Dialog)
			if !ok {
				continue
			}
			peer := peerIDFromTGPeer(d.GetPeer())
			if peer == "" {
				continue
			}
			chat := Chat{
				UnreadCount: d.GetUnreadCount(),
				Pinned:      d.GetPinned(),
			}
			switch p := d.GetPeer().(type) {
			case *tg.PeerUser:
				if u, ok := userByID[p.UserID]; ok {
					chat.Peer = c.peerFromTGUser(u)
					chat.Title = chat.Peer.DisplayName
				} else {
					chat.Peer = Peer{ID: peer, Kind: PeerUser, NumericID: p.UserID}
				}
			case *tg.PeerChat:
				if cc, ok := chatByID[p.ChatID]; ok {
					chat.Peer = c.peerFromTGChat(cc)
					chat.Title = chat.Peer.DisplayName
				} else {
					chat.Peer = Peer{ID: peer, Kind: PeerChat, NumericID: p.ChatID}
				}
			case *tg.PeerChannel:
				if cc, ok := chatByID[p.ChannelID]; ok {
					chat.Peer = c.peerFromTGChat(cc)
					chat.Title = chat.Peer.DisplayName
				} else {
					chat.Peer = Peer{ID: peer, Kind: PeerChannel, NumericID: p.ChannelID}
				}
			}
			if tm, ok := topMsg[peer]; ok {
				chat.LastMessageID = tm.GetID()
				if mm, ok := tm.(*tg.Message); ok {
					chat.LastMessageAt = time.Unix(int64(mm.Date), 0).UTC()
				}
			}
			if !applyChatFilter(chat, params) {
				continue
			}
			out = append(out, chat)
			c.upsertChatMeta(ctx, chat)
		}
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].LastMessageAt.After(out[j].LastMessageAt)
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func applyChatFilter(c Chat, p ChatListParams) bool {
	if p.OnlyUnread && c.UnreadCount == 0 {
		return false
	}
	if p.OnlyDirect && c.Peer.Kind != PeerUser {
		return false
	}
	if p.OnlyGroups && c.Peer.Kind != PeerChat && c.Peer.Kind != PeerChannel {
		return false
	}
	if p.OnlyChannels && c.Peer.Kind != PeerChannel {
		return false
	}
	return true
}

func messagePeer(m tg.MessageClass) string {
	switch v := m.(type) {
	case *tg.Message:
		return peerIDFromTGPeer(v.PeerID)
	case *tg.MessageService:
		return peerIDFromTGPeer(v.PeerID)
	}
	return ""
}
