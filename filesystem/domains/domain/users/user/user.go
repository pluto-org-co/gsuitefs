package user

import (
	"context"
	"log/slog"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	"github.com/pluto-org-co/gsuitefs/filesystem/domains/domain/users/user/personaldrive"
	admin "google.golang.org/api/admin/directory/v1"
)

const NodeName = "user"

type User struct {
	fs.Inode

	user   *admin.User
	logger *slog.Logger
	config *config.Config
}

func New(logger *slog.Logger, c *config.Config, user *admin.User) (u *User) {
	return &User{logger: logger.With("inode", NodeName, "primary-email", user.PrimaryEmail), config: c, user: user}
}

var (
	_ fs.NodeOnAdder = (*User)(nil)
)

func (u *User) OnAdd(ctx context.Context) {
	logger := u.logger.With("action", "OnAdd")
	if u.config.Include.Domains.Users.PersonalDrive != nil {
		logger.Debug("Including personal drive")
		node := u.NewPersistentInode(ctx, personaldrive.New(u.logger, u.config, u.user), fs.StableAttr{Mode: syscall.S_IFDIR})
		u.AddChild(personaldrive.NodeName, node, false)
	} else {
		logger.Debug("Ignoring personal drive")
	}
	// TODO: Shared drives
	// TODO: Gmail
}
