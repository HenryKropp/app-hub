//go:build linux

package versions

import (
	"io/fs"
	"sort"
	"time"

	"golang.org/x/sys/unix"
)

// walk uses directory-fd relative calls (openat/fstatat) instead of full
// paths. On WSL's 9p drives every full-path lookup costs ~10 ms, so this is
// several times faster than filepath.WalkDir.
func (f LocalFolder) walk(fn func(rel string, dir bool, size int64, mod time.Time) error) error {
	fd, err := unix.Open(f.Root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return &fs.PathError{Op: "open", Path: f.Root, Err: err}
	}
	return walkFD(fd, f.Root, "", fn)
}

// walkFD takes ownership of fd and closes it.
func walkFD(fd int, path, prefix string, fn func(rel string, dir bool, size int64, mod time.Time) error) error {
	defer unix.Close(fd)
	names, err := readNames(fd)
	if err != nil {
		return &fs.PathError{Op: "readdirent", Path: path, Err: err}
	}
	sort.Strings(names)
	for _, name := range names {
		rel := prefix + name
		var st unix.Stat_t
		if err := unix.Fstatat(fd, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return &fs.PathError{Op: "lstat", Path: path + "/" + name, Err: err}
		}
		switch st.Mode & unix.S_IFMT {
		case unix.S_IFDIR:
			if err := fn(rel, true, 0, time.Time{}); err != nil {
				if err == fs.SkipDir {
					continue
				}
				return err
			}
			sub, err := unix.Openat(fd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
			if err != nil {
				return &fs.PathError{Op: "open", Path: path + "/" + name, Err: err}
			}
			if err := walkFD(sub, path+"/"+name, rel+"/", fn); err != nil {
				return err
			}
		case unix.S_IFREG:
			if err := fn(rel, false, st.Size, time.Unix(int64(st.Mtim.Sec), int64(st.Mtim.Nsec))); err != nil {
				return err
			}
		}
	}
	return nil
}

func readNames(fd int) ([]string, error) {
	var names []string
	buf := make([]byte, 64*1024)
	for {
		n, err := unix.ReadDirent(fd, buf)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return nil, err
		}
		if n <= 0 {
			return names, nil
		}
		_, _, names = unix.ParseDirent(buf[:n], -1, names)
	}
}
