//go:build !linux

// Package hostinfo reads the server's memory and disk. Outside Linux (a
// developer's Windows or Mac) there is nothing to read: the server page shows
// no machine figures and the rest of the server works as usual.
package hostinfo

import "errors"

type Info struct{ MemTotal, MemUsed, DiskTotal, DiskUsed int64 }

// Read always fails outside Linux.
func Read(dataDir string) (Info, error) {
	return Info{}, errors.New("hostinfo: memory and disk are only read on Linux")
}
