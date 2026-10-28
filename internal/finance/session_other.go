//go:build !windows

package finance

import "errors"

func protectSession(b []byte) ([]byte, error) {
	return nil, errors.New("This Finance distribution requires Windows DPAPI")
}
func openSession(b []byte) ([]byte, error) {
	return nil, errors.New("This Finance distribution requires Windows DPAPI")
}
