// Package extlink holds the rules of external links (ADR 045): artifacts
// opened by people outside the company, without a Ziv login. It is pure:
// the handler loads the facts and asks here.
package extlink

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"
)

// ExpiryDays are the only lifetimes a link may have. There is no "never".
var ExpiryDays = []int{7, 30, 90, 365}

const (
	DefaultExpiryDays = 30
	CodeTTL           = 10 * time.Minute
	MaxWrongTries     = 5
	SessionTTL        = 30 * 24 * time.Hour
)

// NewToken returns 32 random bytes in URL-safe base64 without padding.
func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// NewCode returns a 6-digit code, zero-padded.
func NewCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%06d", n.Int64())
}

func Hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func ValidExpiry(days int) bool { return slices.Contains(ExpiryDays, days) }

// MaskEmail keeps the first letter and the domain: h***@cliente.com.
func MaskEmail(email string) string {
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" || domain == "" {
		return "***"
	}
	r := []rune(local)
	return string(r[0]) + "***@" + domain
}

// State is everything that decides whether a link opens right now.
type State struct {
	Now, ExpiresAt      time.Time
	Cancelled           bool
	CreatorGone         bool // the person who created it no longer exists
	CreatorCanShare     bool // still has share_external
	CreatorSeesArtifact bool // still sees the artifact
	ContentIsFile       bool // the version shown is an HTML file
}

// Open is true only when nothing closes the link.
func Open(s State) bool {
	return s.Now.Before(s.ExpiresAt) && !s.Cancelled && !s.CreatorGone &&
		s.CreatorCanShare && s.CreatorSeesArtifact && s.ContentIsFile
}

// SessionExpiry is how long a browser stays let in after the code: 30 days,
// never past the link itself.
func SessionExpiry(now, linkExpires time.Time) time.Time {
	if t := now.Add(SessionTTL); t.Before(linkExpires) {
		return t
	}
	return linkExpires
}
