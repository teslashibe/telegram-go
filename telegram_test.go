package telegram

import (
	"context"
	"errors"
	"testing"

	"github.com/gotd/td/tg"
)

func TestNewDefaults(t *testing.T) {
	c := New(Config{APIID: 1, APIHash: "h"})
	if c == nil {
		t.Fatal("New returned nil")
	}
	if !c.confirmWrites {
		t.Error("WithRequireConfirm default should be true")
	}
	if c.storeDir == "" {
		t.Error("StoreDir default should be set")
	}
}

func TestRequireConfirm(t *testing.T) {
	c := New(Config{APIID: 1, APIHash: "h"})
	if err := c.requireConfirm(false); !errors.Is(err, ErrConfirmRequired) {
		t.Errorf("expected ErrConfirmRequired, got %v", err)
	}
	if err := c.requireConfirm(true); err != nil {
		t.Errorf("expected nil with confirm=true, got %v", err)
	}
	c2 := New(Config{APIID: 1, APIHash: "h"}, WithRequireConfirm(false))
	if err := c2.requireConfirm(false); err != nil {
		t.Errorf("expected nil with WithRequireConfirm(false), got %v", err)
	}
}

func TestAllowedPeers(t *testing.T) {
	c := New(Config{APIID: 1, APIHash: "h"}, WithAllowedPeers([]string{"@gotd_en", "user:42"}))
	if err := c.recipientAllowed("@gotd_en"); err != nil {
		t.Errorf("@gotd_en should be allowed: %v", err)
	}
	if err := c.recipientAllowed("user:42"); err != nil {
		t.Errorf("user:42 should be allowed: %v", err)
	}
	if err := c.recipientAllowed("user:99"); !errors.Is(err, ErrPeerNotAllowed) {
		t.Errorf("user:99 should be denied, got %v", err)
	}
	// empty/garbage entries are dropped silently and don't disable the allowlist
	c2 := New(Config{APIID: 1, APIHash: "h"}, WithAllowedPeers([]string{"", "!!!"}))
	// list is entirely empty after normalisation -> allowlist disabled
	if err := c2.recipientAllowed("user:1"); err != nil {
		t.Errorf("expected disabled allowlist, got %v", err)
	}
}

func TestValidateConfig(t *testing.T) {
	c := New(Config{}) // missing APIID + APIHash
	if err := c.validateConfig(); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("expected ErrInvalidParams, got %v", err)
	}
}

func TestStatusReturnsHelpAndIsCallableBeforeConnect(t *testing.T) {
	c := New(Config{APIID: 1, APIHash: "h"})
	rep, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status before Connect should not error: %v", err)
	}
	if rep.Connected {
		t.Errorf("Connected should be false before Connect")
	}
	if rep.Authorized {
		t.Errorf("Authorized should be false before Connect")
	}
	if rep.HelpAuth == "" {
		t.Errorf("HelpAuth should be set when not connected")
	}
}

func TestNotConnectedReturnsErr(t *testing.T) {
	c := New(Config{APIID: 1, APIHash: "h"})
	if err := c.withAPI(func(_ *tg.Client) error { return nil }); err == nil {
		t.Error("expected ErrNotConnected when called before Connect")
	}
}
