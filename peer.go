package telegram

import (
	"fmt"
	"strconv"
	"strings"
)

// NormalizePeerID returns a canonical string form for a peer reference.
//
//   - "@gotd_en"             -> "@gotd_en"
//   - "gotd_en"              -> "@gotd_en"
//   - "https://t.me/gotd_en" -> "@gotd_en"
//   - "user:12345"           -> "user:12345"
//   - "chat:67"              -> "chat:67"
//   - "channel:88"           -> "channel:88"
//
// Returns "" for unrecognisable inputs.
func NormalizePeerID(in string) string {
	in = strings.TrimSpace(in)
	if in == "" {
		return ""
	}
	// t.me URL
	for _, prefix := range []string{"https://t.me/", "http://t.me/", "t.me/"} {
		if strings.HasPrefix(in, prefix) {
			in = strings.TrimPrefix(in, prefix)
			break
		}
	}
	// kind:id pair
	if i := strings.IndexByte(in, ':'); i > 0 {
		kind := strings.ToLower(strings.TrimSpace(in[:i]))
		id := strings.TrimSpace(in[i+1:])
		switch PeerKind(kind) {
		case PeerUser, PeerChat, PeerChannel:
			if _, err := strconv.ParseInt(id, 10, 64); err == nil && id != "" {
				return kind + ":" + id
			}
		}
		return ""
	}
	// "@name" or "name" -> "@name"
	in = strings.TrimPrefix(in, "@")
	in = strings.TrimSuffix(in, "/")
	if in == "" {
		return ""
	}
	if !isValidUsername(in) {
		return ""
	}
	return "@" + in
}

// ParsePeerID splits a canonical id into kind+numeric or username form.
// Returns username with the leading "@".
func ParsePeerID(in string) (kind PeerKind, numericID int64, username string, err error) {
	norm := NormalizePeerID(in)
	if norm == "" {
		err = fmt.Errorf("%w: %q", ErrInvalidPeer, in)
		return
	}
	if strings.HasPrefix(norm, "@") {
		username = norm
		return
	}
	i := strings.IndexByte(norm, ':')
	kind = PeerKind(norm[:i])
	numericID, err = strconv.ParseInt(norm[i+1:], 10, 64)
	if err != nil {
		err = fmt.Errorf("%w: bad numeric id %q", ErrInvalidPeer, norm[i+1:])
	}
	return
}

// FormatPeerID builds a canonical peer ID from kind + numeric id.
func FormatPeerID(kind PeerKind, id int64) string {
	switch kind {
	case PeerUser, PeerChat, PeerChannel:
		return string(kind) + ":" + strconv.FormatInt(id, 10)
	}
	return ""
}

// isValidUsername mirrors Telegram's @username rules conservatively:
// 5–32 chars, must start with a letter, alphanumeric and underscore.
func isValidUsername(s string) bool {
	if len(s) < 5 || len(s) > 32 {
		return false
	}
	if !isLetter(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !isLetter(c) && !isDigit(c) && c != '_' {
			return false
		}
	}
	return true
}

func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
