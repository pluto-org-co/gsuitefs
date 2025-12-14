package config

import (
	"context"
	"net/http"
	"time"
)

type HttpClientProviderFunc func(ctx context.Context, subject string) (client *http.Client)

type (
	IncludeDrive struct {
		Active  bool
		Trashed bool
	}
	IncludeUsers struct {
		PersonalDrive *IncludeDrive
		SharedFiles   bool
		Gmail         bool
	}
	IncludeGroups  struct{}
	IncludeDomains struct {
		Users  *IncludeUsers
		Groups *IncludeGroups
	}
	Include struct {
		Domains      *IncludeDomains
		SharedDrives *IncludeDrive
	}
	Cache struct {
		Path       string
		Expiration time.Duration
	}
	Config struct {
		Cache                  Cache
		AdministratorSubject   string
		HttpClientProviderFunc HttpClientProviderFunc
		Include                Include
	}
)
