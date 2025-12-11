package testsuite

import (
	"os"
	"testing"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stretchr/testify/assert"
)

// Mounts a root
func TestMount(t *testing.T, root fs.InodeEmbedder) (tempDir string, server *fuse.Server) {
	t.Helper()

	assertions := assert.New(t)

	var err error

	tempDir, err = os.MkdirTemp("", "*")
	if !assertions.Nil(err, "failed to create temporary directory") {
		return "", nil
	}

	var options fs.Options
	options.FirstAutomaticIno = 1
	options.Debug = true
	server, err = fs.Mount(tempDir, root, &options)
	if !assertions.Nil(err, "failed to mount root") {
		return "", nil
	}

	t.Cleanup(func() {
		err := server.Unmount()
		if !assertions.Nil(err, "failed to umount fs") {
			return
		}
		os.RemoveAll(tempDir)
	})
	return tempDir, server
}
