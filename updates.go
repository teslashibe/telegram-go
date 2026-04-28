package telegram

import (
	"context"
	"strings"
	"time"

	"github.com/gotd/td/tg"
)

// installUpdateDispatcher returns a configured *tg.UpdateDispatcher
// that mirrors incoming UpdateNewMessage / UpdateNewChannelMessage
// events into the local SQLite log so Watch can serve them. Audit fix
// for H1: without this, Watch never returned anything because the log
// was only populated by synchronous fetches.
func (c *Client) installUpdateDispatcher() tg.UpdateDispatcher {
	disp := tg.NewUpdateDispatcher()
	disp.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		c.ingestEntities(e)
		c.recordIncomingMessage(ctx, u.GetMessage(), e)
		return nil
	})
	disp.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		c.ingestEntities(e)
		c.recordIncomingMessage(ctx, u.GetMessage(), e)
		return nil
	})
	return disp
}

// ingestEntities pulls user/channel objects out of an Entities map
// (delivered alongside every update) into our peerCache so subsequent
// inputPeer / ResolveContact calls succeed without round-trips.
func (c *Client) ingestEntities(e tg.Entities) {
	cache := c.cache()
	users := make([]tg.UserClass, 0, len(e.Users))
	for _, u := range e.Users {
		users = append(users, u)
	}
	cache.ingestUsers(users)
	chats := make([]tg.ChatClass, 0, len(e.Chats)+len(e.Channels))
	for _, ch := range e.Chats {
		chats = append(chats, ch)
	}
	for _, ch := range e.Channels {
		chats = append(chats, ch)
	}
	cache.ingestChats(chats)
}

// recordIncomingMessage upserts a live message into the local log.
// SenderName lookup uses the entities map to avoid an extra RPC.
func (c *Client) recordIncomingMessage(ctx context.Context, mc tg.MessageClass, e tg.Entities) {
	m, ok := mc.(*tg.Message)
	if !ok {
		return
	}
	peer := peerIDFromTGPeer(m.PeerID)
	if peer == "" {
		return
	}
	from, _ := m.GetFromID()
	senderID := peerIDFromTGPeer(from)
	if senderID == "" {
		senderID = peer
	}
	c.mu.RLock()
	selfID := int64(0)
	if c.selfUser != nil {
		selfID = c.selfUser.ID
	}
	c.mu.RUnlock()
	isFromMe := m.Out
	if !isFromMe {
		if pu, ok := from.(*tg.PeerUser); ok && pu.UserID == selfID {
			isFromMe = true
		}
	}
	var senderName string
	if pu, ok := from.(*tg.PeerUser); ok {
		if u, ok := e.Users[pu.UserID]; ok {
			senderName = strings.TrimSpace(u.FirstName + " " + u.LastName)
			if senderName == "" {
				senderName = u.Username
			}
		}
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
		IsRead:     isFromMe, // outbound messages are presumed delivered/seen
	}
	if err := c.upsertMessage(ctx, msg); err != nil {
		c.logger.Warn("upsertMessage failed for live update")
	}
}
