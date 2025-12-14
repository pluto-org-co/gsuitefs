package shareddrive

import (
	"context"
	"log/slog"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	"github.com/pluto-org-co/gsuitefs/filesystem/domains/driveutils/directory"
	"google.golang.org/api/drive/v3"
)

const NodeName = "shared-drive"

const (
	ActiveNodeName  = "active"
	TrashedNodeName = "trashed"
)

type Drive struct {
	fs.Inode

	drive  *drive.Drive
	logger *slog.Logger
	config *config.Config
}

func New(logger *slog.Logger, c *config.Config, driveEntry *drive.Drive) (d *Drive) {
	return &Drive{
		drive:  driveEntry,
		logger: logger.With("inode", NodeName, "drive-name", driveEntry.Name),
		config: c,
	}
}

var _ fs.NodeOnAdder = (*Drive)(nil)

func (d *Drive) OnAdd(ctx context.Context) {
	logger := d.logger.With("action", "OnAdd")
	if d.config.Include.SharedDrives.Active {
		logger.Debug("Including active")
		cfg := directory.Config{
			Logger: d.logger,
			Config: d.config,
			Drive:  d.drive,
		}
		node := d.NewPersistentInode(ctx, directory.New(&cfg), fs.StableAttr{Mode: syscall.S_IFDIR})
		d.AddChild(ActiveNodeName, node, false)
	} else {
		logger.Debug("Ignoring active")
	}

	if d.config.Include.SharedDrives.Trashed {
		logger.Debug("Including trashed")
		cfg := directory.Config{
			Logger:  d.logger,
			Config:  d.config,
			Drive:   d.drive,
			Trashed: true,
		}
		node := d.NewPersistentInode(ctx, directory.New(&cfg), fs.StableAttr{Mode: syscall.S_IFDIR})
		d.AddChild(TrashedNodeName, node, false)
	} else {
		logger.Debug("Ignoring trashed")
	}
}
