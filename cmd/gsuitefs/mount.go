package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/pluto-org-co/gsuitefs"
	"github.com/pluto-org-co/gsuitefs/filesystem"
	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	"github.com/pluto-org-co/gsuitefs/httputils"
	"github.com/urfave/cli/v3"
	"golang.org/x/oauth2/google"
	"gopkg.in/yaml.v3"
)

const (
	MountpointArg = "MOUNTPOINT"
)

const (
	ConfigFlag     = "config"
	ForegroundFlag = "foreground"
	LogLevelFlag   = "log-level"
)

var homedir, _ = os.UserHomeDir()

func doMount(ctx context.Context, c *cli.Command) (err error) {
	mountpoint := c.StringArg(MountpointArg)
	if mountpoint == "" {
		return errors.New("mount point not specified")
	}

	info, err := os.Stat(mountpoint)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed get info from directory: %w", err)
		}
		err = os.Mkdir(mountpoint, 0o755)
		if err != nil {
			return fmt.Errorf("failed to create mountpoint: %s: %w", mountpoint, err)
		}
	} else if !info.IsDir() {
		return fmt.Errorf("mountpoint: %s: is not a directory", mountpoint)
	}

	logLevel := c.Int(LogLevelFlag)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.Level(logLevel)}))

	cfgfileContents, err := os.ReadFile(c.String(ConfigFlag))
	if err != nil {
		return fmt.Errorf("failed to read configuration file: %s: %w", c.String(ConfigFlag), err)
	}

	var yamlConfig Config
	err = yaml.Unmarshal(cfgfileContents, &yamlConfig)
	if err != nil {
		return fmt.Errorf("failed to load config from file: %w", err)
	}

	var fsConfig = config.Config{
		AdministratorSubject: yamlConfig.AdministratorSubject,
		Include:              yamlConfig.Include,
	}

	svcAccountContents, err := os.ReadFile(yamlConfig.ServiceAccountFile)
	if err != nil {
		return fmt.Errorf("failed to read service account file: %w", err)
	}

	_, err = google.JWTConfigFromJSON(svcAccountContents, gsuitefs.Scopes...)
	if err != nil {
		return fmt.Errorf("failed to load configuration from file: %w", err)
	}

	fsConfig.HttpClientProviderFunc = func(ctx context.Context, subject string) (client *http.Client) {
		logger.With("action", "Creating HTTP client", "subject", subject)

		logger.Debug("Importing JWT Config")
		conf, _ := google.JWTConfigFromJSON(svcAccountContents, gsuitefs.Scopes...)
		conf.Subject = subject

		logger.Debug("Generating HTTP Client")
		client = conf.Client(ctx)
		client.Transport = httputils.NewRetryTransport(client.Transport, 1_000, time.Minute)
		return client
	}

	root, err := filesystem.New(logger, &fsConfig)
	if err != nil {
		return fmt.Errorf("failed to prepare root filesystem: %w", err)
	}

	var options fs.Options
	options.FirstAutomaticIno = 1
	options.UID = uint32(os.Getuid())
	options.GID = uint32(os.Getgid())
	options.FsName = mountpoint
	options.Name = "gsuitefs"
	timeout := 10 * time.Second
	options.EntryTimeout = &timeout
	server, err := fs.Mount(mountpoint, root, &options)
	if err != nil {
		return fmt.Errorf("failed to mount filesystem: %w", err)
	}

	defer server.Unmount()

	server.Wait()
	return nil
}

var MountCmd = cli.Command{
	Name:        "mount",
	Description: "Mount the filesystem into a directory",
	ArgsUsage:   "MOUNTPOINT",
	Arguments: []cli.Argument{
		&cli.StringArg{
			Name:      MountpointArg,
			UsageText: "Place to mount the Filesystem",
		},
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     ConfigFlag,
			Category: "Configuration",
			OnlyOnce: true,
			Usage:    "Configuration yaml file",
			Value:    "config.yaml",
		},
		&cli.IntFlag{
			Name:     LogLevelFlag,
			Category: "Logging",
			OnlyOnce: true,
			Usage:    "Log level to use by slog (-4:Debug, 0: Info, 4: Warn, 8: Error)",
			Value:    int(slog.LevelInfo),
		},
		&cli.BoolFlag{
			Name:     ForegroundFlag,
			Category: "Runtime",
			OnlyOnce: true,
			Usage:    "Run the FUSE FS in the foreground",
		},
	},
	Action: func(ctx context.Context, c *cli.Command) (err error) {
		if c.Bool(ForegroundFlag) {
			err = doMount(ctx, c)
			if err != nil {
				return fmt.Errorf("failed to mount: %w", err)
			}
			return nil
		}

		args := make([]string, 0, 1+len(os.Args[1:]))
		args = append(args, os.Args[1:]...)
		args = append(args, "--"+ForegroundFlag)
		cmd := exec.Command(os.Args[0], args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}
		err = cmd.Start()
		if err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}

		err = cmd.Process.Release()
		if err != nil {
			return fmt.Errorf("failed to release daemon: %w", err)
		}
		return nil
	},
}
