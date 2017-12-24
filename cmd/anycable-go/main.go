package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/anycable/anycable-go/cli"
)

var (
	version string
)

func init() {
	if version == "" {
		version = "unknown"
	}
}

func main() {
	versionPtr := flag.Bool("v", false, "Show version")
	// Disable usage for this flag to show only cli flag
	flag.Usage = func() {}
	flag.Parse()

	if *versionPtr {
		fmt.Println(version)
		os.Exit(0)
	}

	config := cli.LoadConfig()

	fmt.Println(config.Port)
}
