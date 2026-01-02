package directory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pluto-org-co/gsuitefs/cache"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	"github.com/pluto-org-co/gsuitefs/filesystem/domains/driveutils/files"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const NodeName = "dir-node"

type Config struct {
	Logger    *slog.Logger
	Config    *config.Config
	User      *admin.User
	Drive     *drive.Drive
	Trashed   bool
	Directory *drive.File
}

const ReaddirCacheKey = 0

type Directory struct {
	fs.Inode

	lookupCache  cache.Cache[string, *drive.File]
	readdirCache cache.Cache[int, []fuse.DirEntry]

	// Leave empty for root
	trashed   bool
	directory *drive.File

	drive  *drive.Drive
	user   *admin.User
	logger *slog.Logger
	config *config.Config
}

func New(cfg *Config) (p *Directory) {
	var logger = cfg.Logger.With("inode", NodeName)
	switch {
	case cfg.Drive != nil:
		var dirId, dirName string
		if cfg.Directory == nil {
			dirId = cfg.Drive.Id
			dirName = cfg.Drive.Name
		} else {
			dirId = cfg.Directory.Id
			dirName = cfg.Directory.Name
		}
		logger = logger.With("mode", "personal-drive", "directory-name", dirName, "directory-id", dirId)
	case cfg.User != nil:
		var dirId, dirName string
		if cfg.Directory == nil {
			dirId = "root"
			dirName = "root"
		} else {
			dirId = cfg.Directory.Id
			dirName = cfg.Directory.Name
		}
		logger = logger.With("mode", "personal-drive", "directory-name", dirName, "directory-id", dirId)
	}

	return &Directory{
		logger:    logger,
		config:    cfg.Config,
		user:      cfg.User,
		drive:     cfg.Drive,
		trashed:   cfg.Trashed,
		directory: cfg.Directory,
	}
}

var (
	_ fs.NodeLookuper  = (*Directory)(nil)
	_ fs.NodeReaddirer = (*Directory)(nil)
	_ fs.NodeGetattrer = (*Directory)(nil)
)

func (d *Directory) HttpClient(ctx context.Context) (client *http.Client) {
	if d.drive != nil {
		return d.config.HttpClientProviderFunc(ctx, d.config.AdministratorSubject)
	}
	return d.config.HttpClientProviderFunc(ctx, d.user.PrimaryEmail)
}

func (d *Directory) ListCall(svc *drive.Service, name string) (call *drive.FilesListCall, err error) {
	call = svc.Files.List()
	switch {
	case d.user != nil:
		var dirId string
		if d.directory == nil {
			dirId = "root"
		} else {
			dirId = d.directory.Id
		}

		call = call.
			Corpora("user").
			Fields("nextPageToken,files(id,name,fullFileExtension,mimeType,size,modifiedTime,createdTime,exportLinks)").
			OrderBy("name")
		if name == "" {
			return call.Q(fmt.Sprintf("trashed=%t and '%s' in parents", d.trashed, dirId)), nil
		}
		return call.Q(fmt.Sprintf("trashed=%t and '%s' in parents and name = '%s'", d.trashed, dirId, name)), nil
	case d.drive != nil:
		var dirId string
		if d.directory == nil {
			dirId = d.drive.Id
		} else {
			dirId = d.directory.Id
		}

		call = call.
			Corpora("drive").
			Fields("nextPageToken,files(id,name,fullFileExtension,mimeType,size,modifiedTime,createdTime,exportLinks)").
			OrderBy("name").
			IncludeTeamDriveItems(true).
			IncludeItemsFromAllDrives(true).
			SupportsAllDrives(true).
			SupportsTeamDrives(true).
			DriveId(d.drive.Id)
		if name == "" {
			return call.Q(fmt.Sprintf("trashed=%t and '%s' in parents", d.trashed, dirId)), nil
		}
		return call.Q(fmt.Sprintf("trashed=%t and '%s' in parents and name = '%s'", d.trashed, dirId, name)), nil
	default:
		return nil, errors.New("incomplete directory definition: expecting user or drive to be passed")
	}
}

func (d *Directory) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	logger := d.logger.With("action", "Readdir")

	logger.Debug("Checking cache")
	file, found := d.lookupCache.Load(name)
	if !found {
		client := d.HttpClient(ctx)

		logger.Debug("Preparing drive service")
		driveSvc, err := drive.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			logger.Error("failed to prepare service", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		logger.Debug("Pulling file list")
		call, err := d.ListCall(driveSvc, name)
		if err != nil {
			logger.Error("Failed to prepare list call", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		fl, err := call.Context(ctx).PageSize(10).Do()
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
		cfg := Config{
			Logger:    d.logger,
			Config:    d.config,
			User:      d.user,
			Drive:     d.drive,
			Trashed:   d.trashed,
			Directory: file,
		}
		node = d.NewInode(ctx, New(&cfg), fs.StableAttr{Mode: syscall.S_IFDIR})
	default:
		cfg := files.Config{
			Logger:  d.logger,
			Config:  d.config,
			User:    d.user,
			Drive:   d.drive,
			Trashed: d.trashed,
			File:    file,
		}
		node = d.NewInode(ctx, files.New(&cfg), fs.StableAttr{Mode: syscall.S_IFREG})
	}
	return node, fs.OK
}

func (d *Directory) Readdir(ctx context.Context) (ds fs.DirStream, errno syscall.Errno) {
	logger := d.logger.With("action", "Readdir")

	logger.Debug("Checking cache")
	dirEntries, found := d.readdirCache.Load(ReaddirCacheKey)
	if !found {
		client := d.HttpClient(ctx)

		logger.Debug("Preparing drive service")
		driveSvc, err := drive.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			logger.Error("failed to prepare service", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		call, err := d.ListCall(driveSvc, "")
		if err != nil {
			logger.Error("Failed to prepare list call", "error-msg", err)
			return nil, fs.ToErrno(err)
		}

		logger.Debug("Pulling file list")
		err = call.
			Context(ctx).
			PageSize(1_000).
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

func (d *Directory) fileInfo() (modTime, creationTime time.Time, err error) {
	now := time.Now()
	if d.directory == nil {
		return now, now, nil
	}

	modTime, err = time.Parse(time.RFC3339, d.directory.ModifiedTime)
	if err != nil {
		return modTime, creationTime, fmt.Errorf("failed to parse file modtime: %w", err)
	}

	creationTime, err = time.Parse(time.RFC3339, d.directory.CreatedTime)
	if err != nil {
		return modTime, creationTime, fmt.Errorf("failed to parse file modtime: %w", err)
	}
	return modTime, creationTime, nil
}

func (d *Directory) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) (errno syscall.Errno) {
	modTime, creationTime, err := d.fileInfo()
	if err != nil {
		return fs.ToErrno(err)
	}

	out.Ctime = uint64(creationTime.Unix())
	out.Mtime = uint64(modTime.Unix())
	return fs.OK
}
