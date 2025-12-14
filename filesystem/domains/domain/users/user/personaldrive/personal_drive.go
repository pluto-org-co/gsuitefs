package personaldrive

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
	NodeName        = "personal-drive"
	ActiveNodeName  = "active"
	TrashedNodeName = "trashed"
)

type PersonalDrive struct {
	fs.Inode

	user   *admin.User
	logger *slog.Logger
	config *config.Config
}

var _ fs.NodeOnAdder = (*PersonalDrive)(nil)

func New(logger *slog.Logger, c *config.Config, user *admin.User) (p *PersonalDrive) {
	return &PersonalDrive{
		user:   user,
		logger: logger.With("inode", NodeName),
		config: c,
	}
}

func (p *PersonalDrive) OnAdd(ctx context.Context) {
	logger := p.logger.With("action", "OnAdd")
	if p.config.Include.Domains.Users.PersonalDrive.Active {
		logger.Debug("Including active")
		cfg := directory.Config{
			Logger: p.logger,
			Config: p.config,
			User:   p.user,
		}
		node := p.NewPersistentInode(ctx, directory.New(&cfg), fs.StableAttr{Mode: syscall.S_IFDIR})
		p.AddChild(ActiveNodeName, node, false)
	} else {
		logger.Debug("Ignoring active")
	}
	if p.config.Include.Domains.Users.PersonalDrive.Trashed {
		logger.Debug("Including trashed")
		cfg := directory.Config{
			Logger:  p.logger,
			Config:  p.config,
			User:    p.user,
			Trashed: true,
		}
		node := p.NewPersistentInode(ctx, directory.New(&cfg), fs.StableAttr{Mode: syscall.S_IFDIR})
		p.AddChild(TrashedNodeName, node, false)
	} else {
		logger.Debug("Ignoring trashed")
	}
}
