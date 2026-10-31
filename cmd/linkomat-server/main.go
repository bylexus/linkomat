package main

import (
	"fmt"
	"os"

	"github.com/bylexus/linkomat/api"
)

func main() {
	if err := api.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
