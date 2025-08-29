package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
	"automock/internal/cloud"
)

func main() {
	app := &cli.App{
		Name:  "automock",
		Usage: "Generate and deploy mock API infrastructure to the cloud",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "profile",
				Usage: "Optional credential profile name to use",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "Initialize a new AutoMock project",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "project",
						Usage: "Project name",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					project := c.String("project")
					profile := c.String("profile")
					return cloud.AutoDetectAndInit(profile, project)
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
