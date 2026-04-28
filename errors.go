package telegram

import "errors"

var (
	ErrClosed                  = errors.New("telegram: client is closed")
	ErrNotConnected            = errors.New("telegram: client is not connected; call Connect first")
	ErrAlreadyConnected        = errors.New("telegram: client is already connected")
	ErrInvalidParams           = errors.New("telegram: invalid parameters")
	ErrInvalidPeer             = errors.New("telegram: invalid peer (expected @username, t.me/<name>, or user:|chat:|channel:<id>)")
	ErrConfirmRequired         = errors.New("telegram: confirm=true required for write operations")
	ErrPeerNotAllowed          = errors.New("telegram: peer is not in the configured allowlist")
	ErrPasswordPromptMissing   = errors.New("telegram: 2FA password required but no WithPasswordPrompt callback was supplied")
	ErrCodePromptMissing       = errors.New("telegram: SMS code required but no WithCodePrompt callback was supplied")
	ErrPhoneRequired           = errors.New("telegram: Config.Phone is required for first-time auth")
	ErrAuthFailed              = errors.New("telegram: authentication failed")
	ErrPassword2FARequired     = errors.New("telegram: account has 2FA enabled; provide WithPasswordPrompt")
	ErrSendFailed              = errors.New("telegram: send failed")
	ErrNotFound                = errors.New("telegram: not found")
	ErrParseFailed             = errors.New("telegram: failed to parse stored row")
	ErrStoreInit               = errors.New("telegram: failed to initialise local store")
	ErrMessageEmpty            = errors.New("telegram: message body is empty")
	ErrUnsupportedMedia        = errors.New("telegram: unsupported or undownloadable media")
)
