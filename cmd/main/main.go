package main

import (
	"github.com/kobu/repo-int/pkg/app"
)

func main() {
	runtimeApp := app.NewApp()
	runtimeApp.Run()
}
