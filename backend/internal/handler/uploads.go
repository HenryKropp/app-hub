package handler

import (
	"io"
	"sync"
	"time"
)

// upload is one agent upload handed to the waiting browser request.
type upload struct {
	body    io.Reader
	missing bool
	done    chan error // the browser side reports when it finished copying
}

// uploadHub pairs a browser download (waiting on a send_file job) with the
// agent's upload for that job. The Hub is a single process in the pilot.
type uploadHub struct {
	mu      sync.Mutex
	waiting map[int64]chan upload
}

func newUploadHub() *uploadHub { return &uploadHub{waiting: map[int64]chan upload{}} }

func (u *uploadHub) expect(id int64) <-chan upload {
	u.mu.Lock()
	defer u.mu.Unlock()
	ch := make(chan upload, 1)
	u.waiting[id] = ch
	return ch
}

func (u *uploadHub) cancel(id int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.waiting, id)
}

func (u *uploadHub) take(id int64, wait time.Duration) (chan upload, bool) {
	deadline := time.Now().Add(wait)
	for {
		u.mu.Lock()
		ch, ok := u.waiting[id]
		if ok {
			delete(u.waiting, id)
		}
		u.mu.Unlock()
		if ok || time.Now().After(deadline) {
			return ch, ok
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// deliver hands body to the waiting browser; the returned channel reports
// when the copy ended.
func (u *uploadHub) deliver(id int64, body io.Reader, wait time.Duration) (<-chan error, bool) {
	ch, ok := u.take(id, wait)
	if !ok {
		return nil, false
	}
	done := make(chan error, 1)
	ch <- upload{body: body, done: done}
	return done, true
}

// missing tells the waiting browser that the file is gone.
func (u *uploadHub) missing(id int64) {
	if ch, ok := u.take(id, 0); ok {
		ch <- upload{missing: true}
	}
}
