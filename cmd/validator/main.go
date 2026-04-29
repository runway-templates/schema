package main

import (
	"context"
	"fmt"
	"os"

	"github.com/runway-templates/schema"
	"github.com/urfave/cli/v3"
)

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

func main() {
	app := cli.Command{
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "quiet",
				Aliases: []string{"q"},
				Value:   false,
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			o := out{silent: c.Bool("quiet")}

			if c.Args().Len() == 0 {
				o.Println("Missing argument")
				return fmt.Errorf("missing argument")
			}

			path := c.Args().First()

			if err := schema.ValidateFile(path); err != nil {
				o.Printf("❌ %s is invalid: %s\n", path, err.Error())
				return err
			}
			o.Printf("✅ %s is valid\n", path)
			return nil
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
