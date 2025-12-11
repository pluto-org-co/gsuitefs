package filesystem

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	"github.com/pluto-org-co/gsuitefs/filesystem/domains"
	"github.com/pluto-org-co/gsuitefs/filesystem/shareddrives"
)

const DriverName = "gsuitefs"

type Root struct {
	fs.Inode

	logger *slog.Logger
	config *config.Config
}

func New(logger *slog.Logger, c *config.Config) (r *Root, err error) {
	if c.Cache.Expiration == 0 {
		c.Cache.Expiration = time.Minute
		logger.Warn("Cache expiration not set", "new-value", c.Cache.Expiration)
	}
	if c.Cache.Path == "" {
		c.Cache.Path, err = os.MkdirTemp("", "*")
		logger.Warn("Cache path not set", "new-value", c.Cache.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize cache path: %w", err)
		}
	}
	return &Root{
		logger: logger.With("context", DriverName, "inode", "root"),
		config: c,
	}, nil
}

var (
	_ fs.NodeOnAdder = (*Root)(nil)
)

func (r *Root) OnAdd(ctx context.Context) {
	logger := r.logger.With("action", "OnAdd")

	if r.config.Include.Domains != nil {
		logger.Debug("Including domains")
		node := r.NewPersistentInode(ctx, domains.New(r.logger, r.config), fs.StableAttr{Mode: syscall.S_IFDIR})
		r.AddChild(domains.NodeName, node, false)
	} else {
		logger.Debug("Ignoring domains")
	}
	if r.config.Include.SharedDrives {
		logger.Debug("Including shared-drives")
		node := r.NewPersistentInode(ctx, shareddrives.New(r.logger, r.config), fs.StableAttr{Mode: syscall.S_IFDIR})
		r.AddChild(shareddrives.NodeName, node, false)
	} else {
		logger.Debug("Ignoring shared-drives")
	}
}
