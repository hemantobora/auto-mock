package main

import (
	"fmt"
	"os"

	"github.com/hemantobora/auto-mock/internal/cloud"
	"github.com/urfave/cli/v2"
)

// version is set via -ldflags "-X main.version=<version>" during build
var version = "0.0.1-alpha"

func main() {
	app := &cli.App{
		Name:    "automock",
		Usage:   "Generate and deploy mock API infrastructure",
		Version: version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "profile",
				Usage: "Credential profile name (e.g., dev, prod)",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "Initialize AutoMock project with expectations and optional infrastructure deployment",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "project",
						Usage: "Project name (bypasses interactive project selection)",
					},
					&cli.StringFlag{
						Name:  "provider",
						Usage: "LLM provider (anthropic, openai, template) - bypasses provider selection",
					},
					&cli.StringFlag{
						Name:  "collection-file",
						Usage: "Path to API collection file (Postman/Bruno/Insomnia)",
					},
					&cli.StringFlag{
						Name:  "collection-type",
						Usage: "Collection type (postman, bruno, insomnia) - required with --collection-file",
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")

					cliContext := &cloud.CLIContext{
						ProjectName:    c.String("project"),
						Provider:       c.String("provider"),
						CollectionFile: c.String("collection-file"),
						CollectionType: c.String("collection-type"),
					}

					return cloud.AutoDetectAndInit(profile, cliContext)
				},
			},
			{
				Name:  "deploy",
				Usage: "Deploy complete infrastructure for existing project",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "project",
						Usage:    "Project name to deploy",
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "skip-confirmation",
						Usage: "Skip deployment confirmation prompt",
					},
				},
				Action: func(c *cli.Context) error {
					return deployCommand(c)
				},
			},
			{
				Name:  "destroy",
				Usage: "Destroy infrastructure for a project",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "project",
						Usage:    "Project name to destroy",
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "force",
						Usage: "Skip confirmation prompts",
					},
				},
				Action: func(c *cli.Context) error {
					return destroyCommand(c)
				},
			},
			{
				Name:  "status",
				Usage: "Show infrastructure status for a project",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "project",
						Usage:    "Project name to check",
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "detailed",
						Usage: "Show detailed information including metrics",
					},
				},
				Action: func(c *cli.Context) error {
					return statusCommand(c)
				},
			},
			{
				Name:  "locust",
				Usage: "Generate and optionally upload a Locust load testing bundle",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "project", Usage: "Project name."},
					&cli.BoolFlag{Name: "upload", Usage: "Upload bundle to cloud storage."},
					&cli.BoolFlag{Name: "dry-run", Usage: "Simulate upload without persisting objects."},
					&cli.BoolFlag{Name: "edit", Usage: "Download current active bundle for editing (interactive re-upload option)."},
					&cli.BoolFlag{Name: "delete-pointer", Usage: "Delete current bundle pointer (current.json) only; keep versions/bundles."},
					&cli.StringFlag{
						Name:  "collection-file",
						Usage: "Path to API collection file (Postman/Bruno/Insomnia)",
					},
					&cli.StringFlag{
						Name:  "collection-type",
						Usage: "Collection type (postman, bruno, insomnia) - required with --collection-file",
					},
					&cli.StringFlag{
						Name:  "dir",
						Usage: "Output directory for the generated Locust files",
					},
					&cli.BoolFlag{
						Name:  "headless",
						Usage: "Run Locust in headless mode (without UI)",
					},
					&cli.BoolFlag{
						Name:  "distributed",
						Usage: "Generate distributed mode helpers (master/worker scripts)",
					},
				},
				Action: locustCommand,
			},
			{
				Name:  "help",
				Usage: "Show detailed help",
				Action: func(c *cli.Context) error {
					return showDetailedHelp(c)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
