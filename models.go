package telegram

import "time"

// PeerKind identifies the kind of peer a canonical ID refers to.
type PeerKind string

const (
	PeerUser    PeerKind = "user"
	PeerChat    PeerKind = "chat"    // legacy small group
	PeerChannel PeerKind = "channel" // includes supergroups + broadcast channels
)

// MediaKind enumerates supported attachment kinds.
type MediaKind string

const (
	MediaPhoto    MediaKind = "photo"
	MediaVideo    MediaKind = "video"
	MediaAudio    MediaKind = "audio"
	MediaDocument MediaKind = "document"
	MediaSticker  MediaKind = "sticker"
	MediaVoice    MediaKind = "voice"
)

// Peer is the resolved identity of a chat partner (user, group, channel).
type Peer struct {
	ID          string   `json:"id"` // canonical "user:123", "chat:456", "channel:789"
	Kind        PeerKind `json:"kind"`
	NumericID   int64    `json:"numericId"`
	Username    string   `json:"username,omitempty"`
	DisplayName string   `json:"displayName,omitempty"`
	Phone       string   `json:"phone,omitempty"`
	IsBot       bool     `json:"isBot,omitempty"`
	IsSelf      bool     `json:"isSelf,omitempty"`
}

// Chat is a conversation thread (DM, small group, supergroup, channel).
type Chat struct {
	Peer           Peer      `json:"peer"`
	Title          string    `json:"title,omitempty"`
	UnreadCount    int       `json:"unreadCount"`
	LastMessageAt  time.Time `json:"lastMessageAt,omitempty"`
	LastMessageID  int       `json:"lastMessageId,omitempty"`
	Pinned         bool      `json:"pinned,omitempty"`
	Muted          bool      `json:"muted,omitempty"`
}

// Message is a single message stored in (or fetched into) the local log.
type Message struct {
	ID         int       `json:"id"`         // Telegram-side message id (per-peer)
	RowID      int64     `json:"rowId,omitempty"` // local-log monotonic id (for Watch)
	PeerID     string    `json:"peerId"`     // canonical peer id
	SenderID   string    `json:"senderId,omitempty"`
	SenderName string    `json:"senderName,omitempty"`
	IsFromMe   bool      `json:"isFromMe"`
	Body       string    `json:"body,omitempty"`
	MediaKind  MediaKind `json:"mediaKind,omitempty"`
	HasMedia   bool      `json:"hasMedia,omitempty"`
	ReplyToID  int       `json:"replyToId,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	IsRead     bool      `json:"isRead,omitempty"`
}

// StatusReport summarises the current client state.
type StatusReport struct {
	StoreDir       string `json:"storeDir"`
	HasSession     bool   `json:"hasSession"`
	Connected      bool   `json:"connected"`
	Authorized     bool   `json:"authorized"`
	SelfID         int64  `json:"selfId,omitempty"`
	SelfUsername   string `json:"selfUsername,omitempty"`
	SelfFirstName  string `json:"selfFirstName,omitempty"`
	StoredMessages int64  `json:"storedMessages"`
	HelpAuth       string `json:"helpAuth,omitempty"`
}

// --- param structs ---

type ChatListParams struct {
	Limit       int  // default 20, max 200
	OnlyUnread  bool
	OnlyDirect  bool
	OnlyGroups  bool
	OnlyChannels bool
}

type MessageListParams struct {
	PeerID    string
	Limit     int   // default 50, max 200
	OffsetID  int   // Telegram-side: only msgs with id < OffsetID
	MinID     int   // Telegram-side: only msgs with id > MinID (newer than)
	FromLocal bool  // true = read from local log only; false = call MTProto
}

type SearchParams struct {
	Query    string
	PeerID   string // empty = global search
	Limit    int
	OnlyMine *bool
}

type SendParams struct {
	PeerID    string
	Body      string
	ReplyToID int
	Silent    bool
	Confirm   bool
}

type SendMediaParams struct {
	PeerID   string
	FilePath string
	Kind     MediaKind // auto-detected when empty
	Caption  string
	MIMEType string
	Silent   bool
	Confirm  bool
}

type ReactParams struct {
	PeerID    string
	MessageID int
	Emoji     string // empty = remove
	Big       bool   // "big" reaction (premium animation)
	Confirm   bool
}

type MarkReadParams struct {
	PeerID         string
	MaxMessageID   int // mark everything <= this id as read; 0 = mark all
	Confirm        bool
}

type JoinLeaveParams struct {
	PeerID  string // user ignored; chat/channel id or @username
	Confirm bool
}

type WatchParams struct {
	SinceRowID int64
	PeerID     string
	Limit      int
}

type WatchResult struct {
	Messages []Message `json:"messages"`
	Cursor   int64     `json:"cursor"`
}
