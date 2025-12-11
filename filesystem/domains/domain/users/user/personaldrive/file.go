package personaldrive

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	"github.com/pluto-org-co/gsuitefs/internal/openat"
	"golang.org/x/sys/unix"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const FileNodeName = "file-node"

type File struct {
	fs.Inode

	// Leave empty for root
	trashed bool
	file    *drive.File
	user    *admin.User
	logger  *slog.Logger
	config  *config.Config
}

func NewFile(logger *slog.Logger, c *config.Config, user *admin.User, trashed bool, file *drive.File) (f *File) {
	f = &File{
		logger:  logger.With("inode", FileNodeName, "filename", file.Name),
		config:  c,
		user:    user,
		trashed: trashed,
		file:    file,
	}
	return f
}

var (
	_ fs.NodeOpener     = (*File)(nil)
	_ fs.NodeGetattrer  = (*File)(nil)
	_ fs.NodeGetxattrer = (*File)(nil)
)

func (f *File) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	logger := f.logger.With("action", "Open")
	fileLocation := path.Join(f.config.Cache.Path, f.file.Id)

	logger.Debug("Checking file", "location", fileLocation)
	_, err := os.Stat(fileLocation)
	if err == nil {
		logger.Debug("Found cached file")
	} else if os.IsNotExist(err) {
		logger.Debug("Pulling from remote")

		client := f.config.HttpClientProviderFunc(ctx, f.user.PrimaryEmail)

		logger.Debug("Preparing drive service")
		driveSvc, err := drive.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			logger.Error("failed to prepare service", "error-msg", err)
			return nil, 0, fs.ToErrno(err)
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
				logger.Error("Failed to download file", "error-msg", err)
				return nil, 0, fs.ToErrno(err)
			}
			defer download.Body.Close()
		} else {
			download, err = driveSvc.Files.
				Get(f.file.Id).
				Context(ctx).
				Download()
			if err != nil {
				logger.Error("Failed to download file", "error-msg", err)
				return nil, 0, fs.ToErrno(err)
			}
			defer download.Body.Close()
		}

		logger.Debug("Saving file in cache")
		srcBuffer := bufio.NewReader(download.Body)

		file, err := os.Create(fileLocation)
		if err != nil {
			logger.Error("Failed to create file", "error-msg", err)
			return nil, 0, fs.ToErrno(err)
		}
		dstBuffer := bufio.NewWriter(file)

		_, err = io.Copy(dstBuffer, srcBuffer)
		if err != nil {
			logger.Error("Failed to copy contents", "error-msg", err)
			return nil, 0, fs.ToErrno(err)
		}
		err = dstBuffer.Flush()
		if err != nil {
			logger.Error("Failed to flush file", "error-msg", err)
			return nil, 0, fs.ToErrno(err)
		}
		file.Close()
		logger.Debug("File saved")
		//	time.AfterFunc(f.config.Cache.Expiration, func() { os.Remove(fileLocation) })
	} else {
		logger.Error("Failed to retrieve file info", "error-msg", err)
		return nil, 0, fs.ToErrno(err)
	}

	logger.Debug("Openning file")
	flags = flags &^ (syscall.O_APPEND | fuse.FMODE_EXEC)

	fd, err := openat.OpenSymlinkAware(f.config.Cache.Path, f.file.Id, int(flags), 0)
	if err != nil {
		logger.Error("failed to open file", "error-msg", err)
		return nil, 0, fs.ToErrno(err)
	}
	fh = fs.NewLoopbackFile(fd)
	return fh, 0, fs.OK
}

func (f *File) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) (errno syscall.Errno) {
	if fh != nil {
		if fga, ok := fh.(fs.FileGetattrer); ok {
			return fga.Getattr(ctx, out)
		}
	}

	p := path.Join(f.config.Cache.Path, f.file.Id)

	var rst syscall.Stat_t

	err := syscall.Lstat(p, &rst)
	if err != nil {
		return fs.ToErrno(err)
	}
	out.FromStat(&rst)
	return fs.OK
}

func (f *File) Getxattr(ctx context.Context, attr string, dest []byte) (out uint32, errno syscall.Errno) {
	sz, err := unix.Lgetxattr(path.Join(f.config.Cache.Path, f.file.Id), attr, dest)
	return uint32(sz), fs.ToErrno(err)
}
