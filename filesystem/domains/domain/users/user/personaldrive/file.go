package personaldrive

import (
	"log/slog"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/drive/v3"
)

type File struct {
	*fs.LoopbackNode
	// Leave empty for root
	trashed bool
	file    *drive.File
	user    *admin.User
	logger  *slog.Logger
	config  *config.Config
}

func NewFile(logger *slog.Logger, c *config.Config, user *admin.User, trashed bool, file *drive.File) (p *File) {
	n := &fs.LoopbackNode{}

	root := &fs.LoopbackRoot{
		Path: c.Cache.Path,
		NewNode: func(rootData *fs.LoopbackRoot, parent *fs.Inode, name string, st *syscall.Stat_t) fs.InodeEmbedder {
			return n
		},
	}
	n.RootData = root
	return &File{
		LoopbackNode: n,
		logger:       logger.With("inode", NodeName, "filename", file.Name),
		config:       c,
		user:         user,
		trashed:      trashed,
		file:         file,
	}
}
