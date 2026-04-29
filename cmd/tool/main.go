package main

import (
	"context"
	"fmt"
	"os"

	"github.com/runway-templates/schema"
	"github.com/urfave/cli/v3"
)

type ctxKey int

const outputKey ctxKey = iota

type out struct {
	silent bool
}

func (o *out) Printf(msg string, args ...any) {
	if o.silent {
		return
	}

	fmt.Printf(msg, args...)
}

func (o *out) Println(msg string) {
	if o.silent {
		return
	}
	fmt.Println(msg)
}

var (
	argPath = &cli.StringArg{Name: "path", UsageText: "path to a template file"}
)

func main() {
	app := cli.Command{
		Name:        "tool",
		Usage:       "lint and validate",
		Description: "lint and validate Runway templates",
		Version:     "dev",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "quiet",
				Aliases: []string{"q"},
				Value:   false,
			},
		},
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			o := &out{silent: c.Bool("quiet")}
			return context.WithValue(ctx, outputKey, o), nil
		},
		Commands: []*cli.Command{
			{
				Name:      "lint",
				Arguments: []cli.Argument{argPath},
				Action: func(ctx context.Context, c *cli.Command) error {
					o := ctx.Value(outputKey).(*out)
					path := c.StringArg("path")
					if path == "" {
						return fmt.Errorf("missing path argument")
					}

					if err := schema.ValidateFile(path); err != nil {
						o.Printf("❌ %s is invalid: %s\n", path, err)
						return err
					}

					issues, err := schema.LintFile(path)
					if err != nil {
						o.Printf("❌ %s is invalid: %s\n", path, err)
						return err
					}

					if len(issues) == 0 {
						o.Println("✅ no linting issues")
						return nil
					}

					o.Printf("Found issues: %d\n", len(issues))

					for _, issue := range issues {
						o.Println("❌ " + issue.Message)
					}
					return nil
				},
			},
			{
				Name:      "validate",
				Arguments: []cli.Argument{argPath},
				Action: func(ctx context.Context, c *cli.Command) error {
					o := ctx.Value(outputKey).(*out)
					path := c.StringArg("path")
					if path == "" {
						return fmt.Errorf("missing path argument")
					}

					if err := schema.ValidateFile(path); err != nil {
						o.Printf("❌ %s is invalid: %s\n", path, err.Error())
						return err
					}
					o.Printf("✅ %s is valid\n", path)
					return nil
				},
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
