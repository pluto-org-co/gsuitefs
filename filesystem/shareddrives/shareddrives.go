package shareddrives

import (
	"context"
	"log/slog"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
)

const NodeName = "shared-drives"

type SharedDrives struct {
	fs.Inode
	logger *slog.Logger
	config *config.Config
}

func New(logger *slog.Logger, c *config.Config) (d *SharedDrives) {
	return &SharedDrives{logger: logger.With("inode", NodeName), config: c}
}

var _ fs.NodeOnAdder = (*SharedDrives)(nil)

func (d *SharedDrives) OnAdd(ctx context.Context) {
	// TODO: Implement me
}
