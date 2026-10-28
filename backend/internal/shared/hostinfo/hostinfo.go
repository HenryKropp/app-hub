//go:build linux

// Package hostinfo reads the server's memory and disk (Linux).
package hostinfo

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type Info struct{ MemTotal, MemUsed, DiskTotal, DiskUsed int64 }

// Read gives the machine's memory (/proc/meminfo) and the disk holding dataDir.
func Read(dataDir string) (Info, error) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return Info{}, err
	}
	total, used, err := parseMeminfo(string(b))
	if err != nil {
		return Info{}, err
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return Info{}, err
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &st); err != nil {
		return Info{}, err
	}
	dt, du := diskUsage(st.Blocks, st.Bfree, st.Bavail, int64(st.Bsize))
	return Info{MemTotal: total, MemUsed: used, DiskTotal: dt, DiskUsed: du}, nil
}

// diskUsage computes the disk numbers the way df does: used is what is
// taken (blocks - free); total is used plus what is still available to
// ordinary users, so the blocks reserved for root are left out of both.
func diskUsage(blocks, bfree, bavail uint64, bsize int64) (total, used int64) {
	used = int64(blocks-bfree) * bsize
	return used + int64(bavail)*bsize, used
}

func parseMeminfo(text string) (int64, int64, error) {
	vals := map[string]int64{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) >= 2 {
			n, _ := strconv.ParseInt(f[1], 10, 64)
			vals[strings.TrimSuffix(f[0], ":")] = n * 1024
		}
	}
	t, a := vals["MemTotal"], vals["MemAvailable"]
	if t == 0 {
		return 0, 0, errors.New("meminfo sem MemTotal")
	}
	return t, t - a, nil
}
