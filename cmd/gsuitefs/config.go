package main

import "github.com/pluto-org-co/gsuitefs/filesystem/config"

type Config struct {
	AdministratorSubject string         `yaml:"administrator-subject"`
	ServiceAccountFile   string         `yaml:"service-account-file"`
	Include              config.Include `yaml:"include"`
}
