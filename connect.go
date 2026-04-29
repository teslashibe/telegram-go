package telegram

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// Connect brings up the MTProto session, authenticating if needed,
// and keeps it alive until Close (or the supplied ctx) is cancelled.
//
// First-time auth requires Config.Phone plus a WithCodePrompt callback;
// 2FA accounts also need WithPasswordPrompt. Subsequent runs reuse the
// persisted session.json.
//
// Returns nil once the client is ready for use. Calling Connect again
// while already connected returns ErrAlreadyConnected.
func (c *Client) Connect(ctx context.Context) error {
	if err := c.validateConfig(); err != nil {
		return err
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrClosed
	}
	if c.connected {
		c.mu.Unlock()
		return ErrAlreadyConnected
	}
	c.mu.Unlock()

	if err := c.initLogStore(ctx); err != nil {
		return err
	}
	if err := os.MkdirAll(c.storeDir, 0o700); err != nil {
		return fmt.Errorf("%w: mkdir %s: %v", ErrStoreInit, c.storeDir, err)
	}

	dispatcher := c.installUpdateDispatcher()
	tgClient := telegram.NewClient(c.apiID, c.apiHash, telegram.Options{
		SessionStorage: c.sessionStorage(),
		Logger:         c.logger.Named("gotd"),
		UpdateHandler:  dispatcher,
		Device: telegram.DeviceConfig{
			DeviceModel:   c.deviceModel,
			SystemVersion: c.systemVersion,
			AppVersion:    c.appVersion,
		},
	})

	runCtx, cancel := context.WithCancel(context.Background())
	ready := make(chan error, 1)
	done := make(chan error, 1)

	go func() {
		done <- tgClient.Run(runCtx, func(ctx context.Context) error {
			c.mu.Lock()
			c.tg = tgClient
			c.api = tgClient.API()
			c.connected = true
			c.mu.Unlock()

			if err := c.ensureAuthorized(ctx, tgClient); err != nil {
				ready <- err
				return err
			}

			if self, err := tgClient.Self(ctx); err == nil {
				c.mu.Lock()
				c.selfUser = self
				c.mu.Unlock()
			}

			ready <- nil
			<-ctx.Done()
			return ctx.Err()
		})
		c.mu.Lock()
		c.connected = false
		c.api = nil
		c.tg = nil
		c.mu.Unlock()
	}()

	select {
	case err := <-ready:
		if err != nil {
			cancel()
			<-done
			return err
		}
		c.mu.Lock()
		c.cancelRun = cancel
		c.runDone = done
		c.mu.Unlock()
		return nil
	case err := <-done:
		cancel()
		if err == nil {
			return errors.New("telegram: gotd Run exited before becoming ready")
		}
		return err
	case <-ctx.Done():
		cancel()
		<-done
		return ctx.Err()
	}
}

// Disconnect tears down the live MTProto connection but keeps session
// state on disk (so the next Connect doesn't re-prompt for a code).
// Safe to call when not connected.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	cancel := c.cancelRun
	done := c.runDone
	c.cancelRun = nil
	c.runDone = nil
	c.connected = false
	c.api = nil
	c.tg = nil
	c.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	if done != nil {
		<-done
	}
	return nil
}

// ensureAuthorized inspects the session and runs the auth flow if the
// user is not already authorised.
func (c *Client) ensureAuthorized(ctx context.Context, client *telegram.Client) error {
	status, err := client.Auth().Status(ctx)
	if err != nil {
		return fmt.Errorf("%w: status: %v", ErrAuthFailed, err)
	}
	if status.Authorized {
		return nil
	}
	if c.phone == "" {
		return ErrPhoneRequired
	}
	if c.codeAuth == nil {
		return ErrCodePromptMissing
	}
	flow := auth.NewFlow(
		userAuthenticator{c: c},
		auth.SendCodeOptions{},
	)
	if err := flow.Run(ctx, client.Auth()); err != nil {
		// gotd surfaces the missing-password case as auth.ErrPasswordNotProvided.
		if errors.Is(err, auth.ErrPasswordNotProvided) {
			return ErrPassword2FARequired
		}
		return fmt.Errorf("%w: %v", ErrAuthFailed, err)
	}
	return nil
}

// userAuthenticator adapts our Config + prompt callbacks into gotd's
// auth.UserAuthenticator interface. SignUp is intentionally rejected
// — agents shouldn't be silently registering new Telegram accounts.
type userAuthenticator struct {
	c *Client
}

func (u userAuthenticator) Phone(ctx context.Context) (string, error) {
	if u.c.phone == "" {
		return "", ErrPhoneRequired
	}
	return u.c.phone, nil
}

func (u userAuthenticator) Password(ctx context.Context) (string, error) {
	if u.c.passwordAuth == nil {
		return "", ErrPassword2FARequired
	}
	return u.c.passwordAuth(ctx)
}

func (u userAuthenticator) Code(ctx context.Context, _ *tg.AuthSentCode) (string, error) {
	if u.c.codeAuth == nil {
		return "", ErrCodePromptMissing
	}
	return u.c.codeAuth(ctx)
}

func (u userAuthenticator) AcceptTermsOfService(ctx context.Context, _ tg.HelpTermsOfService) error {
	return errors.New("telegram: refusing to accept ToS automatically — sign up via the official client first")
}

func (u userAuthenticator) SignUp(ctx context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("telegram: refusing to sign up new account; use the official client")
}
