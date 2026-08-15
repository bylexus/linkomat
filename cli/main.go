package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	fmt.Fprintf(os.Stderr, "linkomat %s\n", version)
	os.Exit(0)
}
