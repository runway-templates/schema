package main

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/runway-templates/schema/release"
	"github.com/urfave/cli/v3"
)

var errAborted = errors.New("release aborted")

func releaseAction(ctx context.Context, c *cli.Command) error {
	o := output(ctx)

	p, err := release.Check(ctx, ".")
	if err != nil {
		o.Printf("❌ %s\n", err)
		return err
	}

	printIssues(o, p.Warnings)
	o.Printf("✅ checks passed\n\n")
	o.Printf("tag:      %s\n", p.Tag)
	o.Printf("previous: %s\n", cmp.Or(p.PreviousTag, "none"))
	o.Printf("branch:   %s\n\n", p.Branch)
	o.Printf("%s\n", p.Message())

	if c.Bool("dry-run") {
		return nil
	}
	if !c.Bool("yes") && !confirm(fmt.Sprintf("push %s to origin? [y/N] ", p.Tag)) {
		o.Println(errAborted.Error())
		return errAborted
	}

	if err := release.Publish(ctx, p); err != nil {
		o.Printf("❌ %s\n", err)
		return err
	}
	o.Printf("✅ pushed %s\n", p.Tag)
	return nil
}

func confirm(prompt string) bool {
	fmt.Print(prompt)
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}
