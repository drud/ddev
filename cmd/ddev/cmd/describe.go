package cmd

import (
	"fmt"
	"github.com/drud/ddev/pkg/ddevapp"
	"github.com/drud/ddev/pkg/dockerutil"
	"github.com/drud/ddev/pkg/globalconfig"
	"github.com/drud/ddev/pkg/output"
	"github.com/drud/ddev/pkg/util"
	"github.com/fatih/color"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	"strings"
)

// Define flags for the describe command
var (

	// service is the service for additional output.
	service string

	// verbose is whether we want full output.
	verbose bool
)

// DescribeCommand represents the `ddev config` command
var DescribeCommand = &cobra.Command{
	Use:   "describe [projectname] [--service=name]",
	Aliases: []string{"status", "st", "desc"},
	Short: "Get a detailed description of a running ddev project.",
	Long: `Get a detailed description of a running ddev project. Describe provides basic
information about a ddev project, including its name, location, url, and status.
It also provides details for MySQL connections, and connection information for
additional services like MailHog and phpMyAdmin. You can run 'ddev describe' from
a project directory to describe that project, or you can specify a project to describe by
running 'ddev describe <projectname>.`,
	Example: "ddev describe\nddev describe <projectname>\nddev describe --service=web\nddev status\nddev st",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 1 {
			util.Failed("Too many arguments provided. Please use 'ddev describe' or 'ddev describe [projectname]'")
		}

		projects, err := getRequestedProjects(args, false)
		if err != nil {
			util.Failed("Failed to describe project(s): %v", err)
		}
		project := projects[0]

		if err := ddevapp.CheckForMissingProjectFiles(project); err != nil {
			util.Failed("Failed to describe %s: %v", project.Name, err)
		}

		desc, err := project.Describe(false)
		if err != nil {
			util.Failed("Failed to describe project %s: %v", project.Name, err)
		}

		renderedDesc, err := renderAppDescribe(desc)
		util.CheckErr(err) // We shouldn't ever end up with an unrenderable desc.
		output.UserOut.WithField("raw", desc).Print(renderedDesc)
	},
}

