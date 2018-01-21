package main

import (
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
	if cli.ShowVersion() {
		fmt.Println(version)
		os.Exit(0)
	}

	if cli.ShowHelp() {
		cli.PrintHelp()
		os.Exit(0)
	}

	config := cli.GetConfig()

	fmt.Println(config)
}
