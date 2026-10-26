// preview is an explicit local fixture tool, not part of the API executable.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/daviddoherty10/Restringing.ie/backend/internal/preview"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: preview seed|status|session|validate [-database-url URL] [-role customer|stringer|staff|admin] [-browser]")
	}
	action := args[0]
	if action != "seed" && action != "status" && action != "session" && action != "validate" {
		return errors.New("choose seed, status, session or validate")
	}
	flags := flag.NewFlagSet("preview "+action, flag.ContinueOnError)
	flags.SetOutput(errOut)
	target := flags.String("database-url", preview.DefaultURL, "dedicated local preview database URL; DATABASE_URL is deliberately ignored")
	role := flags.String("role", "customer", "fixture account for session command: customer, stringer, staff or admin")
	browser := flags.Bool("browser", false, "print a local browser-console command instead of session JSON")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if err := preview.ValidateTarget(*target); err != nil {
		return err
	}
	if action == "validate" {
		fmt.Fprintln(out, "Local preview database target accepted.")
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := preview.Open(ctx, *target)
	if err != nil {
		return err
	}
	defer pool.Close()
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	switch action {
	case "seed":
		s, err := preview.Seed(ctx, pool)
		if err != nil {
			return err
		}
		return encoder.Encode(s)
	case "status":
		s, err := preview.Status(ctx, pool)
		if err != nil {
			return err
		}
		return encoder.Encode(s)
	case "session":
		s, err := preview.SessionForRole(ctx, pool, *role)
		if err != nil {
			return err
		}
		fmt.Fprintf(errOut, "Local fictional %s session; expires %s. Use only at http://127.0.0.1:5174.\n", s.Role, s.ExpiresAt.Format(time.RFC3339))
		if *browser {
			fmt.Fprintln(out, s.BrowserCommand)
			return nil
		}
		return encoder.Encode(s)
	}
	return nil
}
