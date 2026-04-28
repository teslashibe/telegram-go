package telegram

import (
	"context"
	"os"
)

// Status returns a snapshot of the client's auth/connection state.
// Returns (StatusReport{...}, nil) even when not authorised — callers
// inspect the booleans to decide what to do.
func (c *Client) Status(ctx context.Context) (StatusReport, error) {
	rep := StatusReport{
		StoreDir: c.storeDir,
	}
	if _, err := os.Stat(c.sessionFile()); err == nil {
		rep.HasSession = true
	}
	c.mu.RLock()
	rep.Connected = c.connected
	self := c.selfUser
	c.mu.RUnlock()

	if self != nil {
		rep.Authorized = true
		rep.SelfID = self.ID
		rep.SelfUsername = self.Username
		rep.SelfFirstName = self.FirstName
	}
	rep.StoredMessages = c.storedMessageCount(ctx)

	if !rep.Connected {
		rep.HelpAuth = "call Connect with WithCodePrompt (and WithPasswordPrompt if 2FA enabled)"
	} else if !rep.Authorized {
		rep.HelpAuth = "session not authorised; reconnect to re-run the SMS/2FA flow"
	}
	return rep, nil
}
