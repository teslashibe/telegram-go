package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gotd/td/tg"
)

// GetMessages returns up to params.Limit messages for a peer. When
// params.FromLocal is true the result is served from the local SQLite
// log only (no MTProto round trip). Otherwise the messages come from
// MessagesGetHistory and are written through to the local log.
func (c *Client) GetMessages(ctx context.Context, params MessageListParams) ([]Message, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if NormalizePeerID(params.PeerID) == "" {
		return nil, fmt.Errorf("%w: PeerID required", ErrInvalidParams)
	}
	if params.FromLocal {
		return c.getMessagesLocal(ctx, params.PeerID, limit, params.OffsetID)
	}

	var out []Message
	err := c.withAPI(func(api *tg.Client) error {
		ip, err := c.inputPeer(ctx, params.PeerID)
		if err != nil {
			return err
		}
		req := &tg.MessagesGetHistoryRequest{
			Peer:     ip,
			Limit:    limit,
			OffsetID: params.OffsetID,
			MinID:    params.MinID,
		}
		resp, err := api.MessagesGetHistory(ctx, req)
		if err != nil {
			return fmt.Errorf("MessagesGetHistory: %w", err)
		}
		out = c.convertMessages(ctx, resp, NormalizePeerID(params.PeerID))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Search runs Telegram's server-side search. PeerID empty = global
// search across all dialogs.
func (c *Client) Search(ctx context.Context, params SearchParams) ([]Message, error) {
	if strings.TrimSpace(params.Query) == "" {
		return nil, fmt.Errorf("%w: Query required", ErrInvalidParams)
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var out []Message
	err := c.withAPI(func(api *tg.Client) error {
		if params.PeerID == "" {
			resp, err := api.MessagesSearchGlobal(ctx, &tg.MessagesSearchGlobalRequest{
				Q:          params.Query,
				Filter:     &tg.InputMessagesFilterEmpty{},
				Limit:      limit,
				OffsetPeer: &tg.InputPeerEmpty{},
			})
			if err != nil {
				return fmt.Errorf("MessagesSearchGlobal: %w", err)
			}
			out = c.convertMessages(ctx, resp, "")
			return nil
		}
		ip, err := c.inputPeer(ctx, params.PeerID)
		if err != nil {
			return err
		}
		resp, err := api.MessagesSearch(ctx, &tg.MessagesSearchRequest{
			Peer:   ip,
			Q:      params.Query,
			Filter: &tg.InputMessagesFilterEmpty{},
			Limit:  limit,
		})
		if err != nil {
			return fmt.Errorf("MessagesSearch: %w", err)
		}
		out = c.convertMessages(ctx, resp, NormalizePeerID(params.PeerID))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MarkRead marks all messages in a peer up to MaxMessageID as read
// (0 = mark all).
func (c *Client) MarkRead(ctx context.Context, params MarkReadParams) error {
	if NormalizePeerID(params.PeerID) == "" {
		return fmt.Errorf("%w: PeerID required", ErrInvalidParams)
	}
	if err := c.requireConfirm(params.Confirm); err != nil {
		return err
	}
	if c.dryRun {
		c.logger.Info("dry-run: would MarkRead", logFields(params)...)
		return nil
	}
	err := c.withAPI(func(api *tg.Client) error {
		ip, err := c.inputPeer(ctx, params.PeerID)
		if err != nil {
			return err
		}
		switch p := ip.(type) {
		case *tg.InputPeerChannel:
			_, err := api.ChannelsReadHistory(ctx, &tg.ChannelsReadHistoryRequest{
				Channel: &tg.InputChannel{ChannelID: p.ChannelID, AccessHash: p.AccessHash},
				MaxID:   params.MaxMessageID,
			})
			return err
		default:
			_, err := api.MessagesReadHistory(ctx, &tg.MessagesReadHistoryRequest{
				Peer:  ip,
				MaxID: params.MaxMessageID,
			})
			return err
		}
	})
	if err != nil {
		return err
	}
	c.markChatRead(ctx, NormalizePeerID(params.PeerID))
	return nil
}

// convertMessages flattens a tg.MessagesMessagesClass into our Message
// type, ingesting users/chats into the cache and writing through to
// the local log. peerHint is the canonical peer id when known (per-peer
// history); leave empty for global search.
func (c *Client) convertMessages(ctx context.Context, resp tg.MessagesMessagesClass, peerHint string) []Message {
	var msgs []tg.MessageClass
	var chats []tg.ChatClass
	var users []tg.UserClass
	switch v := resp.(type) {
	case *tg.MessagesMessages:
		msgs, chats, users = v.Messages, v.Chats, v.Users
	case *tg.MessagesMessagesSlice:
		msgs, chats, users = v.Messages, v.Chats, v.Users
	case *tg.MessagesChannelMessages:
		msgs, chats, users = v.Messages, v.Chats, v.Users
	default:
		return nil
	}
	c.cache().ingestUsers(users)
	c.cache().ingestChats(chats)

	userName := map[int64]string{}
	for _, u := range users {
		if uu, ok := u.(*tg.User); ok {
			n := strings.TrimSpace(uu.FirstName + " " + uu.LastName)
			if n == "" {
				n = uu.Username
			}
			userName[uu.ID] = n
		}
	}

	c.mu.RLock()
	selfID := int64(0)
	if c.selfUser != nil {
		selfID = c.selfUser.ID
	}
	c.mu.RUnlock()

	out := make([]Message, 0, len(msgs))
	for _, mm := range msgs {
		m, ok := mm.(*tg.Message)
		if !ok {
			continue
		}
		peer := peerHint
		if peer == "" {
			peer = peerIDFromTGPeer(m.PeerID)
		}
		from, _ := m.GetFromID()
		senderID := peerIDFromTGPeer(from)
		if senderID == "" && peer != "" {
			senderID = peer // 1:1 chat — sender == peer when not from-me
		}
		isFromMe := m.Out
		if !isFromMe && from != nil {
			if pu, ok := from.(*tg.PeerUser); ok && pu.UserID == selfID {
				isFromMe = true
			}
		}
		var senderName string
		if pu, ok := from.(*tg.PeerUser); ok {
			senderName = userName[pu.UserID]
		}
		md, hasMedia := m.GetMedia()
		mk := mediaKindFor(md)
		replyTo := 0
		if rt, ok := m.GetReplyTo(); ok {
			if h, ok := rt.(*tg.MessageReplyHeader); ok {
				replyTo = h.ReplyToMsgID
			}
		}
		msg := Message{
			ID:         m.ID,
			PeerID:     peer,
			SenderID:   senderID,
			SenderName: senderName,
			IsFromMe:   isFromMe,
			Body:       m.Message,
			MediaKind:  mk,
			HasMedia:   hasMedia && md != nil,
			ReplyToID:  replyTo,
			Timestamp:  time.Unix(int64(m.Date), 0).UTC(),
			IsRead:     true, // history fetched is presumed delivered
		}
		out = append(out, msg)
		_ = c.upsertMessage(ctx, msg)
	}
	return out
}

func mediaKindFor(m tg.MessageMediaClass) MediaKind {
	switch v := m.(type) {
	case *tg.MessageMediaPhoto:
		return MediaPhoto
	case *tg.MessageMediaDocument:
		if v.Voice {
			return MediaVoice
		}
		if v.Video {
			return MediaVideo
		}
		if v.Document != nil {
			if doc, ok := v.Document.AsNotEmpty(); ok {
				for _, attr := range doc.Attributes {
					switch attr.(type) {
					case *tg.DocumentAttributeAudio:
						return MediaAudio
					case *tg.DocumentAttributeVideo:
						return MediaVideo
					case *tg.DocumentAttributeSticker:
						return MediaSticker
					}
				}
			}
		}
		return MediaDocument
	}
	return ""
}

// getMessagesLocal serves messages from the SQLite log only.
func (c *Client) getMessagesLocal(ctx context.Context, peerID string, limit, offsetID int) ([]Message, error) {
	c.mu.RLock()
	db := c.logDB
	c.mu.RUnlock()
	if db == nil {
		return nil, ErrNotConnected
	}
	q := `SELECT rowid, msg_id, peer_id, COALESCE(sender_id,''), COALESCE(sender_name,''),
              is_from_me, COALESCE(body,''), COALESCE(media_kind,''), has_media,
              reply_to_id, timestamp, is_read
         FROM messages
        WHERE peer_id = ?`
	args := []any{NormalizePeerID(peerID)}
	if offsetID > 0 {
		q += ` AND msg_id < ?`
		args = append(args, offsetID)
	}
	q += ` ORDER BY msg_id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var ts int64
		var hasMedia, isFromMe, isRead int
		var mk string
		if err := rows.Scan(&m.RowID, &m.ID, &m.PeerID, &m.SenderID, &m.SenderName,
			&isFromMe, &m.Body, &mk, &hasMedia, &m.ReplyToID, &ts, &isRead); err != nil {
			return nil, err
		}
		m.IsFromMe = isFromMe == 1
		m.HasMedia = hasMedia == 1
		m.IsRead = isRead == 1
		m.MediaKind = MediaKind(mk)
		m.Timestamp = time.UnixMilli(ts).UTC()
		out = append(out, m)
	}
	return out, rows.Err()
}

