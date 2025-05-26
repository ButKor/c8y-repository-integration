package main

import (
	"github.com/kobu/c8y-devmgmt-repo-intgr/pkg/app"
)

func main() {
	runtimeApp := app.NewApp()
	runtimeApp.Run()
}
