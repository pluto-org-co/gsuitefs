package domain

import (
	"context"
	"log/slog"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	"github.com/pluto-org-co/gsuitefs/filesystem/domains/domain/users"
	admin "google.golang.org/api/admin/directory/v1"
)

const NodeName = "domain"

type Domain struct {
	fs.Inode

	domain *admin.Domains
	logger *slog.Logger
	config *config.Config
}

func New(logger *slog.Logger, c *config.Config, domain *admin.Domains) (d *Domain) {
	return &Domain{
		domain: domain,
		logger: logger.With("inode", NodeName, "domain-name", domain.DomainName),
		config: c,
	}
}

var (
	_ fs.NodeOnAdder = (*Domain)(nil)
)

func (d *Domain) OnAdd(ctx context.Context) {
	logger := d.logger.With("action", "OnAdd")
	if d.config.Include.Domains.Users != nil {
		logger.Debug("Including users")
		node := d.NewPersistentInode(ctx, users.New(d.logger, d.config, d.domain), fs.StableAttr{Mode: syscall.S_IFDIR})
		d.AddChild(users.NodeName, node, false)
	} else {
		logger.Debug("Ignoring users")
	}
}
