//go:build !windows

package finance

import (
	"errors"
	"os"
	"path/filepath"
)

func safeAncestors(p string) error {
	for {
		if info, e := os.Lstat(p); e == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("Linked paths are not allowed")
		}
		q := filepath.Dir(p)
		if q == p {
			return nil
		}
		p = q
	}
}
