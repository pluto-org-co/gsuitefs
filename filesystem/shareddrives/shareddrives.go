package shareddrives

import (
	"context"
	"io"
	"log/slog"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pluto-org-co/gsuitefs/cache"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	"github.com/pluto-org-co/gsuitefs/filesystem/shareddrives/shareddrive"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const NodeName = "shared-drives"

const ReaddirCacheKey = 0

type SharedDrives struct {
	fs.Inode

	readdirCache cache.Cache[int, []fuse.DirEntry]
	lookupCache  cache.Cache[string, *drive.Drive]
	logger       *slog.Logger
	config       *config.Config
}

func New(logger *slog.Logger, c *config.Config) (d *SharedDrives) {
	return &SharedDrives{logger: logger.With("inode", NodeName), config: c}
}

var (
	_ fs.NodeReaddirer = (*SharedDrives)(nil)
	_ fs.NodeLookuper  = (*SharedDrives)(nil)
)

func (s *SharedDrives) Readdir(ctx context.Context) (ds fs.DirStream, errno syscall.Errno) {
	logger := s.logger.With("action", "Readdir")

	logger.Debug("Checking cache")
	dirEntries, found := s.readdirCache.Load(ReaddirCacheKey)
	if !found {
		client := s.config.HttpClientProviderFunc(ctx, s.config.AdministratorSubject)

		logger.Debug("Preparing drive service")
		driveSvc, err := drive.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			logger.Error("failed to prepare service", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		logger.Debug("Pulling Shared Drives names list")
		dirEntries = make([]fuse.DirEntry, 0, 100)
		err = driveSvc.Drives.
			List().
			Context(ctx).
			Pages(ctx, func(dl *drive.DriveList) (err error) {
				for _, sharedDrive := range dl.Drives {
					logger.Debug("Listing shared drive", "drive-name", sharedDrive.Name)
					s.lookupCache.Store(sharedDrive.Name, sharedDrive, s.config.Cache.Expiration)
					dirEntries = append(dirEntries, fuse.DirEntry{
						Mode: syscall.S_IFDIR,
						Name: sharedDrive.Name,
					})
				}
				return nil
			})
		if err != nil {
			logger.Error("failed to retrieve shared drives", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		logger.Debug("Storing in cache")
		s.readdirCache.Store(ReaddirCacheKey, dirEntries, s.config.Cache.Expiration)
	} else {
		logger.Debug("Using cache")
	}

	ds = fs.NewListDirStream(dirEntries)
	return ds, fs.OK
}

func (s *SharedDrives) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	logger := s.logger.With("action", "Lookup", "name", name)

	logger.Debug("Checking cache")
	driveEntry, found := s.lookupCache.Load(name)
	if !found {
		client := s.config.HttpClientProviderFunc(ctx, s.config.AdministratorSubject)

		logger.Debug("Preparing drive service")
		driveSvc, err := drive.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			logger.Error("failed to prepare service", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		err = driveSvc.Drives.
			List().
			Context(ctx).
			Pages(ctx, func(dl *drive.DriveList) (err error) {
				for _, sharedDrive := range dl.Drives {
					if sharedDrive.Name == name {
						driveEntry = sharedDrive
						return io.EOF
					}
				}
				return nil
			})
		if err != nil && driveEntry == nil {
			logger.Error("failed to retrieve shared drive", "error-msg", err)
			return nil, fs.ToErrno(err)
		}
		logger.Debug("Storing in cache")
		s.lookupCache.Store(name, driveEntry, s.config.Cache.Expiration)
	} else {
		logger.Debug("Using cache")
	}

	node = s.NewInode(ctx, shareddrive.New(s.logger, s.config, driveEntry), fs.StableAttr{Mode: syscall.S_IFDIR})
	return node, fs.OK
}
