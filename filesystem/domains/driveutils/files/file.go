package files

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	"golang.org/x/sys/unix"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const NodeName = "file-node"

type File struct {
	fs.Inode

	// Leave empty for root
	trashed bool
	file    *drive.File
	user    *admin.User
	drive   *drive.Drive
	logger  *slog.Logger
	config  *config.Config
}

type Config struct {
	Logger  *slog.Logger
	Config  *config.Config
	User    *admin.User
	Drive   *drive.Drive
	Trashed bool
	File    *drive.File
}

func New(cfg *Config) (f *File) {
	f = &File{
		logger:  cfg.Logger.With("inode", NodeName, "filename", cfg.File.Name),
		config:  cfg.Config,
		user:    cfg.User,
		drive:   cfg.Drive,
		trashed: cfg.Trashed,
		file:    cfg.File,
	}
	return f
}

var (
	_ fs.NodeOpener    = (*File)(nil)
	_ fs.NodeGetattrer = (*File)(nil)
)

func (f *File) fileInfo() (cacheFilename string, modTime, creationTime time.Time, cached bool, err error) {
	cacheFilename = path.Join(f.config.Cache.Path, f.file.Id)

	modTime, err = time.Parse(time.RFC3339, f.file.ModifiedTime)
	if err != nil {
		return cacheFilename, modTime, creationTime, false, fmt.Errorf("failed to parse file modtime: %w", err)
	}

	creationTime, err = time.Parse(time.RFC3339, f.file.CreatedTime)
	if err != nil {
		return cacheFilename, modTime, creationTime, false, fmt.Errorf("failed to parse file modtime: %w", err)
	}

	info, err := os.Stat(cacheFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return cacheFilename, modTime, creationTime, false, nil
		}
		return cacheFilename, modTime, creationTime, false, fmt.Errorf("failed to retrieve file info: %w", err)
	}

	return cacheFilename, modTime, creationTime, info.ModTime().Before(modTime), nil
}

func (f *File) HttpClient(ctx context.Context) (client *http.Client) {
	if f.drive != nil {
		return f.config.HttpClientProviderFunc(ctx, f.config.AdministratorSubject)
	}
	return f.config.HttpClientProviderFunc(ctx, f.user.PrimaryEmail)
}

func (f *File) downloadFile(ctx context.Context, logger *slog.Logger) (cacheFilename string, err error) {
	cacheFilename, modTime, _, cached, err := f.fileInfo()
	if err != nil {
		return cacheFilename, fmt.Errorf("failed to get file information: %w", err)
	}

	if cached {
		logger.Debug("File already cached")
		return cacheFilename, nil
	}

	logger.Debug("Pulling from remote")

	client := f.HttpClient(ctx)

	logger.Debug("Preparing drive service")
	driveSvc, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return "", fmt.Errorf("failed to prepare drive service: %w", err)
	}

	logger.Debug("Downloading file", "mime-type", f.file.MimeType)
	var download *http.Response
	if strings.Contains(f.file.MimeType, "google") {
		var targetMime string
		for mime := range f.file.ExportLinks {
			if strings.Contains(mime, "officedocument") {
				targetMime = mime
				break
			}
			targetMime = mime
		}
		download, err = driveSvc.Files.
			Export(f.file.Id, targetMime).
			Context(ctx).
			Download()
		if err != nil {
			return "", fmt.Errorf("failed to export file contents: %s: %w", targetMime, err)
		}
		defer download.Body.Close()
	} else {
		download, err = driveSvc.Files.
			Get(f.file.Id).
			SupportsAllDrives(true).
			SupportsTeamDrives(true).
			AcknowledgeAbuse(true).
			Context(ctx).
			Download()
		if err != nil {
			return "", fmt.Errorf("failed to download file: %w", err)
		}
		defer download.Body.Close()
	}

	logger.Debug("Saving file in cache")
	srcBuffer := bufio.NewReader(download.Body)

	logger.Debug("Creating file")
	file, err := os.Create(cacheFilename)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	dstBuffer := bufio.NewWriter(file)

	logger.Debug("Copying contents")
	_, err = io.Copy(dstBuffer, srcBuffer)
	if err != nil {
		return "", fmt.Errorf("failed to copy contents: %w", err)
	}
	logger.Debug("Flushing missing data")
	err = dstBuffer.Flush()
	if err != nil {
		return "", fmt.Errorf("failed to flush contents: %w", err)
	}

	logger.Debug("Closing file")
	err = file.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close file: %w", err)
	}

	logger.Debug("Updating mod time")
	err = os.Chtimes(cacheFilename, time.Now(), modTime)
	if err != nil {
		return "", fmt.Errorf("failed to change modify time to local cache: %w", err)
	}

	logger.Debug("File saved")
	return cacheFilename, nil
}

func (f *File) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	logger := f.logger.With("action", "Open")
	filename, err := f.downloadFile(ctx, logger)
	if err != nil {
		logger.Error("Failed to download file", "filename", filename, "error-msg", err)
		return nil, 0, fs.ToErrno(err)
	}

	logger.Debug("Openning file")
	fd, err := unix.Open(filename, 0, flags)
	if err != nil {
		logger.Error("Failed to open file", "error-msg", err)
		return nil, 0, fs.ToErrno(err)
	}

	fh = fs.NewLoopbackFile(fd)
	return fh, 0, fs.OK
}

func (f *File) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) (errno syscall.Errno) {
	logger := f.logger.With("action", "Getattr")

	if fh != nil {
		logger.Debug("Checking file handle")
		if fga, ok := fh.(fs.FileGetattrer); ok {
			logger.Debug("Using file handle")
			return fga.Getattr(ctx, out)
		} else {
			logger.Debug("File handle of wrong type")
		}
	}

	logger.Debug("Populating from syscall", "function", "syscall.Lstat")
	filename, modTime, creationTime, cached, err := f.fileInfo()
	if err != nil {
		logger.Error("failed to get file information", "error-msg", err)
		return fs.ToErrno(err)
	}

	var stat syscall.Stat_t
	if cached {
		err = syscall.Lstat(filename, &stat)
		if err != nil {
			logger.Error("Failed to get file Lstat", "error-msg", err)
			return fs.ToErrno(err)
		}
	} else {
		stat = syscall.Stat_t{
			Mode: syscall.S_IFREG,
			Size: f.file.Size,
			Atim: syscall.NsecToTimespec(modTime.UnixNano()),
			Mtim: syscall.NsecToTimespec(modTime.UnixNano()),
			Ctim: syscall.NsecToTimespec(creationTime.UnixNano()),
		}
	}

	logger.Debug("Loading from Stat")
	out.FromStat(&stat)

	return fs.OK
}
