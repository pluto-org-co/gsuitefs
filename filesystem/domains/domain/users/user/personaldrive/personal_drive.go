package personaldrive

import (
	"context"
	"log/slog"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
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
		node := p.NewPersistentInode(ctx, NewDirectory(p.logger, p.config, p.user, false, nil), fs.StableAttr{Mode: syscall.S_IFDIR})
		p.AddChild(ActiveNodeName, node, false)
	} else {
		logger.Debug("Ignoring active")
	}
	if p.config.Include.Domains.Users.PersonalDrive.Trashed {
		logger.Debug("Including trashed")
		node := p.NewPersistentInode(ctx, NewDirectory(p.logger, p.config, p.user, true, nil), fs.StableAttr{Mode: syscall.S_IFDIR})
		p.AddChild(TrashedNodeName, node, false)
	} else {
		logger.Debug("Ignoring trashed")
	}
}
