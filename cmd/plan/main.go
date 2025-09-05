package main

import (
	"os"

	"github.com/Kizunad/modular-workflow-v2/components/common/cli"
)

func main() {
	app := cli.NewPlanApp()
	if err := app.Run(os.Args); err != nil {
		app.ShowError(err)
		os.Exit(1)
	}
}
