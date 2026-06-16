package main

import (
	"os"

	"ariadne/internal/workmemorycli"
)

func main() {
	os.Exit(workmemorycli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
