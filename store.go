package telegram

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// initLogStore opens (or creates) the local message-log SQLite at
// <storeDir>/messages.db and runs the schema migrations. Idempotent.
func (c *Client) initLogStore(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.logDB != nil {
		return nil
	}
	if err := os.MkdirAll(c.storeDir, 0o700); err != nil {
		return fmt.Errorf("%w: mkdir %s: %v", ErrStoreInit, c.storeDir, err)
	}
	dsn := "file:" + filepath.Join(c.storeDir, "messages.db") +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(2000)&_pragma=foreign_keys(on)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("%w: open: %v", ErrStoreInit, err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("%w: ping: %v", ErrStoreInit, err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return fmt.Errorf("%w: migrate: %v", ErrStoreInit, err)
	}
	c.logDB = db
	return nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS messages (
    rowid       INTEGER PRIMARY KEY AUTOINCREMENT,
    msg_id      INTEGER NOT NULL,
    peer_id     TEXT NOT NULL,
    sender_id   TEXT,
    sender_name TEXT,
    is_from_me  INTEGER NOT NULL,
    body        TEXT,
    media_kind  TEXT,
    has_media   INTEGER NOT NULL DEFAULT 0,
    reply_to_id INTEGER,
    timestamp   INTEGER NOT NULL,
    is_read     INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL,
    UNIQUE(peer_id, msg_id)
);
CREATE INDEX IF NOT EXISTS idx_msg_peer_ts ON messages(peer_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_msg_body    ON messages(body);

CREATE TABLE IF NOT EXISTS chats (
    peer_id      TEXT PRIMARY KEY,
    kind         TEXT NOT NULL,
    title        TEXT,
    unread_count INTEGER NOT NULL DEFAULT 0,
    last_msg_at  INTEGER
);`
	_, err := db.ExecContext(ctx, schema)
	return err
}

// upsertMessage inserts (or refreshes) a message row, idempotent on
// (peer_id, msg_id). Touches the chats row for last_msg_at.
func (c *Client) upsertMessage(ctx context.Context, m Message) error {
	c.mu.RLock()
	db := c.logDB
	c.mu.RUnlock()
	if db == nil {
		return nil
	}
	const q = `
INSERT INTO messages
  (msg_id, peer_id, sender_id, sender_name, is_from_me, body, media_kind, has_media, reply_to_id, timestamp, is_read, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(peer_id, msg_id) DO UPDATE SET
  body        = excluded.body,
  media_kind  = excluded.media_kind,
  has_media   = excluded.has_media,
  is_read     = MAX(messages.is_read, excluded.is_read),
  reply_to_id = excluded.reply_to_id`
	hasMedia := 0
	if m.HasMedia {
		hasMedia = 1
	}
	isRead := 0
	if m.IsRead {
		isRead = 1
	}
	if _, err := db.ExecContext(ctx, q,
		m.ID, m.PeerID, nullable(m.SenderID), nullable(m.SenderName),
		boolToInt(m.IsFromMe), nullable(m.Body), nullable(string(m.MediaKind)),
		hasMedia, m.ReplyToID, m.Timestamp.UnixMilli(), isRead,
		time.Now().UnixMilli(),
	); err != nil {
		return err
	}
	const upsertChat = `
INSERT INTO chats (peer_id, kind, title, last_msg_at, unread_count)
VALUES (?, ?, '', ?, ?)
ON CONFLICT(peer_id) DO UPDATE SET
  last_msg_at = MAX(COALESCE(last_msg_at, 0), excluded.last_msg_at),
  unread_count = unread_count + excluded.unread_count`
	kind, _, _, _ := ParsePeerID(m.PeerID)
	if kind == "" {
		kind = PeerUser
	}
	unread := 0
	if !m.IsFromMe && !m.IsRead {
		unread = 1
	}
	_, err := db.ExecContext(ctx, upsertChat,
		m.PeerID, string(kind), m.Timestamp.UnixMilli(), unread,
	)
	return err
}

// upsertChatMeta records / refreshes display metadata for a chat.
func (c *Client) upsertChatMeta(ctx context.Context, chat Chat) {
	c.mu.RLock()
	db := c.logDB
	c.mu.RUnlock()
	if db == nil {
		return
	}
	_, _ = db.ExecContext(ctx, `
INSERT INTO chats (peer_id, kind, title, unread_count, last_msg_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(peer_id) DO UPDATE SET
  kind = excluded.kind,
  title = CASE WHEN excluded.title <> '' THEN excluded.title ELSE chats.title END,
  unread_count = excluded.unread_count,
  last_msg_at = MAX(COALESCE(chats.last_msg_at, 0), COALESCE(excluded.last_msg_at, 0))`,
		chat.Peer.ID, string(chat.Peer.Kind), chat.Title, chat.UnreadCount,
		chat.LastMessageAt.UnixMilli(),
	)
}

// markChatRead clears the unread counter for a peer.
func (c *Client) markChatRead(ctx context.Context, peerID string) {
	c.mu.RLock()
	db := c.logDB
	c.mu.RUnlock()
	if db == nil {
		return
	}
	_, _ = db.ExecContext(ctx, `UPDATE chats SET unread_count = 0 WHERE peer_id = ?`, peerID)
	_, _ = db.ExecContext(ctx, `UPDATE messages SET is_read = 1 WHERE peer_id = ?`, peerID)
}

// storedMessageCount is used by Status.
func (c *Client) storedMessageCount(ctx context.Context) int64 {
	c.mu.RLock()
	db := c.logDB
	c.mu.RUnlock()
	if db == nil {
		return 0
	}
	var n int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages`).Scan(&n); err != nil {
		return 0
	}
	return n
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
