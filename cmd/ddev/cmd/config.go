package cmd

import (
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/drud/ddev/pkg/appconfig"
	"github.com/spf13/cobra"
)

// TODO: Temporary hack left in until workspace issues moved, March 2017
var (
	activeApp    string
	activeDeploy string
)

// ConfigCommand represents the `ddev config` command
var ConfigCommand = &cobra.Command{
	Use:   "config",
	Short: "Create or modify a ddev application config in the current directory",
	Run: func(cmd *cobra.Command, args []string) {
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			log.Fatalf("Could not determine current working directory: %v\n", err)
		}

		configFile := path.Join(dir, ".ddev", "config.yaml")

		c, err := appconfig.NewAppConfig(configFile)
		err = c.Config()
		if err != nil {
			log.Fatalf("There was a problem configuring your application: %v\n", err)
		}

		err = c.Write()
		if err != nil {
			log.Fatalf("Could not write ddev config file: %v\n", err)
		}
	},
}

func init() {
	RootCmd.AddCommand(ConfigCommand)
}