// renderAppDescribe takes the map describing the app and renders it for plain-text output
func renderAppDescribe(desc map[string]interface{}) (string, error) {

	var output string
	var formattedStatus string

	status := desc["status"]

	url := ""
	if status == ddevapp.SiteRunning {
		if globalconfig.GetCAROOT() != "" {
			url = desc["httpsurl"].(string)
		} else {
			url = desc["httpurl"].(string)
		}
	}

	output = "\nProject Information\n-------------------\n"
	appTable := uitable.New()
	appTable.Separator = "  "
	appTable.Wrap = true
	appTable.AddRow("Name", desc["name"])
	appTable.AddRow("Type", desc["type"])
	appTable.AddRow("Location", desc["shortroot"])
	appTable.AddRow("URL", url)

	formattedStatus = formatStatus(status.(string))
	appTable.AddRow("Status", formattedStatus)
	output = output + fmt.Sprint(appTable)

	// Only show extended status for running sites.
	if status == ddevapp.SiteRunning {
		dockerIP, err := dockerutil.GetDockerIP()
		if err != nil {
			return "", err
		}

		// Build our service table.
		output = output + "\n\nServices\n-------------------\n"
		services := uitable.New()
		services.Separator = " | "
		services.AddRow(
			"SERVICE",
			"HOST",
			"STATE",
			"URL/PORT",
			"INFO",
		)

		// Basic info about the web container.
		services.AddRow(
			"Web",
			"web",
			"@todo",
			url,
			"PHP "+desc["php_version"].(string)+", "+desc["webserver_type"].(string),
		)

		// Basic info about the database container.
		var dbinfo map[string]interface{}
		var dbinfoString = ""
		if _, ok := desc["dbinfo"]; ok {
			dbinfo = desc["dbinfo"].(map[string]interface{})
			if _, ok := dbinfo["mariadb_version"]; ok {
				dbinfoString = "MariaDB " + dbinfo["mariadb_version"].(string)
			} else if _, ok := dbinfo["mysql_version"]; ok {
				dbinfoString = "MySQL" + dbinfo["mysql_version"].(string)
			} else {
				dbinfoString = dbinfo["database_type"].(string)
			}
		}

		if dbinfo != nil {
			services.AddRow(
				"Database",
				"db",
				"@todo",
				fmt.Sprintf("%s:%d", dockerIP, dbinfo["published_port"]),
				dbinfoString,
			)
		}

		services.AddRow(
			"MailHog",
			"",
			"@todo",
			desc["mailhog_https_url"],
			"",
		)

		if _, ok := desc["phpmyadmin_https_url"]; ok {
			services.AddRow(
				"phpMyAdmin",
				"",
				"@todo",
				desc["phpmyadmin_https_url"],
				"",
			)
		}

		for k, v := range desc["extra_services"].(map[string]map[string]string) {
			url := ""

			if httpsURL, ok := v["https_url"]; ok {
				url = httpsURL
			} else if httpURL, ok := v["http_url"]; ok {
				url = httpURL
			}

			services.AddRow(
				k,
				"",
				formatStatus(v["status"]),
				url,
				v["version"],
			)
		}

		// Output our service table.
		output = output + fmt.Sprintln(services)

		// Extended info about the web container.
		if verbose || service == "web" {
			urlTable := uitable.New()
			urlTable.MaxColWidth = 80
			for _, url := range desc["urls"].([]string) {
				urlTable.AddRow(url)
			}
			output = output + "\nURLs\n----\n"

			output = output + fmt.Sprintln(urlTable)
		}

		// Extended info about the database container.
		if verbose || service == "db" {
			if dbinfo != nil {
				output = output + "\n" + "MySQL/MariaDB Credentials\n-------------------------\n" + `Username: "db", Password: "db", Default database: "db"` + "\n"
				output = output + "\n" + `or use root credentials when needed: Username: "root", Password: "root"` + "\n\n"

				output = output + "Database hostname and port INSIDE container: db:3306\n"
				output = output + fmt.Sprintf("To connect to db server inside container or in project settings files: \nmysql --host=db --user=db --password=db --database=db\n")

				output = output + fmt.Sprintf("Database hostname and port from HOST: %s:%d\n", dockerIP, dbinfo["published_port"])
				output = output + fmt.Sprintf("To connect to mysql from your host machine, \nmysql --host=%s --port=%d --user=db --password=db --database=db\n", dockerIP, dbinfo["published_port"])
			} else {
				output = output + "\n" + "DB container is excluded, so no db information provided\n"
			}
		}

		// Extended info about MailHog.
		if verbose || service == "mailhog" {
			output = output + "\n" + "MailHog URLs\n------------\n"
			mailhog := uitable.New()
			mailhog.AddRow("HTTPS", desc["mailhog_https_url"])
			mailhog.AddRow("HTTP", desc["mailhog_url"])
			output = output + fmt.Sprintln(mailhog)
		}

		// Extended info about database administration.
		if _, ok := desc["phpmyadmin_https_url"]; ok {
			if verbose || service == "dba" {
				output = output + "\n" + "phpMyAdmin URLs\n------------\n"
				dba := uitable.New()
				if _, ok := desc["phpmyadmin_https_url"]; ok {
					dba.AddRow("HTTPS", desc["phpmyadmin_https_url"])
				}
				if _, ok := desc["phpmyadmin_url"]; ok {
					dba.AddRow("HTTP", desc["phpmyadmin_url"])
				}
				output = output + fmt.Sprintln(dba)
			}
		}

		for k, v := range desc["extra_services"].(map[string]map[string]string) {
			if verbose || service == k {
				output = output + "\n" + k + " URLs\n------------\n"
				other := uitable.New()
				if httpsURL, ok := v["https_url"]; ok {
					other.AddRow("HTTPS", httpsURL)
				} else if httpURL, ok := v["http_url"]; ok {
					other.AddRow("HTTP", httpURL)
				}
				output = output + fmt.Sprintln(other)
			}
		}

		output = output + "\n" + ddevapp.RenderRouterStatus() + "\t" + ddevapp.RenderSSHAuthStatus()
	}

	return output, nil
}

func init() {
	RootCmd.AddCommand(DescribeCommand)
	DescribeCommand.Flags().StringVarP(&service, "service", "s", "", "The service for additional information")
	DescribeCommand.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show extended output for all services")
}

func formatStatus(status string) string {
	formattedStatus := fmt.Sprint(status)
	switch {
	case strings.Contains(formattedStatus, ddevapp.SitePaused):
		formattedStatus = color.YellowString(formattedStatus)
	case strings.Contains(formattedStatus, ddevapp.SiteStopped):
		formattedStatus = color.RedString(formattedStatus)
	case strings.Contains(formattedStatus, ddevapp.SiteDirMissing):
		formattedStatus = color.RedString(formattedStatus)
	case strings.Contains(formattedStatus, ddevapp.SiteConfigMissing):
		formattedStatus = color.RedString(formattedStatus)
	default:
		formattedStatus = color.CyanString(formattedStatus)
	}
	return formattedStatus
}
