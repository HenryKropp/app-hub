//go:build !windows

package agent

// Outside Windows (tests and development) there is no DPAPI: the token is
// kept as before, in a file only this user can read.
func protect([]byte) ([]byte, error) { return nil, errNoProtection }

func unprotect([]byte) ([]byte, error) { return nil, errNoProtection }
