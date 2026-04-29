package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gotd/td/tg"
)

// peerCache stores access_hashes harvested from dialog / message
// responses so we can build InputPeer{User,Channel} for arbitrary
// numeric IDs without round-tripping every time.
type peerCache struct {
	mu       sync.RWMutex
	users    map[int64]*tg.User
	channels map[int64]*tg.Channel
	chats    map[int64]*tg.Chat
}

func newPeerCache() *peerCache {
	return &peerCache{
		users:    map[int64]*tg.User{},
		channels: map[int64]*tg.Channel{},
		chats:    map[int64]*tg.Chat{},
	}
}

func (p *peerCache) ingestUsers(users []tg.UserClass) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, u := range users {
		if uu, ok := u.(*tg.User); ok {
			p.users[uu.ID] = uu
		}
	}
}

func (p *peerCache) ingestChats(chats []tg.ChatClass) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, ch := range chats {
		switch v := ch.(type) {
		case *tg.Chat:
			p.chats[v.ID] = v
		case *tg.Channel:
			p.channels[v.ID] = v
		}
	}
}

func (p *peerCache) user(id int64) (*tg.User, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	u, ok := p.users[id]
	return u, ok
}

func (p *peerCache) channel(id int64) (*tg.Channel, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	c, ok := p.channels[id]
	return c, ok
}

func (p *peerCache) chat(id int64) (*tg.Chat, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	c, ok := p.chats[id]
	return c, ok
}

// peerCache singleton lives on the Client; lazily allocated.
func (c *Client) cache() *peerCache {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.peers == nil {
		c.peers = newPeerCache()
	}
	return c.peers
}

// inputPeer turns a canonical peer id (or @username) into a tg.InputPeerClass.
// Hits the cache first; falls back to ContactsResolveUsername for usernames
// that aren't yet known. Numeric peers must be cached (typically via a prior
// ListChats call) — Telegram's MTProto API does not expose access_hash by
// numeric id without context.
func (c *Client) inputPeer(ctx context.Context, peerID string) (tg.InputPeerClass, error) {
	kind, num, username, err := ParsePeerID(peerID)
	if err != nil {
		return nil, err
	}
	if username != "" {
		return c.resolveUsername(ctx, strings.TrimPrefix(username, "@"))
	}
	cache := c.cache()
	switch kind {
	case PeerUser:
		if u, ok := cache.user(num); ok {
			return &tg.InputPeerUser{UserID: u.ID, AccessHash: u.AccessHash}, nil
		}
	case PeerChat:
		if _, ok := cache.chat(num); ok {
			return &tg.InputPeerChat{ChatID: num}, nil
		}
		// Legacy chats have no access hash; we can try optimistically.
		return &tg.InputPeerChat{ChatID: num}, nil
	case PeerChannel:
		if ch, ok := cache.channel(num); ok {
			return &tg.InputPeerChannel{ChannelID: ch.ID, AccessHash: ch.AccessHash}, nil
		}
	}
	return nil, fmt.Errorf("%w: peer %s not in cache; call ListChats or ResolveContact first", ErrNotFound, peerID)
}

func (c *Client) resolveUsername(ctx context.Context, username string) (tg.InputPeerClass, error) {
	var ip tg.InputPeerClass
	err := c.withAPI(func(api *tg.Client) error {
		resolved, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: username})
		if err != nil {
			return err
		}
		c.cache().ingestUsers(resolved.Users)
		c.cache().ingestChats(resolved.Chats)
		switch p := resolved.Peer.(type) {
		case *tg.PeerUser:
			if u, ok := c.cache().user(p.UserID); ok {
				ip = &tg.InputPeerUser{UserID: u.ID, AccessHash: u.AccessHash}
				return nil
			}
		case *tg.PeerChat:
			ip = &tg.InputPeerChat{ChatID: p.ChatID}
			return nil
		case *tg.PeerChannel:
			if ch, ok := c.cache().channel(p.ChannelID); ok {
				ip = &tg.InputPeerChannel{ChannelID: ch.ID, AccessHash: ch.AccessHash}
				return nil
			}
		}
		return fmt.Errorf("%w: resolved username %q but couldn't form InputPeer", ErrNotFound, username)
	})
	if err != nil {
		return nil, err
	}
	return ip, nil
}

// peerIDFromTGPeer maps a tg.PeerClass to our canonical "kind:id" form.
func peerIDFromTGPeer(p tg.PeerClass) string {
	switch v := p.(type) {
	case *tg.PeerUser:
		return FormatPeerID(PeerUser, v.UserID)
	case *tg.PeerChat:
		return FormatPeerID(PeerChat, v.ChatID)
	case *tg.PeerChannel:
		return FormatPeerID(PeerChannel, v.ChannelID)
	}
	return ""
}

// peerFromTGUser builds a Peer from a *tg.User (after ingesting it).
func (c *Client) peerFromTGUser(u *tg.User) Peer {
	c.cache().ingestUsers([]tg.UserClass{u})
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		name = u.Username
	}
	return Peer{
		ID:          FormatPeerID(PeerUser, u.ID),
		Kind:        PeerUser,
		NumericID:   u.ID,
		Username:    u.Username,
		DisplayName: name,
		Phone:       u.Phone,
		IsBot:       u.Bot,
		IsSelf:      u.Self,
	}
}

func (c *Client) peerFromTGChat(ch tg.ChatClass) Peer {
	switch v := ch.(type) {
	case *tg.Chat:
		c.cache().ingestChats([]tg.ChatClass{v})
		return Peer{
			ID:          FormatPeerID(PeerChat, v.ID),
			Kind:        PeerChat,
			NumericID:   v.ID,
			DisplayName: v.Title,
		}
	case *tg.Channel:
		c.cache().ingestChats([]tg.ChatClass{v})
		kind := PeerChannel
		return Peer{
			ID:          FormatPeerID(kind, v.ID),
			Kind:        kind,
			NumericID:   v.ID,
			Username:    v.Username,
			DisplayName: v.Title,
		}
	}
	return Peer{}
}
