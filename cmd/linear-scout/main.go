// Command linear-scout is the CLI entry point.
package main

import (
	"fmt"
	"os"

	"github.com/EricGrill/linear-scout/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}