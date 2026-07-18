//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package importer

import (
	"context"
	"errors"
	"os"
)

func lockFileExclusive(ctx context.Context, _ *os.File) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("exclusive file locking is unavailable on this platform")
}
