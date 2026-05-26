package main

import (
	"github.com/hungthai1401/hostie/src/internal/cmd"
)

// version is injected at build time via -ldflags="-X main.version=<value>".
var version = "dev"

func main() {
	cmd.Execute(version)
}
