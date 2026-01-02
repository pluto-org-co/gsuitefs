package sharedfiles

import (
	"context"
	"log/slog"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	"github.com/pluto-org-co/gsuitefs/filesystem/domains/driveutils/directory"
	admin "google.golang.org/api/admin/directory/v1"
)

const (
	NodeName       = "shared-files"
	ActiveNodeName = "active"
)

type SharedFiles struct {
	fs.Inode

	user   *admin.User
	logger *slog.Logger
	config *config.Config
}

var _ fs.NodeOnAdder = (*SharedFiles)(nil)

func New(logger *slog.Logger, c *config.Config, user *admin.User) (p *SharedFiles) {
	return &SharedFiles{
		user:   user,
		logger: logger.With("inode", NodeName),
		config: c,
	}
}

func (p *SharedFiles) OnAdd(ctx context.Context) {
	logger := p.logger.With("action", "OnAdd")

	logger.Debug("Including active")
	cfg := directory.Config{
		Logger:       p.logger,
		Config:       p.config,
		User:         p.user,
		SharedWithMe: true,
	}
	node := p.NewPersistentInode(ctx, directory.New(&cfg), fs.StableAttr{Mode: syscall.S_IFDIR})
	p.AddChild(ActiveNodeName, node, false)
}
