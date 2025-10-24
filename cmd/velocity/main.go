package main

import (
	"fmt"
	"os"

	"github.com/bit2swaz/velocity-cache/internal/commands"
)

func main() {
	if err := commands.NewRootCommand().Execute(); err != nil {
		if exitErr, ok := err.(commands.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
