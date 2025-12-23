package driveutils

import (
	"os"
)

func IgnoredNames(name string) (err error) {
	if name == ".git" || name == "HEAD" {
		return os.ErrNotExist
	}
	return nil
}
