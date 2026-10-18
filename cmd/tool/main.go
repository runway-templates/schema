// Command tool lints and validates Runway template files.
package main

import (
	"context"
	"errors"
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

	errMissingPath = errors.New("missing path argument")
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
				Action:    lintAction,
			},
			{
				Name:      "validate",
				Arguments: []cli.Argument{argPath},
				Action:    validateAction,
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func output(ctx context.Context) *out {
	if o, ok := ctx.Value(outputKey).(*out); ok {
		return o
	}
	return &out{}
}

func lintAction(ctx context.Context, c *cli.Command) error {
	o := output(ctx)
	path := c.StringArg("path")
	if path == "" {
		return errMissingPath
	}

	validateErr := schema.ValidateFile(path)
	if validateErr != nil {
		o.Printf("❌ %s is invalid: %s\n", path, validateErr)
	}

	issues, err := schema.LintFile(path)
	if err != nil {
		o.Printf("❌ %s could not be linted: %s\n", path, err)
		return err
	}

	lintErrors := printIssues(o, issues)
	if len(issues) == 0 && validateErr == nil {
		o.Println("✅ no linting issues")
	}

	if validateErr != nil {
		return validateErr
	}
	// Warnings are advisory; only errors fail the command, so
	// CI can gate on the exit code.
	if lintErrors > 0 {
		return fmt.Errorf("%d lint error(s) in %s", lintErrors, path)
	}
	return nil
}

// printIssues prints issues and returns how many are errors.
func printIssues(o *out, issues []schema.Issue) int {
	if len(issues) == 0 {
		return 0
	}
	o.Printf("Found issues: %d\n", len(issues))
	var lintErrors int
	for _, issue := range issues {
		icon := "⚠️ "
		if issue.Severity == schema.SeverityError {
			icon = "❌"
			lintErrors++
		}
		o.Printf("%s %s: %s\n", icon, issue.Path, issue.Message)
	}
	return lintErrors
}

func validateAction(ctx context.Context, c *cli.Command) error {
	o := output(ctx)
	path := c.StringArg("path")
	if path == "" {
		return errMissingPath
	}

	if err := schema.ValidateFile(path); err != nil {
		o.Printf("❌ %s is invalid: %s\n", path, err.Error())
		return err
	}
	o.Printf("✅ %s is valid\n", path)
	return nil
}
