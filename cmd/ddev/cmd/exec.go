package cmd

import (
	"os"

	"github.com/drud/ddev/pkg/util"
	"github.com/spf13/cobra"
)

// LocalDevExecCmd allows users to execute arbitrary bash commands within a container.
var LocalDevExecCmd = &cobra.Command{
	Use:     "exec <command>",
	Aliases: []string{"."},
	Short:   "Execute a shell command in the container for a service. Uses the web service by default.",
	Long:    `Execute a shell command in the container for a service. Uses the web service by default. To run your command in the container for another service, run "ddev exec --service <service> <cmd>"`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			err := cmd.Usage()
			util.CheckErr(err)
			os.Exit(1)
		}

		app, err := getActiveApp()
		if err != nil {
			util.Failed("Failed to exec command: %v", err)
		}

		if app.SiteStatus() != "running" {
			util.Failed("App not running locally. Try `ddev start`.")
		}

		app.DockerEnv()

		err = app.Exec(serviceType, true, args...)
		if err != nil {
			util.Failed("Failed to execute command %s: %v", args, err)
		}
	},
}

func init() {
	LocalDevExecCmd.Flags().StringVarP(&serviceType, "service", "s", "web", "Defines the service to connect to. [e.g. web, db]")
	// This requires flags for exec to be specified prior to any arguments, allowing for
	// flags to be ignored by cobra for commands that are to be executed in a container.
	LocalDevExecCmd.Flags().SetInterspersed(false)
	RootCmd.AddCommand(LocalDevExecCmd)
}
