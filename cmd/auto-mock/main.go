package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/hemantobora/auto-mock/internal/cloud"
)

func main() {
	app := &cli.App{
		Name:  "automock",
		Usage: "Generate and deploy mock API infrastructure (interactive by default)",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "profile",
				Usage: "Optional credential profile name to use (e.g., dev, prod)",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "Initialize a new or existing AutoMock project",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "project",
						Usage:   "Optional project name to bypass interactive selection",
						Required: false,
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")
					project := c.String("project")
					return cloud.AutoDetectAndInit(profile, project)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
