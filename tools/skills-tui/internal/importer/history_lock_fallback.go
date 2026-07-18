//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package importer

import (
	"errors"
	"os"
)

func lockFileExclusive(*os.File) (func(), error) {
	return nil, errors.New("exclusive file locking is unavailable on this platform")
}
