package cmd

import (
	"log"

	"os"

	"github.com/drud/ddev/pkg/dockerutil"
	"github.com/drud/ddev/pkg/util"
	"github.com/spf13/cobra"
)

var dbSource string
var dbExtPath string

// ImportDBCmd represents the `ddev import-db` command.
var ImportDBCmd = &cobra.Command{
	Use:   "import-db",
	Short: "Import the database of an existing site to the local dev environment.",
	Long: `Import the database of an existing site to the local development environment.
The database can be provided as a SQL dump in a .sql, .sql.gz, .zip, or .tar.gz
format. For the .zip and .tar.gz formats, a SQL dump in .sql format must be
present at the root of the archive.`,
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			err := cmd.Usage()
			util.CheckErr(err)
			os.Exit(0)
		}

		client := dockerutil.GetDockerClient()

		err := dockerutil.EnsureNetwork(client, netName)
		if err != nil {
			log.Fatal(err)
		}

	},
	Run: func(cmd *cobra.Command, args []string) {
		app, err := getActiveApp("")
		if err != nil {
			util.Failed("Failed to import database: %v", app.GetName(), err)
		}

		err = app.ImportDB(dbSource, dbExtPath)
		if err != nil {
			util.Failed("Failed to import database for %s: %v", app.GetName(), err)
		}
		util.Success("Successfully imported database for %v", app.GetName())
	},
}

func init() {
	ImportDBCmd.Flags().StringVarP(&dbSource, "src", "", "", "Provide the path to a sql dump in .sql or .tar.gz format")
	ImportDBCmd.Flags().StringVarP(&dbExtPath, "extract-path", "", "", "If provided asset is an archive, provide the path to extract within the archive.")
	RootCmd.AddCommand(ImportDBCmd)
}
