//go:build windows

package finance

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
)

func safeAncestors(p string) error {
	for {
		if _, e := os.Lstat(p); e == nil {
			u, e := windows.UTF16PtrFromString(p)
			if e != nil {
				return e
			}
			attr, e := windows.GetFileAttributes(u)
			if e != nil {
				return e
			}
			if attr&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
				return errors.New("Linked folders cannot store Finance runtime or backups")
			}
		} else if !os.IsNotExist(e) {
			return e
		}
		q := filepath.Dir(p)
		if q == p {
			return nil
		}
		p = q
	}
}
