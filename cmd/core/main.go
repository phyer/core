package main

import (
	"fmt"
	"os"
)

const (
	Name    = "core"
	Version = "1.0.0"
	Author  = "phyer"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("%s %s (by %s)\n", Name, Version, Author)
		return
	}
	fmt.Println("This is a library package, not an executable")
}
