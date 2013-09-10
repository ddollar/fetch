package main

import (
	"fmt"
	"os"
)

func main() {
	gf, err := ReadGemfileLock("Gemfile.lock")
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		os.Exit(1)
	}

	gf.Install("vendor/bundle")
}
