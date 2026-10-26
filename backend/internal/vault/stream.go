package vault

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
)

// Large files (database copies) are sealed in chunks, so they never need to
// fit in memory. Layout: streamMagic, then for each chunk a 4-byte length and
// nonce||ciphertext. Each chunk is bound to its position and to whether it is
// the last one, so chunks cannot be reordered, dropped or cut off unnoticed.
const (
	streamMagic = "AHVAULT1"
	chunkSize   = 64 << 10
)

var ErrStreamCut = errors.New("arquivo protegido incompleto ou alterado")

// Sealed reports whether data starts like a stream written by SealStream.
func Sealed(head []byte) bool { return bytes.HasPrefix(head, []byte(streamMagic)) }

type sealWriter struct {
	v      *Vault
	w      io.Writer
	buf    []byte
	index  uint64
	closed bool
	err    error
}

// SealStream returns a writer that encrypts everything written to it into w.
// Close must be called to write the last chunk; it does not close w.
func (v *Vault) SealStream(w io.Writer) (io.WriteCloser, error) {
	if _, err := io.WriteString(w, streamMagic); err != nil {
		return nil, err
	}
	return &sealWriter{v: v, w: w, buf: make([]byte, 0, chunkSize)}, nil
}

func (s *sealWriter) Write(p []byte) (int, error) {
	if s.closed {
		return 0, errors.New("vault: write after close")
	}
	if s.err != nil {
		return 0, s.err
	}
	n := 0
	for len(p) > 0 {
		if len(s.buf) == chunkSize {
			if s.err = s.flush(false); s.err != nil {
				return n, s.err
			}
		}
		k := copy(s.buf[len(s.buf):chunkSize], p)
		s.buf = s.buf[:len(s.buf)+k]
		p = p[k:]
		n += k
	}
	return n, nil
}

func (s *sealWriter) Close() error {
	if s.closed {
		return s.err
	}
	s.closed = true
	if s.err == nil {
		s.err = s.flush(true)
	}
	return s.err
}

func (s *sealWriter) flush(last bool) error {
	nonce := make([]byte, s.v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	sealed := s.v.aead.Seal(nonce, nonce, s.buf, chunkAD(s.index, last))
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(sealed)))
	if _, err := s.w.Write(size[:]); err != nil {
		return err
	}
	if _, err := s.w.Write(sealed); err != nil {
		return err
	}
	s.index++
	s.buf = s.buf[:0]
	return nil
}

func chunkAD(index uint64, last bool) []byte {
	ad := make([]byte, 9)
	binary.BigEndian.PutUint64(ad, index)
	if last {
		ad[8] = 1
	}
	return ad
}

type openReader struct {
	v     *Vault
	r     *bufio.Reader
	plain []byte
	index uint64
	done  bool
	err   error
}

// OpenStream returns a reader with the plain contents of a stream written by
// SealStream. A missing, altered or cut-off chunk ends in ErrStreamCut.
func (v *Vault) OpenStream(r io.Reader) (io.Reader, error) {
	br := bufio.NewReader(r)
	head := make([]byte, len(streamMagic))
	if _, err := io.ReadFull(br, head); err != nil || !Sealed(head) {
		return nil, ErrStreamCut
	}
	return &openReader{v: v, r: br}, nil
}

func (o *openReader) Read(p []byte) (int, error) {
	for len(o.plain) == 0 {
		if o.done {
			return 0, io.EOF
		}
		if o.err != nil {
			return 0, o.err
		}
		if o.err = o.next(); o.err != nil {
			return 0, o.err
		}
	}
	n := copy(p, o.plain)
	o.plain = o.plain[n:]
	return n, nil
}

func (o *openReader) next() error {
	var size [4]byte
	if _, err := io.ReadFull(o.r, size[:]); err != nil {
		return ErrStreamCut
	}
	n := binary.BigEndian.Uint32(size[:])
	ns := o.v.aead.NonceSize()
	if int(n) < ns+o.v.aead.Overhead() || n > chunkSize+uint32(ns+o.v.aead.Overhead()) {
		return ErrStreamCut
	}
	sealed := make([]byte, n)
	if _, err := io.ReadFull(o.r, sealed); err != nil {
		return ErrStreamCut
	}
	// Try as a middle chunk, then as the last one.
	for _, last := range []bool{false, true} {
		plain, err := o.v.aead.Open(nil, sealed[:ns], sealed[ns:], chunkAD(o.index, last))
		if err != nil {
			continue
		}
		o.index++
		o.plain = plain
		if last {
			if _, err := o.r.ReadByte(); err != io.EOF {
				return ErrStreamCut
			}
			o.done = true
		}
		return nil
	}
	return ErrStreamCut
}
