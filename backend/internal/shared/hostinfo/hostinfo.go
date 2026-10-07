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
	dt := int64(st.Blocks) * int64(st.Bsize)
	return Info{MemTotal: total, MemUsed: used, DiskTotal: dt, DiskUsed: dt - int64(st.Bavail)*int64(st.Bsize)}, nil
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
