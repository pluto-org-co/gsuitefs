package users

import (
	"context"
	"log/slog"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pluto-org-co/gsuitefs/cache"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	"github.com/pluto-org-co/gsuitefs/filesystem/domains/domain/users/user"
	"github.com/pluto-org-co/gsuitefs/filesystem/domains/driveutils"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

const NodeName = "users"

const ReaddirCacheKey = 0

type Users struct {
	fs.Inode

	lookupCache  cache.Cache[string, *admin.User]
	readdirCache cache.Cache[int, []fuse.DirEntry]
	domain       *admin.Domains
	logger       *slog.Logger
	config       *config.Config
}

func New(logger *slog.Logger, c *config.Config, domain *admin.Domains) (u *Users) {
	return &Users{logger: logger.With("inode", NodeName), config: c, domain: domain}
}

var (
	_ fs.NodeLookuper  = (*Users)(nil)
	_ fs.NodeReaddirer = (*Users)(nil)
)

func (u *Users) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	logger := u.logger.With("action", "Lookup", "name", name)

	err := driveutils.IgnoredNames(name)
	if err != nil {
		return nil, syscall.ENOENT
	}

	logger.Debug("Checking cache")
	userEntry, found := u.lookupCache.Load(name)
	if !found {
		client := u.config.HttpClientProviderFunc(ctx, u.config.AdministratorSubject)

		logger.Debug("Preparing admin service")
		adminSvc, err := admin.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			logger.Error("failed to prepare service", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		userEntry, err = adminSvc.Users.Get(name).Context(ctx).Do()
		if err != nil {
			logger.Error("failed to retrieve domain information", "error-msg", err)
			return nil, fs.ToErrno(err)
		}
		logger.Debug("Storing in cache")
		u.lookupCache.Store(name, userEntry, u.config.Cache.Expiration)
	} else {
		logger.Debug("Using cache")
	}

	node = u.NewInode(ctx, user.New(u.logger, u.config, userEntry), fs.StableAttr{Mode: syscall.S_IFDIR})
	return node, fs.OK
}

func (u *Users) Readdir(ctx context.Context) (ds fs.DirStream, errno syscall.Errno) {
	logger := u.logger.With("action", "Readdir")

	logger.Debug("Checking cache")
	dirEntries, found := u.readdirCache.Load(ReaddirCacheKey)
	if !found {
		client := u.config.HttpClientProviderFunc(ctx, u.config.AdministratorSubject)

		logger.Debug("Preparing admin service")
		adminSvc, err := admin.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			return
		}

		logger.Debug("Retrieving user list")
		err = adminSvc.Users.
			List().
			Context(ctx).
			Domain(u.domain.DomainName).
			OrderBy("email").
			Pages(ctx, func(ul *admin.Users) (err error) {
				for _, user := range ul.Users {
					logger.Debug("Found username", "primary-email", user.PrimaryEmail)
					u.lookupCache.Store(user.PrimaryEmail, user, u.config.Cache.Expiration)
					dirEntries = append(dirEntries, fuse.DirEntry{
						Mode: syscall.S_IFDIR,
						Name: user.PrimaryEmail,
					})
				}
				return nil
			})
		if err != nil {
			logger.Error("Failed to retrieve user list", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		logger.Debug("Storing in cache")
		u.readdirCache.Store(ReaddirCacheKey, dirEntries, u.config.Cache.Expiration)
	} else {
		logger.Debug("Using cache")
	}

	ds = fs.NewListDirStream(dirEntries)
	return ds, fs.OK
}
