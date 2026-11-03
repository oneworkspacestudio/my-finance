//go:build windows

package finance

import (
	"errors"
	"golang.org/x/sys/windows"
	"unsafe"
)

func protectSession(b []byte) ([]byte, error) {
	return protectPrivateData(b, 4096)
}
func protectManagerProfile(b []byte) ([]byte, error) {
	return protectPrivateData(b, 128<<10)
}
func protectPrivateData(b []byte, maximum int) ([]byte, error) {
	if len(b) == 0 || len(b) > maximum {
		return nil, errors.New("Invalid session reference")
	}
	in := windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
	var out windows.DataBlob
	if e := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); e != nil {
		return nil, e
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
func openSession(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, errors.New("Missing protected reference")
	}
	in := windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
	var out windows.DataBlob
	if e := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); e != nil {
		return nil, e
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
