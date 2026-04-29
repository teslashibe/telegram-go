package telegram

import (
	"context"
	"time"
)

// Watch returns messages newer than params.SinceRowID from the local
// SQLite log (which is populated by the live update dispatcher and by
// any GetMessages / Search call). Use result.Cursor as the next call's
// SinceRowID for stable polling.
//
// When PeerID is set the cursor still advances against the global
// rowid space, so a second call with a different PeerID may skip rows
// — keep PeerID stable across consecutive Watch calls if you want a
// gap-free per-peer feed. Watch never blocks; agents poll on a cadence
// they choose. The result always carries an empty (not nil) Messages
// slice.
func (c *Client) Watch(ctx context.Context, params WatchParams) (WatchResult, error) {
	c.mu.RLock()
	db := c.logDB
	c.mu.RUnlock()
	if db == nil {
		return WatchResult{Messages: []Message{}}, ErrNotConnected
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	q := `SELECT rowid, msg_id, peer_id, COALESCE(sender_id,''), COALESCE(sender_name,''),
              is_from_me, COALESCE(body,''), COALESCE(media_kind,''), has_media,
              reply_to_id, timestamp, is_read
         FROM messages
        WHERE rowid > ?`
	args := []any{params.SinceRowID}
	if params.PeerID != "" {
		q += ` AND peer_id = ?`
		args = append(args, NormalizePeerID(params.PeerID))
	}
	q += ` ORDER BY rowid ASC LIMIT ?`
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return WatchResult{Messages: []Message{}}, err
	}
	defer rows.Close()
	out := WatchResult{Messages: []Message{}, Cursor: params.SinceRowID}
	for rows.Next() {
		var m Message
		var ts int64
		var hasMedia, isFromMe, isRead int
		var mk string
		if err := rows.Scan(&m.RowID, &m.ID, &m.PeerID, &m.SenderID, &m.SenderName,
			&isFromMe, &m.Body, &mk, &hasMedia, &m.ReplyToID, &ts, &isRead); err != nil {
			return out, err
		}
		m.IsFromMe = isFromMe == 1
		m.HasMedia = hasMedia == 1
		m.IsRead = isRead == 1
		m.MediaKind = MediaKind(mk)
		m.Timestamp = time.UnixMilli(ts).UTC()
		out.Messages = append(out.Messages, m)
		if m.RowID > out.Cursor {
			out.Cursor = m.RowID
		}
	}
	return out, rows.Err()
}
