package main

import (
	"context"
	"fmt"
	"os"

	"asamu.local/platform/api/internal/command"
)

func main() {
	if err := command.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "asamu:", err)
		os.Exit(1)
	}
}
