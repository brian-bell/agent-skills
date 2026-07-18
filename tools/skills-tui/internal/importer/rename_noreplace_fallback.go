//go:build !darwin && !linux

package importer

import (
	"fmt"
)

func renameNoReplace(source, destination string) error {
	return fmt.Errorf("atomic no-overwrite publication is unavailable on this platform")
}
