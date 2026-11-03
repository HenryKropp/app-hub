package agentproto

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
)

// A company's Agente download is one EXE with the company's Ziv address
// appended after the program: the address, then the 8-byte magic and its
// length (4 bytes, big endian). Windows ignores data past the program, so the
// person downloads and opens a single file instead of extracting a ZIP.
var trailerMagic = []byte("ZIVHUB01")

const maxTrailerURL = 2048

// HubURLTrailer is what the server appends to the EXE for one company.
func HubURLTrailer(hubURL string) []byte {
	b := append([]byte(hubURL), trailerMagic...)
	return binary.BigEndian.AppendUint32(b, uint32(len(hubURL)))
}

// ReadHubURLTrailer returns the address appended to the file at path, or ""
// when it has none.
func ReadHubURLTrailer(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	foot := make([]byte, len(trailerMagic)+4)
	if st.Size() < int64(len(foot)) {
		return "", nil
	}
	if _, err := f.ReadAt(foot, st.Size()-int64(len(foot))); err != nil && err != io.EOF {
		return "", err
	}
	if !bytes.Equal(foot[:len(trailerMagic)], trailerMagic) {
		return "", nil
	}
	n := int64(binary.BigEndian.Uint32(foot[len(trailerMagic):]))
	if n == 0 || n > maxTrailerURL || n > st.Size()-int64(len(foot)) {
		return "", nil
	}
	url := make([]byte, n)
	if _, err := f.ReadAt(url, st.Size()-int64(len(foot))-n); err != nil {
		return "", err
	}
	return string(url), nil
}
