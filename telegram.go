// Package telegram provides a Go client + MCP tool surface for Telegram
// (user account, not Bot API), built on github.com/gotd/td.
//
// All write-style methods (SendMessage, SendMedia, React, MarkRead,
// JoinChat, LeaveChat) honour two cross-cutting safety knobs:
//
//   - [WithRequireConfirm] (default true) requires an explicit Confirm
//     flag from the caller.
//   - [WithAllowedPeers] denies sends to peers outside an allowlist.
package telegram

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

// Config configures a Client.
type Config struct {
	// APIID and APIHash come from https://my.telegram.org/apps. Both
	// required.
	APIID   int
	APIHash string

	// Phone is the user's phone number in international format
	// (e.g. "+14155551212"). Required for first-time auth; ignored on
	// subsequent runs that load a persisted session.
	Phone string

	// StoreDir holds the gotd session.json and our messages.db.
	// Defaults to $XDG_DATA_HOME/teslashibe/telegram-go (or
	// ~/Library/Application Support/teslashibe/telegram-go on macOS).
	StoreDir string

	// DeviceModel / SystemVersion / AppVersion appear in the user's
	// "Active Sessions" list. Sensible defaults are filled in.
	DeviceModel   string
	SystemVersion string
	AppVersion    string
}

// CodeAuthenticator returns the SMS code Telegram just sent to the user.
type CodeAuthenticator func(ctx context.Context) (string, error)

// PasswordAuthenticator returns the user's 2FA password.
type PasswordAuthenticator func(ctx context.Context) (string, error)

// Client is a Telegram user-account client. Methods are safe for
// concurrent use after Connect returns. Call Close to release resources.
type Client struct {
	apiID         int
	apiHash       string
	phone         string
	storeDir      string
	deviceModel   string
	systemVersion string
	appVersion    string

	codeAuth     CodeAuthenticator
	passwordAuth PasswordAuthenticator

	confirmWrites bool
	dryRun        bool
	allowedPeers  map[string]struct{}

	logger *zap.Logger

	mu          sync.RWMutex
	tg          *telegram.Client
	api         *tg.Client
	cancelRun   context.CancelFunc
	runDone     chan error
	connected   bool
	closed      bool
	logDB       *sql.DB
	selfUser    *tg.User
	peers       *peerCache
}

// New constructs a Client. Connect must be called before any read or
// write tool.
func New(cfg Config, opts ...Option) *Client {
	storeDir := cfg.StoreDir
	if storeDir == "" {
		storeDir = defaultStoreDir()
	}
	c := &Client{
		apiID:         cfg.APIID,
		apiHash:       cfg.APIHash,
		phone:         cfg.Phone,
		storeDir:      storeDir,
		deviceModel:   firstNonEmpty(cfg.DeviceModel, "telegram-go"),
		systemVersion: firstNonEmpty(cfg.SystemVersion, "macOS"),
		appVersion:    firstNonEmpty(cfg.AppVersion, "0.1.0"),
		confirmWrites: true,
		logger:        zap.NewNop(),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Option configures a Client.
type Option func(*Client)

// WithRequireConfirm controls whether write methods require an explicit
// confirm flag. Default: true.
func WithRequireConfirm(require bool) Option {
	return func(c *Client) { c.confirmWrites = require }
}

// WithDryRun makes write methods log + return without hitting MTProto.
// Useful for agent dev. Default: false.
func WithDryRun(dry bool) Option {
	return func(c *Client) { c.dryRun = dry }
}

// WithAllowedPeers restricts write methods to a fixed set of canonical
// peer IDs (e.g. "user:12345", "chat:67890", "channel:111213" or
// "@username"). Pass nil/empty to disable the allowlist.
func WithAllowedPeers(peers []string) Option {
	return func(c *Client) {
		if len(peers) == 0 {
			c.allowedPeers = nil
			return
		}
		m := make(map[string]struct{}, len(peers))
		for _, p := range peers {
			n := NormalizePeerID(p)
			if n == "" {
				continue
			}
			m[n] = struct{}{}
		}
		if len(m) == 0 {
			c.allowedPeers = nil
			return
		}
		c.allowedPeers = m
	}
}

// WithCodePrompt installs the callback used to ask the user for the
// SMS code Telegram sends during first-time auth.
func WithCodePrompt(fn CodeAuthenticator) Option {
	return func(c *Client) {
		if fn != nil {
			c.codeAuth = fn
		}
	}
}

// WithPasswordPrompt installs the callback used to ask the user for
// the 2FA password (cloud password).
func WithPasswordPrompt(fn PasswordAuthenticator) Option {
	return func(c *Client) {
		if fn != nil {
			c.passwordAuth = fn
		}
	}
}

// WithLogger installs a zap logger; default is no-op.
func WithLogger(l *zap.Logger) Option {
	return func(c *Client) {
		if l != nil {
			c.logger = l
		}
	}
}

// Close disconnects (if connected) and releases resources. Safe to call
// multiple times.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	cancel := c.cancelRun
	done := c.runDone
	logDB := c.logDB
	c.cancelRun = nil
	c.runDone = nil
	c.logDB = nil
	c.api = nil
	c.tg = nil
	c.connected = false
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		// Audit fix (H2): bound the wait so a wedged network can't pin
		// Close forever. gotd's Run terminates promptly once runCtx is
		// cancelled; 5s is generous.
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
	if logDB != nil {
		_ = logDB.Close()
	}
	return nil
}

// recipientAllowed enforces WithAllowedPeers. Returns nil when no
// allowlist is configured.
func (c *Client) recipientAllowed(peer string) error {
	if len(c.allowedPeers) == 0 {
		return nil
	}
	if _, ok := c.allowedPeers[NormalizePeerID(peer)]; ok {
		return nil
	}
	return ErrPeerNotAllowed
}

// requireConfirm returns ErrConfirmRequired when the host enforces
// confirm-on-write and the caller didn't pass confirm=true.
func (c *Client) requireConfirm(confirm bool) error {
	if !c.confirmWrites || confirm {
		return nil
	}
	return ErrConfirmRequired
}

// withAPI runs fn with the active *tg.Client under a read lock so
// callers don't need to hold the mutex across the API call.
func (c *Client) withAPI(fn func(api *tg.Client) error) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrClosed
	}
	api := c.api
	c.mu.RUnlock()
	if api == nil {
		return ErrNotConnected
	}
	return fn(api)
}

// sessionFile is the on-disk path for the gotd session blob.
func (c *Client) sessionFile() string {
	return filepath.Join(c.storeDir, "session.json")
}

// sessionStorage returns the gotd session storage backed by sessionFile.
func (c *Client) sessionStorage() session.Storage {
	return &session.FileStorage{Path: c.sessionFile()}
}

// validateConfig returns ErrInvalidParams when required Config fields
// are missing for the operation about to run.
func (c *Client) validateConfig() error {
	if c.apiID == 0 || c.apiHash == "" {
		return fmt.Errorf("%w: APIID and APIHash are required (get from https://my.telegram.org/apps)", ErrInvalidParams)
	}
	return nil
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

func defaultStoreDir() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "teslashibe", "telegram-go")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".telegram-go")
	}
	return filepath.Join(home, "Library", "Application Support", "teslashibe", "telegram-go")
}

