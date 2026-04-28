package telegram

import (
	"path/filepath"
	"testing"
)

// TestGuessMediaKind asserts the kind heuristic so callers don't end
// up uploading photos as documents (and vice versa) on a typo.
func TestGuessMediaKind(t *testing.T) {
	cases := []struct {
		path string
		mime string
		want MediaKind
	}{
		{"a.jpg", "", MediaPhoto},
		{"a.JPEG", "", MediaPhoto},
		{"a.png", "image/png", MediaPhoto},
		{"a.mp4", "", MediaVideo},
		{"x.unknown", "video/quicktime", MediaVideo},
		{"voice.ogg", "", MediaAudio},
		{"x.unknown", "", MediaDocument},
	}
	for _, c := range cases {
		if got := guessMediaKind(c.path, c.mime); got != c.want {
			t.Errorf("guessMediaKind(%q, %q) = %q, want %q", c.path, c.mime, got, c.want)
		}
	}
}

// TestDefaultStoreDirIsAbsolute guards against a regression where the
// default store path was relative (which broke if the host changed cwd
// after constructing the Client).
func TestDefaultStoreDirIsAbsolute(t *testing.T) {
	d := defaultStoreDir()
	if !filepath.IsAbs(d) {
		t.Errorf("defaultStoreDir = %q is not absolute", d)
	}
}

// TestUpdateDispatcherInstalled is a smoke check that the dispatcher
// is constructed without panicking. The audit added live-update
// ingestion (H1); regressing this would silently break Watch.
func TestUpdateDispatcherInstalled(t *testing.T) {
	c := New(Config{APIID: 1, APIHash: "h"})
	disp := c.installUpdateDispatcher()
	// Just touching Handle proves the value is constructed; we can't
	// drive a real update without a tg.Entities + Updates payload.
	_ = disp
}
