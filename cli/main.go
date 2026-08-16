package main

import (
	"fmt"
	"os"

	"github.com/bylexus/linkomat/shared"
)

func main() {
	fmt.Fprintf(os.Stderr, "linkomat %s\n", shared.Version)
	os.Exit(0)
}
