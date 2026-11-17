package extlink

import (
	"regexp"
	"testing"
	"time"
)

func TestOpenClosesOnEachReason(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	ok := State{Now: now, ExpiresAt: now.Add(time.Hour), CreatorCanShare: true, CreatorSeesArtifact: true, ContentIsFile: true}
	if !Open(ok) {
		t.Fatal("healthy link should open")
	}
	cases := map[string]func(*State){
		"expired at the instant": func(s *State) { s.ExpiresAt = now },
		"expired before":         func(s *State) { s.ExpiresAt = now.Add(-time.Second) },
		"cancelled":              func(s *State) { s.Cancelled = true },
		"creator gone":           func(s *State) { s.CreatorGone = true },
		"creator lost share":     func(s *State) { s.CreatorCanShare = false },
		"creator lost view":      func(s *State) { s.CreatorSeesArtifact = false },
		"content not a file":     func(s *State) { s.ContentIsFile = false },
	}
	for name, change := range cases {
		s := ok
		change(&s)
		if Open(s) {
			t.Errorf("%s: link should be closed", name)
		}
	}
}

func TestNewCodeIsSixDigits(t *testing.T) {
	re := regexp.MustCompile(`^\d{6}$`)
	for range 1000 {
		if c := NewCode(); !re.MatchString(c) {
			t.Fatalf("code %q", c)
		}
	}
}

func TestNewTokenIsLongAndURLSafe(t *testing.T) {
	a, b := NewToken(), NewToken()
	if a == b || len(a) < 43 || !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(a) {
		t.Fatalf("tokens %q %q", a, b)
	}
}

func TestMaskEmail(t *testing.T) {
	for in, want := range map[string]string{
		"a@b.com":           "a***@b.com",
		"henry@cliente.com": "h***@cliente.com",
		"semarroba":         "***",
		"@x.com":            "***",
	} {
		if got := MaskEmail(in); got != want {
			t.Errorf("MaskEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSessionExpiryCapsAtLink(t *testing.T) {
	now := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
	if got := SessionExpiry(now, now.Add(7*24*time.Hour)); !got.Equal(now.Add(7 * 24 * time.Hour)) {
		t.Fatalf("capped: %v", got)
	}
	if got := SessionExpiry(now, now.Add(365*24*time.Hour)); !got.Equal(now.Add(SessionTTL)) {
		t.Fatalf("30 days: %v", got)
	}
}

func TestValidExpiry(t *testing.T) {
	for _, d := range []int{7, 30, 90, 365} {
		if !ValidExpiry(d) {
			t.Errorf("%d should be valid", d)
		}
	}
	for _, d := range []int{0, 1, 366, -7} {
		if ValidExpiry(d) {
			t.Errorf("%d should be invalid", d)
		}
	}
}

func TestHashIsSHA256Hex(t *testing.T) {
	if Hash("abc") != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatal(Hash("abc"))
	}
}
