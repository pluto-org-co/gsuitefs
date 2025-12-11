package main

import (
	"context"
	"fmt"

	"github.com/pluto-org-co/gsuitefs/filesystem/config"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

var ExampleCmd = cli.Command{
	Name:        "example",
	Description: "Writes to stdout an example configuration",
	Action: func(ctx context.Context, c *cli.Command) error {
		cfg := Config{
			AdministratorSubject: "administrator@my-domain.com",
			ServiceAccountFile:   "/path/to/service/account.json",
			Include: config.Include{
				Domains: &config.IncludeDomains{
					Users: &config.IncludeUsers{
						PersonalDrive: &config.IncludePersonalDrive{
							Active:  true,
							Trashed: true,
						},
						SharedFiles: true,
						Gmail:       true,
					},
					Groups: &config.IncludeGroups{},
				},
				SharedDrives: true,
			},
		}

		contents, err := yaml.Marshal(&cfg)
		if err != nil {
			return fmt.Errorf("failed to marshal example: %w", err)
		}

		fmt.Println(string(contents))
		return nil
	},
}
