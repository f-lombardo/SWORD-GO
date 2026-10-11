package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"sword-go/internal/cli"
	"sword-go/internal/env"
	"sword-go/internal/web"
)

func main() {
	if err := env.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load .env: %v\n", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "web" {
		app, err := web.NewApp()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		fmt.Printf("Starting SWORD-GO webserver on %s", app.String())

		if err = app.Run(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	}

	if err := cli.Run(ctx, os.Stdout, os.Stderr, args); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
