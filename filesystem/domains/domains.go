package domains

import (
	"context"
	"log/slog"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pluto-org-co/gsuitefs/cache"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	"github.com/pluto-org-co/gsuitefs/filesystem/domains/domain"
	"github.com/pluto-org-co/gsuitefs/filesystem/domains/driveutils"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

const NodeName = "domains"

const ReaddirCacheKey = 0

type Domains struct {
	fs.Inode

	lookupCache  cache.Cache[string, *admin.Domains]
	readdirCache cache.Cache[int, []fuse.DirEntry]
	logger       *slog.Logger
	config       *config.Config
}

func New(logger *slog.Logger, c *config.Config) (d *Domains) {
	return &Domains{logger: logger.With("inode", NodeName), config: c}
}

var (
	_ fs.NodeLookuper  = (*Domains)(nil)
	_ fs.NodeReaddirer = (*Domains)(nil)
)

func (d *Domains) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	logger := d.logger.With("action", "Lookup", "name", name)

	err := driveutils.IgnoredNames(name)
	if err != nil {
		return nil, syscall.ENOENT
	}

	logger.Debug("Checking cache")
	domainEntry, found := d.lookupCache.Load(name)
	if !found {
		client := d.config.HttpClientProviderFunc(ctx, d.config.AdministratorSubject)

		logger.Debug("Preparing admin service")
		adminSvc, err := admin.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			logger.Error("failed to prepare service", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		domainEntry, err = adminSvc.Domains.Get("my_customer", name).Context(ctx).Do()
		if err != nil {
			logger.Error("failed to retrieve domain information", "error-msg", err)
			return nil, fs.ToErrno(err)
		}
		logger.Debug("Storing in cache")
		d.lookupCache.Store(name, domainEntry, d.config.Cache.Expiration)
	} else {
		logger.Debug("Using cache")
	}

	node = d.NewInode(ctx, domain.New(d.logger, d.config, domainEntry), fs.StableAttr{Mode: syscall.S_IFDIR})
	return node, fs.OK
}

func (d *Domains) Readdir(ctx context.Context) (ds fs.DirStream, errno syscall.Errno) {
	logger := d.logger.With("action", "Readdir")

	logger.Debug("Checking cache")
	dirEntries, found := d.readdirCache.Load(ReaddirCacheKey)
	if !found {
		client := d.config.HttpClientProviderFunc(ctx, d.config.AdministratorSubject)

		logger.Debug("Preparing admin service")
		adminSvc, err := admin.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			logger.Error("failed to prepare service", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		logger.Debug("Pulling Domain list")
		domainList, err := adminSvc.Domains.
			List("my_customer").
			Context(ctx).
			Do()
		if err != nil {
			logger.Error("failed to retrieve domains", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		dirEntries = make([]fuse.DirEntry, 0, len(domainList.Domains))
		for _, domain := range domainList.Domains {
			logger.Debug("Listing domain", "domain-name", domain.DomainName)
			d.lookupCache.Store(domain.DomainName, domain, d.config.Cache.Expiration)
			dirEntries = append(dirEntries, fuse.DirEntry{
				Mode: syscall.S_IFDIR,
				Name: domain.DomainName,
			})
		}
		logger.Debug("Storing in cache")
		d.readdirCache.Store(ReaddirCacheKey, dirEntries, d.config.Cache.Expiration)
	} else {
		logger.Debug("Using cache")
	}

	ds = fs.NewListDirStream(dirEntries)
	return ds, fs.OK
}
