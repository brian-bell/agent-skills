//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package importer

import (
	"context"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

const fileLockRetryInterval = 10 * time.Millisecond

func lockFileExclusive(ctx context.Context, file *os.File) (func(), error) {
	retry := time.NewTicker(fileLockRetryInterval)
	defer retry.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return func() {
				_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
			}, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-retry.C:
		}
	}
}
