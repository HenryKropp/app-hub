package gateway

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type code struct {
	personID, appID int64
	test            bool
	expires         time.Time
}

// Codes are one-time tickets the Hub gives after checking the person, so
// the app address can open a session without a second login.
type Codes struct {
	mu  sync.Mutex
	m   map[string]code
	now func() time.Time
}

func NewCodes() *Codes { return &Codes{m: map[string]code{}, now: time.Now} }

func token() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (c *Codes) Issue(personID, appID int64, test bool) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	for k, v := range c.m { // limpeza dos vencidos
		if now.After(v.expires) {
			delete(c.m, k)
		}
	}
	t := token()
	c.m[t] = code{personID: personID, appID: appID, test: test, expires: now.Add(60 * time.Second)}
	return t
}

func (c *Codes) Redeem(t string, appID int64, test bool) (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[t]
	if !ok || v.appID != appID || v.test != test {
		return 0, false
	}
	delete(c.m, t)
	if c.now().After(v.expires) {
		return 0, false
	}
	return v.personID, true
}
