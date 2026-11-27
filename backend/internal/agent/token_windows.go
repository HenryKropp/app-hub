//go:build windows

package agent

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// protect seals the token with Windows data protection (DPAPI): only this
// Windows user on this computer can open it again.
func protect(plain []byte) ([]byte, error) {
	return dpapi(plain, true)
}

func unprotect(sealed []byte) ([]byte, error) {
	return dpapi(sealed, false)
}

func dpapi(in []byte, seal bool) ([]byte, error) {
	if len(in) == 0 {
		return nil, nil
	}
	blob := windows.DataBlob{Size: uint32(len(in)), Data: &in[0]}
	var out windows.DataBlob
	var err error
	if seal {
		err = windows.CryptProtectData(&blob, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&blob, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
