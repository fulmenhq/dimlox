package fileutil

import (
	"errors"
	"os"
	"runtime"
)

var (
	runtimeGOOS = runtime.GOOS
	removeFile  = os.Remove
	renameFile  = os.Rename
)

func AtomicRename(src, dst string, replaceExisting bool) error {
	if replaceExisting && runtimeGOOS == "windows" {
		if err := removeFile(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return renameFile(src, dst)
}
