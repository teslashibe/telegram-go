package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotd/td/tg"
)

// ResolveContact looks up a peer by @username, t.me link, or canonical
// id and returns its full Peer descriptor (with display name, phone if
// available, etc.). On miss returns ErrNotFound.
func (c *Client) ResolveContact(ctx context.Context, ref string) (Peer, error) {
	norm := NormalizePeerID(ref)
	if norm == "" {
		return Peer{}, fmt.Errorf("%w: %q", ErrInvalidPeer, ref)
	}
	if strings.HasPrefix(norm, "@") {
		var p Peer
		err := c.withAPI(func(api *tg.Client) error {
			resolved, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
				Username: strings.TrimPrefix(norm, "@"),
			})
			if err != nil {
				return err
			}
			c.cache().ingestUsers(resolved.Users)
			c.cache().ingestChats(resolved.Chats)
			switch t := resolved.Peer.(type) {
			case *tg.PeerUser:
				if u, ok := c.cache().user(t.UserID); ok {
					p = c.peerFromTGUser(u)
					return nil
				}
			case *tg.PeerChat:
				if ch, ok := c.cache().chat(t.ChatID); ok {
					p = c.peerFromTGChat(ch)
					return nil
				}
			case *tg.PeerChannel:
				if ch, ok := c.cache().channel(t.ChannelID); ok {
					p = c.peerFromTGChat(ch)
					return nil
				}
			}
			return fmt.Errorf("%w: resolved %q but couldn't extract peer", ErrNotFound, ref)
		})
		if err != nil {
			return Peer{}, err
		}
		return p, nil
	}
	kind, num, _, err := ParsePeerID(norm)
	if err != nil {
		return Peer{}, err
	}
	switch kind {
	case PeerUser:
		if u, ok := c.cache().user(num); ok {
			return c.peerFromTGUser(u), nil
		}
	case PeerChat:
		if ch, ok := c.cache().chat(num); ok {
			return c.peerFromTGChat(ch), nil
		}
	case PeerChannel:
		if ch, ok := c.cache().channel(num); ok {
			return c.peerFromTGChat(ch), nil
		}
	}
	return Peer{}, fmt.Errorf("%w: peer %s unknown to local cache; call ListChats first", ErrNotFound, norm)
}
