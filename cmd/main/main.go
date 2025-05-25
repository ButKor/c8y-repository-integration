package main

import (
	"github.com/kobu/dm-repo-integration/pkg/app"
)

func main() {
	runtimeApp := app.NewApp()
	runtimeApp.Run()
}
