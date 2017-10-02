package cmd

import (
	"log"
	"os"

	"path/filepath"

	"github.com/drud/ddev/pkg/ddevapp"
	"github.com/drud/ddev/pkg/util"
	"github.com/spf13/cobra"
)

var docrootRelPath string
var siteName string

// ConfigCommand represents the `ddev config` command
var ConfigCommand = &cobra.Command{
	Use:   "config [provider]",
	Short: "Create or modify a ddev application config in the current directory",
	Run: func(cmd *cobra.Command, args []string) {

		appRoot, err := os.Getwd()
		if err != nil {
			util.Failed("Could not determine current working directory: %v\n", err)

		}

		provider := ddevapp.DefaultProviderName

		if len(args) > 1 {
			log.Fatal("Invalid argument detected. Please use 'ddev config' or 'ddev config [provider]' to configure a site.")
		}

		if len(args) == 1 {
			provider = args[0]
		}

		c, err := ddevapp.NewConfig(appRoot, provider)
		if err != nil {
			util.Failed("Could not create new config: %v", err)
		}

		// Set the provider value after load so we can ensure we use the passed in provider value
		// for this configuration.
		c.Provider = provider
		c.Name = siteName
		c.Docroot = docrootRelPath

		if siteName == "" && docrootRelPath == "" {
			err = c.PromptForConfig()
			if err != nil {
				util.Failed("There was a problem configuring your application: %v\n", err)
			}
		} else {
			appType, err := ddevapp.DetermineAppType(c.Docroot)
			if err != nil {
				fullPath, _ := filepath.Abs(c.Docroot)
				util.Failed("Failed to determine app Type (drupal7/drupal8/wordpress), your docroot may be incorrect - looking in directory %v (full path: %v), err: %v", c.Docroot, fullPath, err)
			}
			// If we found an application type just set it and inform the user.
			util.Success("Found a %s codebase at %s\n", c.AppType, filepath.Join(c.AppRoot, c.Docroot))
			provider, err := c.GetProvider()
			err = provider.ValidateField("AppType", appType)
			if err != nil {
				util.Failed("Failed to validate appType %s, err: ", appType, err)
			}
			c.AppType = appType
		}
		err = c.Write()
		if err != nil {
			util.Failed("Could not write ddev config file: %v\n", err)

		}

		// If a provider is specified, prompt about whether to do an import after config.
		switch provider {
		case ddevapp.DefaultProviderName:
			util.Success("Configuration complete. You may now run 'ddev start'.")
		default:
			util.Success("Configuration complete. You may now run 'ddev start' or 'ddev pull'")
		}
	},
}

func init() {
	ConfigCommand.Flags().StringVarP(&siteName, "sitename", "", "", "Provide the sitename of site to configure (normally the same as the directory name)")
	ConfigCommand.Flags().StringVarP(&docrootRelPath, "docroot", "", "", "Provide the relative docroot of the site, like 'docroot' or 'htdocs' or 'web', defaults to empty, the current directory")
	RootCmd.AddCommand(ConfigCommand)
}
