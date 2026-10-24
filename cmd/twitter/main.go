package main

import (
	"log"
	"os"

	"github.com/yoyooyooo/twitter-use/internal/cli"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("")
	os.Exit(cli.Execute())
}
