package main

import (
	"fmt"
	"os"

	"github.com/kanopy-platform/argoslower/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
