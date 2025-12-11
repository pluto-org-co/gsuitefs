package main

import (
	"context"
	"log"
	"os"

	"github.com/urfave/cli/v3"
)

var App = cli.Command{
	Name: "gsuitefs",
	Commands: []*cli.Command{
		&MountCmd,
		&ExampleCmd,
	},
}

func main() {
	ctx := context.TODO()
	err := App.Run(ctx, os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
