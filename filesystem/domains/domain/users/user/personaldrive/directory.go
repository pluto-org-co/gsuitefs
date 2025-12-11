package personaldrive

import (
	"context"
	"fmt"
	"log/slog"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pluto-org-co/gsuitefs/cache"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const ReaddirCacheKey = 0

type Directory struct {
	fs.Inode

	lookupCache  cache.Cache[string, *drive.File]
	readdirCache cache.Cache[int, []fuse.DirEntry]

	// Leave empty for root
	trashed   bool
	directory *drive.File

	user   *admin.User
	logger *slog.Logger
	config *config.Config
}

func NewDirectory(logger *slog.Logger, c *config.Config, user *admin.User, trashed bool, directory *drive.File) (p *Directory) {
	var dirId, dirName string
	if directory == nil {
		dirId = "root"
		dirName = "root"
	} else {
		dirId = directory.Id
		dirName = directory.Name
	}
	return &Directory{
		logger:    logger.With("inode", NodeName, "current-directory-id", dirId, "current-directory-name", dirName),
		config:    c,
		user:      user,
		trashed:   trashed,
		directory: directory,
	}
}

var (
	_ fs.NodeLookuper  = (*Directory)(nil)
	_ fs.NodeReaddirer = (*Directory)(nil)
)

func (d *Directory) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	logger := d.logger.With("action", "Readdir")

	logger.Debug("Checking cache")
	file, found := d.lookupCache.Load(name)
	if !found {
		client := d.config.HttpClientProviderFunc(ctx, d.user.PrimaryEmail)

		logger.Debug("Preparing drive service")
		driveSvc, err := drive.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			logger.Error("failed to prepare service", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		var dirId string
		if d.directory == nil {
			dirId = "root"
		} else {
			dirId = d.directory.Id
		}
		logger.Debug("Pulling file list")
		fl, err := driveSvc.Files.
			List().
			Context(ctx).
			Corpora("user").
			PageSize(10).
			Fields("nextPageToken,files(id,name,fullFileExtension,mimeType,modifiedTime,exportLinks)").
			Q(fmt.Sprintf("trashed=%t and '%s' in parents and name = '%s'", d.trashed, dirId, name)).
			OrderBy("name").
			Do()
		if err != nil {
			logger.Error("Failed to pull file list", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		if len(fl.Files) == 0 {
			logger.Error("File not found")
			return nil, syscall.ENOENT
		}

		file = fl.Files[0]
		logger.Debug("Storing in cache")
		d.lookupCache.Store(name, file, d.config.Cache.Expiration)
	} else {
		logger.Debug("Using cache")
	}

	switch file.MimeType {
	case "application/vnd.google-apps.folder":
		node = d.NewInode(ctx, NewDirectory(d.logger, d.config, d.user, d.trashed, file), fs.StableAttr{Mode: syscall.S_IFDIR})
	default:

		node = d.NewInode(ctx, NewFile(d.logger, d.config, d.user, d.trashed, file), fs.StableAttr{Mode: syscall.S_IFREG})
	}
	return node, fs.OK
}

func (d *Directory) Readdir(ctx context.Context) (ds fs.DirStream, errno syscall.Errno) {
	logger := d.logger.With("action", "Readdir")

	logger.Debug("Checking cache")
	dirEntries, found := d.readdirCache.Load(ReaddirCacheKey)
	if !found {
		client := d.config.HttpClientProviderFunc(ctx, d.user.PrimaryEmail)

		logger.Debug("Preparing drive service")
		driveSvc, err := drive.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			logger.Error("failed to prepare service", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		var dirId string
		if d.directory == nil {
			dirId = "root"
		} else {
			dirId = d.directory.Id
		}

		logger.Debug("Pulling file list", "query", fmt.Sprintf("trashed=false and '%s' in parents", dirId))
		err = driveSvc.Files.
			List().
			Context(ctx).
			Corpora("user").
			PageSize(1_000).
			Q(fmt.Sprintf("trashed=%t and '%s' in parents", d.trashed, dirId)).
			Fields("nextPageToken,files(id,name,fullFileExtension,mimeType,modifiedTime,exportLinks)").
			OrderBy("name").
			Pages(ctx, func(fl *drive.FileList) (err error) {
				logger.Debug("Retrieving page", "page-length", len(fl.Files))
				for _, file := range fl.Files {
					logger.Debug("Found file or directory", "name", file.Name)
					d.lookupCache.Store(file.Name, file, d.config.Cache.Expiration)

					switch file.MimeType {
					case "application/vnd.google-apps.folder":
						dirEntries = append(dirEntries, fuse.DirEntry{
							Mode: syscall.S_IFDIR,
							Name: file.Name,
						})
					default:
						dirEntries = append(dirEntries, fuse.DirEntry{
							Mode: syscall.S_IFREG,
							Name: file.Name,
						})
					}
				}
				return nil
			})
		if err != nil {
			logger.Error("failed to retrieve files", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		logger.Debug("Storing in cache")
		d.readdirCache.Store(ReaddirCacheKey, dirEntries, d.config.Cache.Expiration)
	} else {
		logger.Debug("Using cache")
	}

	ds = fs.NewListDirStream(dirEntries)
	return ds, fs.OK
}
