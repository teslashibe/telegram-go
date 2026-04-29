package telegram

import "testing"

func TestNormalizePeerID(t *testing.T) {
	cases := map[string]string{
		"@gotd_en":             "@gotd_en",
		"gotd_en":              "@gotd_en",
		"https://t.me/gotd_en": "@gotd_en",
		"http://t.me/gotd_en/": "@gotd_en",
		"t.me/gotd_en":         "@gotd_en",
		"user:12345":           "user:12345",
		"USER:12345":           "user:12345",
		"channel:99":           "channel:99",
		"chat:42":              "chat:42",
		"":                     "",
		"!!!":                  "",
		"a":                    "",
		"123abc":               "",
		"user:abc":             "",
		"user:":                "",
		"weird:1":              "",
		"has space":            "",
	}
	for in, want := range cases {
		if got := NormalizePeerID(in); got != want {
			t.Errorf("NormalizePeerID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParsePeerID(t *testing.T) {
	if k, n, _, err := ParsePeerID("user:42"); err != nil || k != PeerUser || n != 42 {
		t.Fatalf("ParsePeerID(user:42) = %v,%v,%v", k, n, err)
	}
	if _, _, u, err := ParsePeerID("@foo_bar"); err != nil || u != "@foo_bar" {
		t.Fatalf("ParsePeerID(@foo_bar) = %v,%v", u, err)
	}
	if _, _, _, err := ParsePeerID("!!!"); err == nil {
		t.Fatal("ParsePeerID(!!!) should error")
	}
}

func TestFormatPeerID(t *testing.T) {
	if got := FormatPeerID(PeerChannel, 88); got != "channel:88" {
		t.Errorf("FormatPeerID = %q", got)
	}
	if got := FormatPeerID(PeerKind("nope"), 1); got != "" {
		t.Errorf("FormatPeerID(invalid) = %q", got)
	}
}
