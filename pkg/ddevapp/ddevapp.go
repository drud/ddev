package ddevapp

import (
	"bytes"
	"embed"
	"fmt"
	"github.com/drud/ddev/pkg/globalconfig"
	"github.com/drud/ddev/pkg/nodeps"
	"github.com/lextoumbourou/goodhosts"
	"github.com/mattn/go-isatty"
	"golang.org/x/crypto/ssh/terminal"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	osexec "os/exec"

	"path"
	"time"

	"github.com/drud/ddev/pkg/appimport"
	"github.com/drud/ddev/pkg/archive"
	"github.com/drud/ddev/pkg/ddevhosts"
	"github.com/drud/ddev/pkg/dockerutil"
	"github.com/drud/ddev/pkg/exec"
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/output"
	"github.com/drud/ddev/pkg/util"
	"github.com/drud/ddev/pkg/version"
	"github.com/fsouza/go-dockerclient"
)

// containerWaitTimeout is the max time we wait for all containers to become ready.
var containerWaitTimeout = 61

// SiteRunning defines the string used to denote running sites.
const SiteRunning = "running"

// SiteStarting is the string for a project that is starting
const SiteStarting = "starting"

// SiteStopped defines the string used to denote a site where the containers were not found/do not exist, but the project is there.
const SiteStopped = "stopped"

// SiteDirMissing defines the string used to denote when a site is missing its application directory.
const SiteDirMissing = "project directory missing"

// SiteConfigMissing defines the string used to denote when a site is missing its .ddev/config.yml file.
const SiteConfigMissing = ".ddev/config.yaml missing"

// SitePaused defines the string used to denote when a site is in the paused (docker stopped) state.
const SitePaused = "paused"

// DdevFileSignature is the text we use to detect whether a settings file is managed by us.
// If this string is found, we assume we can replace/update the file.
const DdevFileSignature = "#ddev-generated"

// DdevApp is the struct that represents a ddev app, mostly its config
// from config.yaml.
type DdevApp struct {
	Name                      string                `yaml:"name"`
	Type                      string                `yaml:"type"`
	Docroot                   string                `yaml:"docroot"`
	PHPVersion                string                `yaml:"php_version"`
	WebserverType             string                `yaml:"webserver_type"`
	WebImage                  string                `yaml:"webimage,omitempty"`
	DBImage                   string                `yaml:"dbimage,omitempty"`
	DBAImage                  string                `yaml:"dbaimage,omitempty"`
	RouterHTTPPort            string                `yaml:"router_http_port"`
	RouterHTTPSPort           string                `yaml:"router_https_port"`
	XdebugEnabled             bool                  `yaml:"xdebug_enabled"`
	NoProjectMount            bool                  `yaml:"no_project_mount,omitempty"`
	AdditionalHostnames       []string              `yaml:"additional_hostnames"`
	AdditionalFQDNs           []string              `yaml:"additional_fqdns"`
	MariaDBVersion            string                `yaml:"mariadb_version"`
	MySQLVersion              string                `yaml:"mysql_version"`
	NFSMountEnabled           bool                  `yaml:"nfs_mount_enabled,omitempty"`
	NFSMountEnabledGlobal     bool                  `yaml:"-"`
	FailOnHookFail            bool                  `yaml:"fail_on_hook_fail,omitempty"`
	FailOnHookFailGlobal      bool                  `yaml:"-"`
	ConfigPath                string                `yaml:"-"`
	AppRoot                   string                `yaml:"-"`
	DataDir                   string                `yaml:"-"`
	SiteSettingsPath          string                `yaml:"-"`
	SiteDdevSettingsFile      string                `yaml:"-"`
	ProviderInstance          *Provider             `yaml:"-"`
	Hooks                     map[string][]YAMLTask `yaml:"hooks,omitempty"`
	UploadDir                 string                `yaml:"upload_dir,omitempty"`
	WorkingDir                map[string]string     `yaml:"working_dir,omitempty"`
	OmitContainers            []string              `yaml:"omit_containers,omitempty,flow"`
	OmitContainerGlobal       []string              `yaml:"-"`
	HostDBPort                string                `yaml:"host_db_port,omitempty"`
	HostWebserverPort         string                `yaml:"host_webserver_port,omitempty"`
	HostHTTPSPort             string                `yaml:"host_https_port,omitempty"`
	MailhogPort               string                `yaml:"mailhog_port,omitempty"`
	MailhogHTTPSPort          string                `yaml:"mailhog_https_port,omitempty"`
	PHPMyAdminPort            string                `yaml:"phpmyadmin_port,omitempty"`
	PHPMyAdminHTTPSPort       string                `yaml:"phpmyadmin_https_port,omitempty"`
	WebImageExtraPackages     []string              `yaml:"webimage_extra_packages,omitempty,flow"`
	DBImageExtraPackages      []string              `yaml:"dbimage_extra_packages,omitempty,flow"`
	ProjectTLD                string                `yaml:"project_tld,omitempty"`
	UseDNSWhenPossible        bool                  `yaml:"use_dns_when_possible"`
	MkcertEnabled             bool                  `yaml:"-"`
	NgrokArgs                 string                `yaml:"ngrok_args,omitempty"`
	Timezone                  string                `yaml:"timezone,omitempty"`
	ComposerVersion           string                `yaml:"composer_version"`
	DisableSettingsManagement bool                  `yaml:"disable_settings_management,omitempty"`
	WebEnvironment            []string              `yaml:"web_environment"`
	//Providers                 map[string]*ProviderInfo `yaml:"providers"`
	ComposeYaml map[string]interface{} `yaml:"-"`
}

// List provides the functionality for `ddev list`
// activeOnly if true only shows projects that are currently docker containers
// continuous if true keeps requesting and outputting continuously
// continuousSleepTime is the time between reports
func List(activeOnly bool, continuous bool, continuousSleepTime int) {
	runTime := util.TimeTrack(time.Now(), "ddev list")
	defer runTime()

	for {
		apps, err := GetProjects(activeOnly)
		if err != nil {
			util.Failed("failed getting GetProjects: %v", err)
		}
		appDescs := make([]map[string]interface{}, 0)

		if len(apps) < 1 {
			output.UserOut.WithField("raw", appDescs).Println("No ddev projects were found.")
		} else {
			table := CreateAppTable()
			for _, app := range apps {
				desc, err := app.Describe(true)
				if err != nil {
					util.Error("Failed to describe project %s: %v", app.GetName(), err)
				}
				appDescs = append(appDescs, desc)
				RenderAppRow(table, desc)
			}
			output.UserOut.WithField("raw", appDescs).Print(table.String() + "\n" + RenderRouterStatus())
		}

		if !continuous {
			break
		}

		time.Sleep(time.Duration(continuousSleepTime) * time.Second)
	}
}

// GetType returns the application type as a (lowercase) string
func (app *DdevApp) GetType() string {
	return strings.ToLower(app.Type)
}

// Init populates DdevApp config based on the current working directory.
// It does not start the containers.
func (app *DdevApp) Init(basePath string) error {
	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("app.Init(%s)", basePath))
	defer runTime()

	newApp, err := NewApp(basePath, true)
	if err != nil {
		return err
	}

	err = newApp.ValidateConfig()
	if err != nil {
		return err
	}

	*app = *newApp
	web, err := app.FindContainerByType("web")

	if err != nil {
		return err
	}

	if web != nil {
		containerApproot := web.Labels["com.ddev.approot"]
		isSameFile, err := fileutil.IsSameFile(containerApproot, app.AppRoot)
		if err != nil {
			return err
		}
		if !isSameFile {
			return fmt.Errorf("a project (web container) in %s state already exists for %s that was created at %s", web.State, app.Name, containerApproot).(webContainerExists)
		}
		return nil
	}
	// Init() is just putting together the DdevApp struct, the containers do
	// not have to exist (app doesn't have to have been started, so the fact
	// we didn't find any is not an error.
	return nil
}

// FindContainerByType will find a container for this site denoted by the containerType if it is available.
func (app *DdevApp) FindContainerByType(containerType string) (*docker.APIContainers, error) {
	labels := map[string]string{
		"com.ddev.site-name":         app.GetName(),
		"com.docker.compose.service": containerType,
	}

	return dockerutil.FindContainerByLabels(labels)
}

// Describe returns a map which provides detailed information on services associated with the running site.
func (app *DdevApp) Describe(short bool) (map[string]interface{}, error) {
	err := app.ProcessHooks("pre-describe")
	if err != nil {
		return nil, fmt.Errorf("Failed to process pre-describe hooks: %v", err)
	}

	shortRoot := RenderHomeRootedDir(app.GetAppRoot())
	appDesc := make(map[string]interface{})

	appDesc["name"] = app.GetName()
	appDesc["status"] = app.SiteStatus()
	appDesc["approot"] = app.GetAppRoot()
	appDesc["shortroot"] = shortRoot
	appDesc["httpurl"] = app.GetHTTPURL()
	appDesc["httpsurl"] = app.GetHTTPSURL()
	appDesc["primary_url"] = app.GetPrimaryURL()
	appDesc["type"] = app.GetType()

	// if short is set, we don't need more information, so return what we have.
	if short {
		return appDesc, nil
	}
	appDesc["hostname"] = app.GetHostname()
	appDesc["hostnames"] = app.GetHostnames()
	appDesc["nfs_mount_enabled"] = (app.NFSMountEnabled || app.NFSMountEnabledGlobal)
	appDesc["fail_on_hook_fail"] = (app.FailOnHookFail || app.FailOnHookFailGlobal)
	httpURLs, httpsURLs, allURLs := app.GetAllURLs()
	appDesc["httpURLs"] = httpURLs
	appDesc["httpsURLs"] = httpsURLs
	appDesc["urls"] = allURLs

	if app.MySQLVersion != "" {
		appDesc["database_type"] = "mysql"
		appDesc["mysql_version"] = app.MySQLVersion
	} else {
		appDesc["database_type"] = "mariadb" // default
		appDesc["mariadb_version"] = app.MariaDBVersion
		if app.MariaDBVersion == "" {
			appDesc["mariadb_version"] = nodeps.MariaDBDefaultVersion
		}
	}

	// Only show extended status for running sites.
	if app.SiteStatus() == SiteRunning {
		if !nodeps.ArrayContainsString(app.GetOmittedContainers(), "db") {
			dbinfo := make(map[string]interface{})
			dbinfo["username"] = "db"
			dbinfo["password"] = "db"
			dbinfo["dbname"] = "db"
			dbinfo["host"] = "db"
			dbPublicPort, err := app.GetPublishedPort("db")
			util.CheckErr(err)
			dbinfo["dbPort"] = GetPort("db")
			util.CheckErr(err)
			dbinfo["published_port"] = dbPublicPort
			dbinfo["database_type"] = "mariadb" // default
			if app.MySQLVersion != "" {
				dbinfo["database_type"] = "mysql"
				dbinfo["mysql_version"] = app.MySQLVersion
			} else {
				if app.MariaDBVersion != "" {
					dbinfo["mariadb_version"] = app.MariaDBVersion
				} else {
					dbinfo["mariadb_version"] = nodeps.MariaDBDefaultVersion
				}
			}
			appDesc["dbinfo"] = dbinfo

			if !nodeps.ArrayContainsString(app.GetOmittedContainers(), "dba") {
				appDesc["phpmyadmin_https_url"] = "https://" + app.GetHostname() + ":" + app.PHPMyAdminHTTPSPort
				appDesc["phpmyadmin_url"] = "http://" + app.GetHostname() + ":" + app.PHPMyAdminPort
			}
		}

		appDesc["mailhog_https_url"] = "https://" + app.GetHostname() + ":" + app.MailhogHTTPSPort
		appDesc["mailhog_url"] = "http://" + app.GetHostname() + ":" + app.MailhogPort
	}

	routerStatus, logOutput := GetRouterStatus()
	appDesc["router_status"] = routerStatus
	appDesc["router_status_log"] = logOutput
	appDesc["ssh_agent_status"] = GetSSHAuthStatus()
	appDesc["php_version"] = app.GetPhpVersion()
	appDesc["webserver_type"] = app.GetWebserverType()

	appDesc["router_http_port"] = app.RouterHTTPPort
	appDesc["router_https_port"] = app.RouterHTTPSPort
	appDesc["xdebug_enabled"] = app.XdebugEnabled
	appDesc["webimg"] = app.WebImage
	appDesc["dbimg"] = app.GetDBImage()
	appDesc["dbaimg"] = app.DBAImage
	appDesc["extra_services"] = map[string]map[string]string{}

	if app.ComposeYaml != nil && len(app.ComposeYaml) > 0 {
		if services, ok := app.ComposeYaml["services"].(map[interface{}]interface{}); ok {
			extraServices := appDesc["extra_services"].(map[string]map[string]string)
			for k, v := range services {
				serviceName := k.(string)

				// Standard services are handled in other ways; we want custom services only
				if nodeps.ArrayContainsString([]string{"web", "db", "dba"}, serviceName) {
					continue
				}

				var svc map[interface{}]interface{}
				if svc, ok = v.(map[interface{}]interface{}); !ok {
					continue
				}

				extraServices[serviceName] = map[string]string{}

				if env, ok := svc["environment"].(map[interface{}]interface{}); ok {
					// Extract HTTP_EXPOSE and HTTPS_EXPOSE for additional info
					for envName, envVal := range env {
						if envName == "HTTP_EXPOSE" || envName == "HTTPS_EXPOSE" {
							envValStr := fmt.Sprintf("%s", envVal)
							portSpecs := strings.Split(envValStr, ",")
							// There might be more than one exposed UI port, but this only handles the first listed,
							// most often there's only one.
							if len(portSpecs) > 0 {
								// HTTPS portSpecs typically look like <exposed>:<containerPort>, for example - HTTPS_EXPOSE=1359:1358
								ports := strings.Split(portSpecs[0], ":")
								extraServices[serviceName][envName.(string)] = ports[0]
								switch envName {
								case "HTTP_EXPOSE":
									extraServices[serviceName]["http_url"] = "http://" + appDesc["hostname"].(string) + ":" + ports[0]
								case "HTTPS_EXPOSE":
									extraServices[serviceName]["https_url"] = "https://" + appDesc["hostname"].(string) + ":" + ports[0]
								}
							}
						}
					}
				}
				// TODO: Handle volume names so they can be deleted on ddev destroy
				// TODO: Show host port access, preferably exposed port. Might require docker inspect?
			}
		}
	}

	err = app.ProcessHooks("post-describe")
	if err != nil {
		return nil, fmt.Errorf("Failed to process post-describe hooks: %v", err)
	}

	return appDesc, nil
}

// GetPublishedPort returns the host-exposed public port of a container.
func (app *DdevApp) GetPublishedPort(serviceName string) (int, error) {
	container, err := app.FindContainerByType(serviceName)
	if err != nil || container == nil {
		return -1, fmt.Errorf("Failed to find container of type %s: %v", serviceName, err)
	}

	privatePort, _ := strconv.ParseInt(GetPort(serviceName), 10, 16)

	publishedPort := dockerutil.GetPublishedPort(privatePort, *container)
	return publishedPort, nil
}

// GetOmittedContainers returns full list of global and local omitted containers
func (app *DdevApp) GetOmittedContainers() []string {
	omitted := app.OmitContainerGlobal
	omitted = append(omitted, app.OmitContainers...)
	return omitted
}

// GetAppRoot return the full path from root to the app directory
func (app *DdevApp) GetAppRoot() string {
	return app.AppRoot
}

// AppConfDir returns the full path to the app's .ddev configuration directory
func (app *DdevApp) AppConfDir() string {
	return filepath.Join(app.AppRoot, ".ddev")
}

// GetDocroot returns the docroot path for ddev app
func (app DdevApp) GetDocroot() string {
	return app.Docroot
}

// GetName returns the app's name
func (app *DdevApp) GetName() string {
	return app.Name
}

// GetPhpVersion returns the app's php version
func (app *DdevApp) GetPhpVersion() string {
	v := nodeps.PHPDefault
	if app.PHPVersion != "" {
		v = app.PHPVersion
	}
	return v
}

// GetWebserverType returns the app's webserver type (nginx-fpm/apache-fpm)
func (app *DdevApp) GetWebserverType() string {
	v := nodeps.WebserverDefault
	if app.WebserverType != "" {
		v = app.WebserverType
	}
	return v
}

// ImportDB takes a source sql dump and imports it to an active site's database container.
func (app *DdevApp) ImportDB(imPath string, extPath string, progress bool, noDrop bool, targetDB string) error {
	app.DockerEnv()
	if targetDB == "" {
		targetDB = "db"
	}
	var extPathPrompt bool
	dbPath, err := os.MkdirTemp(filepath.Dir(app.ConfigPath), ".importdb")
	//nolint: errcheck
	defer os.RemoveAll(dbPath)
	if err != nil {
		return err
	}

	err = app.ProcessHooks("pre-import-db")
	if err != nil {
		return err
	}

	// If they don't provide an import path and we're not on a tty (piped in stuff)
	// then prompt for path to db
	if imPath == "" && isatty.IsTerminal(os.Stdin.Fd()) {
		// ensure we prompt for extraction path if an archive is provided, while still allowing
		// non-interactive use of --src flag without providing a --extract-path flag.
		if extPath == "" {
			extPathPrompt = true
		}
		output.UserOut.Println("Provide the path to the database you wish to import.")
		fmt.Print("Pull path: ")

		imPath = util.GetInput("")
	}

	if imPath != "" {
		importPath, isArchive, err := appimport.ValidateAsset(imPath, "db")
		if err != nil {
			if isArchive && extPathPrompt {
				output.UserOut.Println("You provided an archive. Do you want to extract from a specific path in your archive? You may leave this blank if you wish to use the full archive contents")
				fmt.Print("Archive extraction path:")

				extPath = util.GetInput("")
			}
		}

		switch {
		case strings.HasSuffix(importPath, "sql.gz") || strings.HasSuffix(importPath, "mysql.gz"):
			err = archive.Ungzip(importPath, dbPath)
			if err != nil {
				return fmt.Errorf("failed to extract provided archive: %v", err)
			}

		case strings.HasSuffix(importPath, "zip"):
			err = archive.Unzip(importPath, dbPath, extPath)
			if err != nil {
				return fmt.Errorf("failed to extract provided archive: %v", err)
			}

		case strings.HasSuffix(importPath, "tar"):
			fallthrough
		case strings.HasSuffix(importPath, "tar.gz"):
			fallthrough
		case strings.HasSuffix(importPath, "tgz"):
			err := archive.Untar(importPath, dbPath, extPath)
			if err != nil {
				return fmt.Errorf("failed to extract provided archive: %v", err)
			}

		default:
			err = fileutil.CopyFile(importPath, filepath.Join(dbPath, "db.sql"))
			if err != nil {
				return err
			}
		}

		matches, err := filepath.Glob(filepath.Join(dbPath, "*.*sql"))
		if err != nil {
			return err
		}

		if len(matches) < 1 {
			return fmt.Errorf("no .sql or .mysql files found to import")
		}
	}

	// Inside the db container, the dir for imports will be at /mnt/ddev_config/<tmpdir_name>
	insideContainerImportPath := path.Join("/mnt/ddev_config", filepath.Base(dbPath))

	preImportSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s; GRANT ALL ON %s.* TO 'db'@'%%';", targetDB, targetDB)
	if !noDrop {
		preImportSQL = fmt.Sprintf("DROP DATABASE IF EXISTS %s; ", targetDB) + preImportSQL
	}

	// The perl manipulation removes statements like CREATE DATABASE and USE, which
	// throw off imports. This is a scary manipulation, as it must not match actual content
	// as has actually happened with https://www.ddevhq.org/ddev-local/ddev-local-database-management/
	// and in https://github.com/drud/ddev/issues/2787
	// The backtick after USE is inserted via fmt.Sprintf argument because it seems there's
	// no way to escape a backtick in a string literal.
	inContainerCommand := fmt.Sprintf(`mysql -uroot -proot -e "%s" && pv %s/*.*sql | perl -p -e 's/^(CREATE DATABASE \/\*|USE %s)[^;]*;//' | mysql %s`, preImportSQL, insideContainerImportPath, "`", targetDB)

	// Handle the case where we are reading from stdin
	if imPath == "" && extPath == "" {
		inContainerCommand = fmt.Sprintf(`mysql -uroot -proot -e "%s" && perl -p -e 's/^(CREATE DATABASE \/\*|USE %s)[^;]*;//' | mysql %s`, preImportSQL, "`", targetDB)
	}
	_, _, err = app.Exec(&ExecOpts{
		Service: "db",
		Cmd:     inContainerCommand,
		Tty:     progress && isatty.IsTerminal(os.Stdin.Fd()),
	})

	if err != nil {
		return err
	}

	_, err = app.CreateSettingsFile()
	if err != nil {
		util.Warning("A custom settings file exists for your application, so ddev did not generate one.")
		util.Warning("Run 'ddev describe' to find the database credentials for this application.")
	}

	err = app.PostImportDBAction()
	if err != nil {
		return fmt.Errorf("failed to execute PostImportDBAction: %v", err)
	}

	err = fileutil.PurgeDirectory(dbPath)
	if err != nil {
		return fmt.Errorf("failed to clean up %s after import: %v", dbPath, err)
	}

	err = app.ProcessHooks("post-import-db")
	if err != nil {
		return err
	}

	return nil
}

// ExportDB exports the db, with optional output to a file, default gzip
// targetDB is the db name if not default "db"
func (app *DdevApp) ExportDB(outFile string, gzip bool, targetDB string) error {
	app.DockerEnv()
	if targetDB == "" {
		targetDB = "db"
	}
	opts := &ExecOpts{
		Service:   "db",
		Cmd:       "mysqldump " + targetDB,
		NoCapture: true,
	}
	if gzip {
		opts.Cmd = fmt.Sprintf("mysqldump %s | gzip", targetDB)
	}
	if outFile != "" {
		f, err := os.OpenFile(outFile, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return fmt.Errorf("failed to open %s: %v", outFile, err)
		}
		opts.Stdout = f
		// nolint: errcheck
		defer f.Close()
	}

	_, _, err := app.Exec(opts)

	if err != nil {
		return err
	}

	confMsg := "Wrote database dump from " + app.Name + " database '" + targetDB + "'"
	if outFile != "" {
		confMsg = confMsg + " to file " + outFile
	} else {
		confMsg = confMsg + " to stdout"
	}
	if gzip {
		confMsg = confMsg + " in gzip format"
	} else {
		confMsg = confMsg + " in plain text format"
	}

	_, err = fmt.Fprintf(os.Stderr, confMsg+".\n")

	return err
}

// SiteStatus returns the current status of an application determined from web and db service health.
func (app *DdevApp) SiteStatus() string {
	var siteStatus string
	statuses := map[string]string{"web": ""}

	if !nodeps.ArrayContainsString(app.GetOmittedContainers(), "db") {
		statuses["db"] = ""
	}

	if !fileutil.FileExists(app.GetAppRoot()) {
		siteStatus = fmt.Sprintf(`%s: %v; Please "ddev stop --unlist %s"`, SiteDirMissing, app.GetAppRoot(), app.Name)
		return siteStatus
	}

	_, err := CheckForConf(app.GetAppRoot())
	if err != nil {
		siteStatus = fmt.Sprintf("%s", SiteConfigMissing)
		return siteStatus
	}

	for service := range statuses {
		container, err := app.FindContainerByType(service)
		if err != nil {
			util.Error("app.FindContainerByType(%v) failed", service)
			return ""
		}
		if container == nil {
			statuses[service] = SiteStopped
		} else {
			status, _ := dockerutil.GetContainerHealth(container)

			switch status {
			case "exited":
				statuses[service] = SitePaused
			case "healthy":
				statuses[service] = SiteRunning
			case "starting":
				statuses[service] = SiteStarting
			default:
				statuses[service] = status
			}
		}
	}

	// Base the siteStatus on web container. Then override it if others are not the same.
	siteStatus = statuses["web"]
	for serviceName, status := range statuses {
		if status != siteStatus {
			siteStatus = siteStatus + "\n" + serviceName + ": " + status
		}
	}
	return siteStatus
}

// ImportFiles takes a source directory or archive and copies to the uploaded files directory of a given app.
func (app *DdevApp) ImportFiles(importPath string, extPath string) error {
	app.DockerEnv()

	if err := app.ProcessHooks("pre-import-files"); err != nil {
		return err
	}

	if err := app.ImportFilesAction(importPath, extPath); err != nil {
		return err
	}

	//nolint: revive
	if err := app.ProcessHooks("post-import-files"); err != nil {
		return err
	}

	return nil
}

// ComposeFiles returns a list of compose files for a project.
// It has to put the .ddev/docker-compose.*.y*ml first
// It has to put the docker-compose.override.y*l last
func (app *DdevApp) ComposeFiles() ([]string, error) {
	dir, _ := os.Getwd()
	// nolint:errcheck
	defer os.Chdir(dir)
	err := os.Chdir(app.AppConfDir())
	if err != nil {
		return nil, err
	}
	files, err := filepath.Glob("docker-compose.*.y*ml")
	if err != nil {
		return []string{}, fmt.Errorf("unable to glob docker-compose.*.y*ml in %s: err=%v", app.AppConfDir(), err)
	}

	mainfile := app.DockerComposeYAMLPath()
	if !fileutil.FileExists(mainfile) {
		return nil, fmt.Errorf("failed to find %s", mainfile)
	}

	overrides, err := filepath.Glob("docker-compose.override.y*ml")
	util.CheckErr(err)

	orderedFiles := make([]string, 1)

	// Make sure the main file goes first
	orderedFiles[0] = mainfile

	for _, file := range files {
		// We already have the main file, and it's not in the list anyway, so skip when we hit it.
		// We'll add the override later, so skip it.
		if len(overrides) == 1 && file == overrides[0] {
			continue
		}
		orderedFiles = append(orderedFiles, app.GetConfigPath(file))
	}
	if len(overrides) == 1 {
		orderedFiles = append(orderedFiles, app.GetConfigPath(overrides[0]))
	}
	return orderedFiles, nil
}

// ProcessHooks executes Tasks defined in Hooks
func (app *DdevApp) ProcessHooks(hookName string) error {
	if cmds := app.Hooks[hookName]; len(cmds) > 0 {
		output.UserOut.Printf("Executing %s hook...", hookName)
	}

	for _, c := range app.Hooks[hookName] {
		a := NewTask(app, c)
		if a == nil {
			return fmt.Errorf("unable to create task from %v", c)
		}

		if hookName == "pre-start" {
			for k := range c {
				if k == "exec" || k == "composer" {
					return fmt.Errorf("pre-start hooks cannot contain %v", k)
				}
			}
		}

		output.UserOut.Printf("=== Running task: %s, output below", a.GetDescription())

		err := a.Execute()

		if err != nil {
			if app.FailOnHookFail || app.FailOnHookFailGlobal {
				output.UserOut.Errorf("Task failed: %v: %v", a.GetDescription(), err)
				return fmt.Errorf("Task failed: %v", err)
			}
			output.UserOut.Errorf("Task failed: %v: %v", a.GetDescription(), err)
			output.UserOut.Warn("A task failure does not mean that ddev failed, but your hook configuration has a command that failed.")
		}
	}

	return nil
}

// GetDBImage uses the available mariadb or mysql version or provides the default
func (app *DdevApp) GetDBImage() string {
	// If an explicit dbimage is set, just use it.
	if app.DBImage != "" {
		return app.DBImage
	}

	dbImage := ""
	// If the dbimage has not been overridden (because dbimage takes precedence)
	// and the mariadb_version/mysql_version *has* been changed by config,
	// use the dbimage derived from dbversion.
	// IF dbimage has not been specified (it equals mariadb default)
	// AND mariadb version is NOT the default version
	// Then override the dbimage with related mariadb or mysql version

	// If no (dbimage set or it's the default image) and MariaDB or MySQL version set
	if (app.DBImage == "" || app.DBImage == version.GetDBImage(nodeps.MariaDB)) && (app.MariaDBVersion != "" || app.MySQLVersion != "") {
		switch {
		// mariadb_version is explicitly set
		case app.MariaDBVersion != "":
			dbImage = version.GetDBImage(nodeps.MariaDB, app.MariaDBVersion)
		// mysql_version is explicitly set
		case app.MySQLVersion != "":
			dbImage = version.GetDBImage(nodeps.MySQL, app.MySQLVersion)
		}
	}
	// Default behavior is just to use the MariaDB image.
	if dbImage == "" {
		dbImage = version.GetDBImage(nodeps.MariaDB)
	}
	return dbImage
}

// Start initiates docker-compose up
func (app *DdevApp) Start() error {
	var err error

	app.DockerEnv()

	app.DBImage = app.GetDBImage()

	err = app.CheckExistingAppInApproot()
	if err != nil {
		return err
	}
	// Make sure that any ports allocated are available.
	// and of course add to global project list as well
	err = app.UpdateGlobalProjectList()
	if err != nil {
		return err
	}

	err = app.ProcessHooks("pre-start")
	if err != nil {
		return err
	}

	// We used to support nginx overrides in .ddev/nginx-site.conf
	// Warn in that case
	oldNginxConfig := app.GetConfigPath("nginx-site.conf")
	if fileutil.FileExists(oldNginxConfig) {
		util.Warning("An old nginx-site.conf exists at %s. Please move it to ~/.ddev/nginx_full", oldNginxConfig)
	}

	err = app.GenerateWebserverConfig()
	if err != nil {
		return err
	}

	err = app.PullContainerImages()
	if err != nil {
		return err
	}

	// Make sure that important volumes to mount already have correct ownership set
	// Additional volumes can be added here. This allows us to run a single privileged
	// container with a single focus of changing ownership, instead of having to use sudo
	// inside the container
	uid, _, _ := util.GetContainerUIDGid()
	_, _, err = dockerutil.RunSimpleContainer(version.GetWebImage(), "", []string{"sh", "-c", fmt.Sprintf("chown -R %s /var/lib/mysql /mnt/ddev-global-cache", uid)}, []string{}, []string{}, []string{app.Name + "-mariadb:/var/lib/mysql", "ddev-global-cache:/mnt/ddev-global-cache"}, "", true, false, nil)
	if err != nil {
		return err
	}

	if !nodeps.ArrayContainsString(app.GetOmittedContainers(), "ddev-ssh-agent") {
		err = app.EnsureSSHAgentContainer()
		if err != nil {
			return err
		}
	}

	// Warn the user if there is any custom configuration in use.
	app.CheckCustomConfig()

	// Warn user if there are deprecated items used in the config
	app.CheckDeprecations()

	// Copy any global homeadditions content into its mount location
	globalHomeadditionsPath := filepath.Join(globalconfig.GetGlobalDdevDir(), "homeadditions")
	if fileutil.IsDirectory(globalHomeadditionsPath) {
		projectGlobalHomeadditionsPath := app.GetConfigPath(".homeadditions")
		if fileutil.IsDirectory(projectGlobalHomeadditionsPath) {
			err = os.RemoveAll(projectGlobalHomeadditionsPath)
			if err != nil {
				return err
			}
		}
		err = fileutil.CopyDir(globalHomeadditionsPath, projectGlobalHomeadditionsPath)
		if err != nil {
			return err
		}
	}

	caRoot := globalconfig.GetCAROOT()
	if caRoot == "" {
		util.Warning("mkcert may not be properly installed, we suggest installing it for trusted https support, `brew install mkcert nss`, `choco install -y mkcert`, etc. and then `mkcert -install`")
	}
	router, _ := FindDdevRouter()
	// If the router doesn't exist, go ahead and push mkcert root ca certs into the ddev-global-cache/mkcert
	// This will often be redundant
	if router == nil {
		// Copy ca certs into ddev-global-cache/mkcert
		if caRoot != "" {
			uid, _, _ := util.GetContainerUIDGid()
			err = dockerutil.CopyToVolume(caRoot, "ddev-global-cache", "mkcert", uid)
			if err != nil {
				util.Warning("failed to copy root CA into docker volume ddev-global-cache/mkcert: %v", err)
			} else {
				util.Success("Pushed mkcert rootca certs to ddev-global-cache/mkcert")
			}
		}
	}

	certPath := app.GetConfigPath("custom_certs")
	if fileutil.FileExists(certPath) {
		uid, _, _ := util.GetContainerUIDGid()
		err = dockerutil.CopyToVolume(certPath, "ddev-global-cache", "custom_certs", uid)
		if err != nil {
			util.Warning("failed to copy custom certs into docker volume ddev-global-cache/custom_certs: %v", err)
		} else {
			util.Success("Copied custom certs in %s to ddev-global-cache/custom_certs", certPath)
		}
	}

	// WriteConfig .ddev-docker-compose-*.yaml
	err = app.WriteDockerComposeYAML()
	if err != nil {
		return err
	}

	err = app.AddHostsEntriesIfNeeded()
	if err != nil {
		return err
	}

	// Delete the NFS volumes before we bring up docker-compose.
	// We don't care if the volume wasn't there
	_ = dockerutil.RemoveVolume(app.GetNFSMountVolName())

	_, _, err = dockerutil.ComposeCmd([]string{app.DockerComposeFullRenderedYAMLPath()}, "up", "--build", "-d")
	if err != nil {
		return err
	}

	err = StartDdevRouter()
	if err != nil {
		return err
	}

	err = app.WaitByLabels(map[string]string{"com.ddev.site-name": app.GetName()})
	if err != nil {
		return err
	}

	if _, err = app.CreateSettingsFile(); err != nil {
		return fmt.Errorf("failed to write settings file %s: %v", app.SiteDdevSettingsFile, err)
	}

	err = app.PostStartAction()
	if err != nil {
		return err
	}

	err = app.ProcessHooks("post-start")
	if err != nil {
		return err
	}

	return nil
}

// PullContainerImages pulls the main images with full output, since docker-compose up won't show enough output
func (app *DdevApp) PullContainerImages() error {
	containerImages := map[string]string{
		"db":             app.DBImage,
		"dba":            app.DBAImage,
		"ddev-ssh-agent": version.GetSSHAuthImage(),
		"web":            app.WebImage,
		"router":         version.GetRouterImage(),
	}

	omitted := app.GetOmittedContainers()
	for containerName, imageName := range containerImages {
		if !nodeps.ArrayContainsString(omitted, containerName) {
			err := dockerutil.Pull(imageName)
			if err != nil {
				return err
			}
			if globalconfig.DdevDebug {
				output.UserOut.Printf("Pulling image for %s: %s", containerName, imageName)
			}
		}
	}

	return nil
}

// CheckExistingAppInApproot looks to see if we already have a project in this approot with different name
func (app *DdevApp) CheckExistingAppInApproot() error {
	pList := globalconfig.GetGlobalProjectList()
	for name, v := range pList {
		if app.AppRoot == v.AppRoot && name != app.Name {
			return fmt.Errorf(`This project root %s already contains a project named %s. You may want to remove the existing project with "ddev stop --unlist %s"`, v.AppRoot, name, name)
		}
	}
	return nil
}

//go:embed webserver_config_assets
var webserverConfigAssets embed.FS

// GenerateWebserverConfig generates the default nginx and apache config files
func (app *DdevApp) GenerateWebserverConfig() error {
	// Prevent running as root for most cases
	// We really don't want ~/.ddev to have root ownership, breaks things.
	if os.Geteuid() == 0 {
		output.UserOut.Warning("not generating webserver config files because running with root privileges")
		return nil
	}

	var items = map[string]string{
		"nginx":                         app.GetConfigPath(filepath.Join("nginx_full", "nginx-site.conf")),
		"apache":                        app.GetConfigPath(filepath.Join("apache", "apache-site.conf")),
		"nginx_second_docroot_example":  app.GetConfigPath(filepath.Join("nginx_full", "seconddocroot.conf.example")),
		"README.nginx_full.txt":         app.GetConfigPath(filepath.Join("nginx_full", "README.nginx_full.txt")),
		"README.apache.txt":             app.GetConfigPath(filepath.Join("apache", "README.apache.txt")),
		"apache_second_docroot_example": app.GetConfigPath(filepath.Join("apache", "seconddocroot.conf.example")),
	}
	for t, configPath := range items {
		err := os.MkdirAll(filepath.Dir(configPath), 0755)
		if err != nil {
			return err
		}

		if fileutil.FileExists(configPath) {
			sigExists, err := fileutil.FgrepStringInFile(configPath, DdevFileSignature)
			if err != nil {
				return err
			}
			// If the signature doesn't exist, they have taken over the file, so return
			if !sigExists {
				return nil
			}
		}

		cfgFile := fmt.Sprintf("%s-site-%s.conf", t, app.Type)
		c, err := webserverConfigAssets.ReadFile(path.Join("webserver_config_assets", cfgFile))
		if err != nil {
			c, err = webserverConfigAssets.ReadFile(path.Join("webserver_config_assets", fmt.Sprintf("%s-site-php.conf", t)))
			if err != nil {
				return err
			}
		}
		content := string(c)
		docroot := path.Join("/var/www/html", app.Docroot)
		err = fileutil.TemplateStringToFile(content, map[string]interface{}{"Docroot": docroot}, configPath)
		if err != nil {
			return err
		}
	}
	return nil
}

// ExecOpts contains options for running a command inside a container
type ExecOpts struct {
	// Service is the service, as in 'web', 'db', 'dba'
	Service string
	// Dir is the working directory inside the container
	Dir string
	// Cmd is the string to execute
	Cmd string
	// Nocapture if true causes use of ComposeNoCapture, so the stdout and stderr go right to stdout/stderr
	NoCapture bool
	// Tty if true causes a tty to be allocated
	Tty bool
	// Stdout can be overridden with a File
	Stdout *os.File
	// Stderr can be overridden with a File
	Stderr *os.File
}

// Exec executes a given command in the container of given type without allocating a pty
// Returns ComposeCmd results of stdout, stderr, err
// If Nocapture arg is true, stdout/stderr will be empty and output directly to stdout/stderr
func (app *DdevApp) Exec(opts *ExecOpts) (string, string, error) {
	app.DockerEnv()

	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("app.Exec %v", opts))
	defer runTime()

	if opts.Service == "" {
		opts.Service = "web"
	}

	state, err := dockerutil.GetContainerStateByName(fmt.Sprintf("ddev-%s-%s", app.Name, opts.Service))
	if err != nil || state != "running" {
		if state == "doesnotexist" {
			return "", "", fmt.Errorf("service %s does not exist in project %s (state=%s)", opts.Service, app.Name, state)
		}
		return "", "", fmt.Errorf("service %s is not currently running in project %s (state=%s), use `ddev logs -s %s` to see what happened to it", opts.Service, app.Name, state, opts.Service)
	}

	err = app.ProcessHooks("pre-exec")
	if err != nil {
		return "", "", fmt.Errorf("Failed to process pre-exec hooks: %v", err)
	}

	exec := []string{"exec"}
	if workingDir := app.GetWorkingDir(opts.Service, opts.Dir); workingDir != "" {
		exec = append(exec, "-w", workingDir)
	}

	if !isatty.IsTerminal(os.Stdin.Fd()) || !opts.Tty {
		exec = append(exec, "-T")
	}

	exec = append(exec, opts.Service)

	if opts.Cmd == "" {
		return "", "", fmt.Errorf("no command provided")
	}

	// Cases to handle
	// - Free form, all unquoted. Like `ls -l -a`
	// - Quoted to delay pipes and other features to container, like `"ls -l -a | grep junk"`
	// Note that a set quoted on the host in ddev exec will come through as a single arg

	// Use bash for our containers, sh for 3rd-party containers
	// that may not have bash.
	shell := "bash"
	if !nodeps.ArrayContainsString([]string{"web", "db", "dba"}, opts.Service) {
		shell = "sh"
	}
	errcheck := "set -eu"
	exec = append(exec, shell, "-c", errcheck+` && ( `+opts.Cmd+`)`)

	files, err := app.ComposeFiles()
	if err != nil {
		return "", "", err
	}

	stdout := os.Stdout
	stderr := os.Stderr
	if opts.Stdout != nil {
		stdout = opts.Stdout
	}
	if opts.Stderr != nil {
		stderr = opts.Stderr
	}

	var stdoutResult, stderrResult string
	if opts.NoCapture || opts.Tty {
		err = dockerutil.ComposeWithStreams(files, os.Stdin, stdout, stderr, exec...)
	} else {
		stdoutResult, stderrResult, err = dockerutil.ComposeCmd([]string{app.DockerComposeFullRenderedYAMLPath()}, exec...)
	}

	hookErr := app.ProcessHooks("post-exec")
	if hookErr != nil {
		return stdoutResult, stderrResult, fmt.Errorf("Failed to process post-exec hooks: %v", hookErr)
	}

	return stdoutResult, stderrResult, err
}

// ExecWithTty executes a given command in the container of given type.
// It allocates a pty for interactive work.
func (app *DdevApp) ExecWithTty(opts *ExecOpts) error {
	app.DockerEnv()

	if opts.Service == "" {
		opts.Service = "web"
	}

	state, err := dockerutil.GetContainerStateByName(fmt.Sprintf("ddev-%s-%s", app.Name, opts.Service))
	if err != nil || state != "running" {
		return fmt.Errorf("service %s is not current running in project %s (state=%s)", opts.Service, app.Name, state)
	}

	exec := []string{"exec"}
	if workingDir := app.GetWorkingDir(opts.Service, opts.Dir); workingDir != "" {
		exec = append(exec, "-w", workingDir)
	}

	exec = append(exec, opts.Service)

	if opts.Cmd == "" {
		return fmt.Errorf("no command provided")
	}

	// Cases to handle
	// - Free form, all unquoted. Like `ls -l -a`
	// - Quoted to delay pipes and other features to container, like `"ls -l -a | grep junk"`
	// Note that a set quoted on the host in ddev exec will come through as a single arg

	// Use bash for our containers, sh for 3rd-party containers
	// that may not have bash.
	shell := "bash"
	if !nodeps.ArrayContainsString([]string{"web", "db", "dba"}, opts.Service) {
		shell = "sh"
	}
	exec = append(exec, shell, "-c", opts.Cmd)

	files, err := app.ComposeFiles()
	if err != nil {
		return err
	}

	return dockerutil.ComposeWithStreams(files, os.Stdin, os.Stdout, os.Stderr, exec...)
}

func (app *DdevApp) ExecOnHostOrService(service string, cmd string) error {
	var err error
	// Handle case on host
	if service == "host" {
		cwd, _ := os.Getwd()
		err = os.Chdir(app.GetAppRoot())
		if err != nil {
			return fmt.Errorf("Unable to GetAppRoot: %v", err)
		}
		bashPath := "bash"
		if runtime.GOOS == "windows" {
			bashPath = util.FindWindowsBashPath()
			if bashPath == "" {
				return fmt.Errorf("Unable to find bash.exe on Windows")
			}
		}

		args := []string{
			"-c",
			cmd,
		}

		err = exec.RunInteractiveCommand(bashPath, args)
		_ = os.Chdir(cwd)
	} else { // handle case in container
		_, _, err = app.Exec(
			&ExecOpts{
				Service: service,
				Cmd:     cmd,
				Tty:     isatty.IsTerminal(os.Stdin.Fd()),
			})
	}
	return err
}

// Logs returns logs for a site's given container.
// See docker.LogsOptions for more information about valid tailLines values.
func (app *DdevApp) Logs(service string, follow bool, timestamps bool, tailLines string) error {
	client := dockerutil.GetDockerClient()

	var container *docker.APIContainers
	var err error
	// Let people access ddev-router and ddev-ssh-agent logs as well.
	if service == "ddev-router" || service == "ddev-ssh-agent" {
		container, err = dockerutil.FindContainerByLabels(map[string]string{"com.docker.compose.service": service})
	} else {
		container, err = app.FindContainerByType(service)
	}
	if err != nil {
		return err
	}
	if container == nil {
		util.Warning("No running service container %s was found", service)
		return nil
	}

	logOpts := docker.LogsOptions{
		Container:    container.ID,
		Stdout:       true,
		Stderr:       true,
		OutputStream: output.UserOut.Out,
		ErrorStream:  output.UserOut.Out,
		Follow:       follow,
		Timestamps:   timestamps,
	}

	if tailLines != "" {
		logOpts.Tail = tailLines
	}

	err = client.Logs(logOpts)
	if err != nil {
		return err
	}

	return nil
}

// CaptureLogs returns logs for a site's given container.
// See docker.LogsOptions for more information about valid tailLines values.
func (app *DdevApp) CaptureLogs(service string, timestamps bool, tailLines string) (string, error) {
	client := dockerutil.GetDockerClient()

	var container *docker.APIContainers
	var err error
	// Let people access ddev-router and ddev-ssh-agent logs as well.
	if service == "ddev-router" || service == "ddev-ssh-agent" {
		container, err = dockerutil.FindContainerByLabels(map[string]string{"com.docker.compose.service": service})
	} else {
		container, err = app.FindContainerByType(service)
	}
	if err != nil {
		return "", err
	}
	if container == nil {
		util.Warning("No running service container %s was found", service)
		return "", nil
	}

	var out bytes.Buffer

	logOpts := docker.LogsOptions{
		Container:    container.ID,
		Stdout:       true,
		Stderr:       true,
		OutputStream: &out,
		ErrorStream:  &out,
		Follow:       false,
		Timestamps:   timestamps,
	}

	if tailLines != "" {
		logOpts.Tail = tailLines
	}

	err = client.Logs(logOpts)
	if err != nil {
		return "", err
	}

	return out.String(), nil
}

// DockerEnv sets environment variables for a docker-compose run.
func (app *DdevApp) DockerEnv() {

	uidStr, gidStr, _ := util.GetContainerUIDGid()

	// Warn about running as root if we're not on windows.
	if uidStr == "0" || gidStr == "0" {
		util.Warning("Warning: containers will run as root. This could be a security risk on Linux.")
	}

	// DDEV_HOST_DB_PORT is actually used for 2 things.
	// 1. To specify via base docker-compose file the value of host_db_port config. And it's expected to be empty
	//    there if the host_db_port is empty.
	// 2. To tell custom commands the db port. And it's expected always to be populated for them.
	dbPort, err := app.GetPublishedPort("db")
	dbPortStr := strconv.Itoa(dbPort)
	if dbPortStr == "-1" || err != nil {
		dbPortStr = ""
	}
	if app.HostDBPort != "" {
		dbPortStr = app.HostDBPort
	}

	envVars := map[string]string{
		// Without COMPOSE_DOCKER_CLI_BUILD=0, docker-cmpose makes all kinds of mess
		// of output. BUILDKIT_PROGRESS doesn't help either.
		"COMPOSE_DOCKER_CLI_BUILD":      "0",
		"COMPOSE_PROJECT_NAME":          "ddev-" + app.Name,
		"COMPOSE_CONVERT_WINDOWS_PATHS": "true",
		"DDEV_SITENAME":                 app.Name,
		"DDEV_TLD":                      app.ProjectTLD,
		"DDEV_DBIMAGE":                  app.GetDBImage(),
		"DDEV_DBAIMAGE":                 app.DBAImage,
		"DDEV_PROJECT":                  app.Name,
		"DDEV_WEBIMAGE":                 app.WebImage,
		"DDEV_APPROOT":                  app.AppRoot,
		"DDEV_FILES_DIR":                path.Join("/var/www/html", app.GetDocroot(), app.GetUploadDir()),

		"DDEV_HOST_DB_PORT":          dbPortStr,
		"DDEV_HOST_WEBSERVER_PORT":   app.HostWebserverPort,
		"DDEV_HOST_HTTPS_PORT":       app.HostHTTPSPort,
		"DDEV_PHPMYADMIN_PORT":       app.PHPMyAdminPort,
		"DDEV_PHPMYADMIN_HTTPS_PORT": app.PHPMyAdminHTTPSPort,
		"DDEV_MAILHOG_PORT":          app.MailhogPort,
		"DDEV_MAILHOG_HTTPS_PORT":    app.MailhogHTTPSPort,
		"DDEV_DOCROOT":               app.Docroot,
		"DDEV_HOSTNAME":              app.HostName(),
		"DDEV_UID":                   uidStr,
		"DDEV_GID":                   gidStr,
		"DDEV_PHP_VERSION":           app.PHPVersion,
		"DDEV_WEBSERVER_TYPE":        app.WebserverType,
		"DDEV_PROJECT_TYPE":          app.Type,
		"DDEV_ROUTER_HTTP_PORT":      app.RouterHTTPPort,
		"DDEV_ROUTER_HTTPS_PORT":     app.RouterHTTPSPort,
		"DDEV_XDEBUG_ENABLED":        strconv.FormatBool(app.XdebugEnabled),
		"DDEV_PRIMARY_URL":           app.GetPrimaryURL(),
		"DOCKER_SCAN_SUGGEST":        "false",
		"IS_DDEV_PROJECT":            "true",
	}

	// Set the mariadb_local command to empty to prevent docker-compose from complaining normally.
	// It's used for special startup on restoring to a snapshfot.
	if len(os.Getenv("DDEV_MARIADB_LOCAL_COMMAND")) == 0 {
		err := os.Setenv("DDEV_MARIADB_LOCAL_COMMAND", "")
		util.CheckErr(err)
	}

	// Find out terminal dimensions
	columns, lines, err := terminal.GetSize(0)
	if err != nil {
		columns = 80
		lines = 24
	}
	envVars["COLUMNS"] = strconv.Itoa(columns)
	envVars["LINES"] = strconv.Itoa(lines)

	if len(app.AdditionalHostnames) > 0 || len(app.AdditionalFQDNs) > 0 {
		envVars["DDEV_HOSTNAME"] = strings.Join(app.GetHostnames(), ",")
	}

	// Only set values if they don't already exist in env.
	for k, v := range envVars {
		if err := os.Setenv(k, v); err != nil {
			util.Error("Failed to set the environment variable %s=%s: %v", k, v, err)
		}
	}
}

// Pause initiates docker-compose stop
func (app *DdevApp) Pause() error {
	app.DockerEnv()

	if app.SiteStatus() == SiteStopped {
		return fmt.Errorf("no project to stop")
	}

	err := app.ProcessHooks("pre-pause")
	if err != nil {
		return err
	}

	if _, _, err := dockerutil.ComposeCmd([]string{app.DockerComposeFullRenderedYAMLPath()}, "stop"); err != nil {
		return err
	}
	err = app.ProcessHooks("post-pause")
	if err != nil {
		return err
	}

	return StopRouterIfNoContainers()
}

// WaitForServices waits for all the services in docker-compose to come up
func (app *DdevApp) WaitForServices() error {
	requiredContainers := []string{}
	if services, ok := app.ComposeYaml["services"].(map[interface{}]interface{}); ok {
		for k := range services {
			requiredContainers = append(requiredContainers, k.(string))
		}
	} else {
		util.Failed("unable to get required startup services to wait for")
	}
	output.UserOut.Printf("Waiting for these services to become ready: %v", requiredContainers)

	labels := map[string]string{
		"com.ddev.site-name": app.GetName(),
	}
	waitTime := containerWaitTimeout
	_, err := dockerutil.ContainerWait(waitTime, labels)
	if err != nil {
		return fmt.Errorf("timed out waiting for containers (%v) to start: err=%v", requiredContainers, err)
	}
	return nil
}

// Wait ensures that the app service containers are healthy.
func (app *DdevApp) Wait(requiredContainers []string) error {
	for _, containerType := range requiredContainers {
		labels := map[string]string{
			"com.ddev.site-name":         app.GetName(),
			"com.docker.compose.service": containerType,
		}
		waitTime := containerWaitTimeout
		logOutput, err := dockerutil.ContainerWait(waitTime, labels)
		if err != nil {
			return fmt.Errorf("%s container failed: log=%s, err=%v", containerType, logOutput, err)
		}
	}

	return nil
}

// WaitByLabels waits for containers found by list of labels to be
// ready
func (app *DdevApp) WaitByLabels(labels map[string]string) error {
	waitTime := containerWaitTimeout
	err := dockerutil.ContainersWait(waitTime, labels)
	if err != nil {
		// TODO: Improve this error message
		return fmt.Errorf("container failed to become healthy: err=%v", err)
	}
	return nil
}

// StartAndWait is primarily for use in tests.
// It does app.Start() but then waits for extra seconds
// before returning.
// extraSleep arg in seconds is the time to wait if > 0
func (app *DdevApp) StartAndWait(extraSleep int) error {
	err := app.Start()
	if err != nil {
		return err
	}
	if extraSleep > 0 {
		time.Sleep(time.Duration(extraSleep) * time.Second)
	}
	return nil
}

// DetermineSettingsPathLocation figures out the path to the settings file for
// an app based on the contents/existence of app.SiteSettingsPath and
// app.SiteDdevSettingsFile.
func (app *DdevApp) DetermineSettingsPathLocation() (string, error) {
	possibleLocations := []string{app.SiteSettingsPath, app.SiteDdevSettingsFile}
	for _, loc := range possibleLocations {
		// If the file doesn't exist, it's safe to use
		if !fileutil.FileExists(loc) {
			return loc, nil
		}

		// If the file does exist, check for a signature indicating it's managed by ddev.
		signatureFound, err := fileutil.FgrepStringInFile(loc, DdevFileSignature)
		util.CheckErr(err) // Really can't happen as we already checked for the file existence

		// If the signature was found, it's safe to use.
		if signatureFound {
			return loc, nil
		}
	}

	return "", fmt.Errorf("settings files already exist and are being managed by the user")
}

// Snapshot forces a mariadb snapshot of the db to be written into .ddev/db_snapshots
// Returns the dirname of the snapshot and err
func (app *DdevApp) Snapshot(snapshotName string) (string, error) {
	err := app.ProcessHooks("pre-snapshot")
	if err != nil {
		return "", fmt.Errorf("Failed to process pre-stop hooks: %v", err)
	}

	if snapshotName == "" {
		t := time.Now()
		snapshotName = app.Name + "_" + t.Format("20060102150405")
	}

	existingSnapshots, err := app.ListSnapshots()
	if err != nil {
		return "", err
	}
	if nodeps.ArrayContainsString(existingSnapshots, snapshotName) {
		return "", fmt.Errorf("snapshot %s already exists, please use another snapshot name or clean up snapshots with `ddev snapshot --cleanup`", snapshotName)
	}

	// Container side has to use path.Join instead of filepath.Join because they are
	// targeted at the linux filesystem, so won't work with filepath on Windows
	snapshotDir := path.Join("db_snapshots", snapshotName)
	hostSnapshotDir := filepath.Join(filepath.Dir(app.ConfigPath), snapshotDir)
	containerSnapshotDir := path.Join("/mnt/ddev_config", snapshotDir)
	err = os.MkdirAll(hostSnapshotDir, 0777)
	if err != nil {
		return snapshotName, err
	}

	// Ensure that db container is up.
	labels := map[string]string{"com.ddev.site-name": app.Name, "com.docker.compose.service": "db"}
	_, err = dockerutil.ContainerWait(containerWaitTimeout, labels)
	if err != nil {
		return "", fmt.Errorf("unable to snapshot database, \nyour project %v is not running. \nPlease start the project if you want to snapshot it. \nIf removing, you can remove without a snapshot using \n'ddev stop --remove-data --omit-snapshot', \nwhich will destroy your database", app.Name)
	}

	util.Warning("Creating database snapshot %s", snapshotName)
	stdout, stderr, err := app.Exec(&ExecOpts{
		Service: "db",
		Cmd:     fmt.Sprintf("$(/backuptool.sh) --backup --target-dir=%s --user=root --password=root --socket=/var/tmp/mysql.sock 2>/var/log/mariadbackup_backup_%s.log && cp /var/lib/mysql/db_mariadb_version.txt %s", containerSnapshotDir, snapshotName, containerSnapshotDir),
	})

	if err != nil {
		util.Warning("Failed to create snapshot: %v, stdout=%s, stderr=%s", err, stdout, stderr)
		return "", err
	}

	util.Success("Created database snapshot %s in %s", snapshotName, hostSnapshotDir)
	err = app.ProcessHooks("post-snapshot")
	if err != nil {
		return snapshotName, fmt.Errorf("Failed to process pre-stop hooks: %v", err)
	}
	return snapshotName, nil
}

// DeleteSnapshot removes the snapshot directory inside a project
func (app *DdevApp) DeleteSnapshot(snapshotName string) error {
	var err error
	err = app.ProcessHooks("pre-delete-snapshot")
	if err != nil {
		return fmt.Errorf("Failed to process pre-delete-snapshot hooks: %v", err)
	}

	snapshotDir := path.Join("db_snapshots", snapshotName)
	hostSnapshotDir := filepath.Join(filepath.Dir(app.ConfigPath), snapshotDir)

	if err = fileutil.PurgeDirectory(hostSnapshotDir); err != nil {
		return fmt.Errorf("Failed to purge contents of snapshot directory: %v", err)
	}

	if err = os.Remove(hostSnapshotDir); err != nil {
		return fmt.Errorf("Failed to delete snapshot directory: %v", err)
	}

	util.Success("Deleted database snapshot %s in %s", snapshotName, hostSnapshotDir)
	err = app.ProcessHooks("post-delete-snapshot")
	if err != nil {
		return fmt.Errorf("Failed to process post-delete-snapshot hooks: %v", err)
	}

	return nil

}

// GetLatestSnapshot returns the latest created snapshot of a project
func (app *DdevApp) GetLatestSnapshot() (string, error) {
	var snapshots []string

	snapshots, err := app.ListSnapshots()
	if err != nil {
		return "", err
	}

	if len(snapshots) == 0 {
		return "", fmt.Errorf("no snapshots found")
	}

	return snapshots[0], nil
}

// ListSnapshots returns a list of the names of all project snapshots
func (app *DdevApp) ListSnapshots() ([]string, error) {
	var err error
	var snapshots []string

	snapshotDir := filepath.Join(filepath.Dir(app.ConfigPath), "db_snapshots")

	if !fileutil.FileExists(snapshotDir) {
		return snapshots, nil
	}

	dirEntrySlice, err := os.ReadDir(snapshotDir)
	if err != nil {
		return snapshots, err
	}

	// Get the mod times aligned with the DirEntry slice for comparison below.
	dirEntryModTimeSlice := make([]time.Time, len(dirEntrySlice))
	for i, de := range dirEntrySlice {
		de, err := de.Info()
		if err != nil {
			return snapshots, err
		}
		dirEntryModTimeSlice[i] = de.ModTime()
	}

	// Sort snapshots by last modification time
	// we need that to detect the latest snapshot
	// first snapshot is the latest
	sort.Slice(dirEntrySlice, func(i, j int) bool {
		return dirEntryModTimeSlice[i].After(dirEntryModTimeSlice[j])
	})

	for _, de := range dirEntrySlice {
		if de.IsDir() {
			snapshots = append(snapshots, de.Name())
		}
	}

	return snapshots, nil
}

// RestoreSnapshot restores a mariadb snapshot of the db to be loaded
// The project must be stopped and docker volume removed and recreated for this to work.
func (app *DdevApp) RestoreSnapshot(snapshotName string) error {
	var err error
	err = app.ProcessHooks("pre-restore-snapshot")
	if err != nil {
		return fmt.Errorf("Failed to process pre-restore-snapshot hooks: %v", err)
	}

	currentDBVersion := nodeps.MariaDBDefaultVersion
	if app.MariaDBVersion != "" {
		currentDBVersion = app.MariaDBVersion
	} else if app.MySQLVersion != "" {
		currentDBVersion = app.MySQLVersion
	}

	snapshotDir := filepath.Join("db_snapshots", snapshotName)

	hostSnapshotDir := filepath.Join(app.AppConfDir(), snapshotDir)
	if !fileutil.FileExists(hostSnapshotDir) {
		return fmt.Errorf("Failed to find a snapshot in %s", hostSnapshotDir)
	}

	// Find out the mariadb version that correlates to the snapshot.
	versionFile := filepath.Join(hostSnapshotDir, "db_mariadb_version.txt")
	var snapshotDBVersion string
	if fileutil.FileExists(versionFile) {
		snapshotDBVersion, err = fileutil.ReadFileIntoString(versionFile)
		if err != nil {
			return fmt.Errorf("unable to read the version file in the snapshot (%s): %v", versionFile, err)
		}
	} else {
		snapshotDBVersion = "unknown"
	}
	snapshotDBVersion = strings.Trim(snapshotDBVersion, " \n\t")

	if snapshotDBVersion != currentDBVersion {
		return fmt.Errorf("snapshot %s is a DB server %s snapshot and is not compatible with the configured ddev DB server version (%s).  Please restore it using the DB version it was created with, and then you can try upgrading the ddev DB version", snapshotDir, snapshotDBVersion, currentDBVersion)
	}

	//dockerutil.GetAppContainers(app.Name)
	if app.SiteStatus() == SiteRunning || app.SiteStatus() == SitePaused {
		err := app.Stop(false, false)
		if err != nil {
			return fmt.Errorf("Failed to rm  project for RestoreSnapshot: %v", err)
		}
	}

	err = os.Setenv("DDEV_MARIADB_LOCAL_COMMAND", "restore_snapshot "+snapshotName)
	util.CheckErr(err)
	err = app.Start()
	if err != nil {
		return fmt.Errorf("Failed to start project for RestoreSnapshot: %v", err)
	}
	err = os.Unsetenv("DDEV_MARIADB_LOCAL_COMMAND")
	util.CheckErr(err)

	output.UserOut.Printf("Waiting for snapshot restore to complete...\nYou can also follow the restore progress in another terminal window with `ddev logs -s db -f %s`", app.Name)
	// Now it's up, but we need to find out when it finishes loading.
	for {
		// We used to use killall mysqld here, but docker-compose v1.28+
		// has a bug where it reports that kind of error on exit.
		// See https://github.com/docker/compose/issues/8169
		out, _, err := app.Exec(&ExecOpts{
			Cmd:     "pidof mysqld || true",
			Service: "db",
			Tty:     false,
		})
		if err != nil {
			return err
		}
		if out != "" {
			break
		}
		time.Sleep(1 * time.Second)
		fmt.Print(".")
	}
	util.Success("\nRestored database snapshot %s", hostSnapshotDir)
	err = app.ProcessHooks("post-restore-snapshot")
	if err != nil {
		return fmt.Errorf("Failed to process post-restore-snapshot hooks: %v", err)
	}
	return nil
}

// Stop stops and Removes the docker containers for the project in current directory.
func (app *DdevApp) Stop(removeData bool, createSnapshot bool) error {
	app.DockerEnv()
	var err error

	err = app.ProcessHooks("pre-stop")
	if err != nil {
		return fmt.Errorf("Failed to process pre-stop hooks: %v", err)
	}

	if createSnapshot == true {
		if app.SiteStatus() != SiteRunning {
			util.Warning("Must start non-running project to do database snapshot")
			err = app.Start()
			if err != nil {
				return fmt.Errorf("Failed to start project to perform database snapshot")
			}
		}
		t := time.Now()
		_, err = app.Snapshot(app.Name + "_remove_data_snapshot_" + t.Format("20060102150405"))
		if err != nil {
			return err
		}
	}

	if app.SiteStatus() == SiteRunning {
		err = app.Pause()
		if err != nil {
			util.Warning("Failed to pause containers for %s: %v", app.GetName(), err)
		}
	}
	// Remove all the containers and volumes for app.
	err = Cleanup(app)
	if err != nil {
		return err
	}

	// Remove data/database/projectInfo/hostname if we need to.
	if removeData {
		if err = app.RemoveHostsEntries(); err != nil {
			return fmt.Errorf("failed to remove hosts entries: %v", err)
		}
		app.RemoveGlobalProjectInfo()
		err = globalconfig.WriteGlobalConfig(globalconfig.DdevGlobalConfig)
		if err != nil {
			util.Warning("could not WriteGlobalConfig: %v", err)
		}

		for _, volName := range []string{app.Name + "-mariadb"} {
			err = dockerutil.RemoveVolume(volName)
			if err != nil {
				util.Warning("could not remove volume %s: %v", volName, err)
			} else {
				util.Success("Deleting database. Volume %s for project %s was deleted", volName, app.Name)
			}
		}
		desc, err := app.Describe(false)
		if err != nil {
			util.Warning("could not run app.Describe(): %v", err)
		}
		for extraService := range desc["extra_services"].(map[string]map[string]string) {
			// volName default if name: is not specified is ddev-<project>_volume
			volName := strings.ToLower("ddev-" + app.Name + "_" + extraService)
			if dockerutil.VolumeExists(volName) {
				err = dockerutil.RemoveVolume(volName)
				if err != nil {
					util.Warning("could not remove volume %s: %v", volName, err)
				} else {
					util.Success("Deleting third-party persistent volume %s for service %s...", volName, extraService)
				}
			}
		}
		dbBuilt := app.GetDBImage() + "-" + app.Name + "-built"
		_ = dockerutil.RemoveImage(dbBuilt)

		webBuilt := version.GetWebImage() + "-" + app.Name + "-built"
		_ = dockerutil.RemoveImage(webBuilt)
		util.Success("Project %s was deleted. Your code and configuration are unchanged.", app.Name)
	}

	err = app.ProcessHooks("post-stop")
	if err != nil {
		return fmt.Errorf("Failed to process post-stop hooks: %v", err)
	}

	return nil
}

// RemoveGlobalProjectInfo deletes the project from ProjectList
func (app *DdevApp) RemoveGlobalProjectInfo() {
	_ = globalconfig.RemoveProjectInfo(app.Name)
}

// GetHTTPURL returns the HTTP URL for an app.
func (app *DdevApp) GetHTTPURL() string {
	url := "http://" + app.GetHostname()
	if app.RouterHTTPPort != "80" {
		url = url + ":" + app.RouterHTTPPort
	}
	return url
}

// GetHTTPSURL returns the HTTPS URL for an app.
func (app *DdevApp) GetHTTPSURL() string {
	url := "https://" + app.GetHostname()
	if app.RouterHTTPSPort != "443" {
		url = url + ":" + app.RouterHTTPSPort
	}
	return url
}

// GetAllURLs returns an array of all the URLs for the project
func (app *DdevApp) GetAllURLs() (httpURLs []string, httpsURLs []string, allURLs []string) {

	// Get configured URLs
	for _, name := range app.GetHostnames() {
		httpPort := ""
		httpsPort := ""
		if app.RouterHTTPPort != "80" {
			httpPort = ":" + app.RouterHTTPPort
		}
		if app.RouterHTTPSPort != "443" {
			httpsPort = ":" + app.RouterHTTPSPort
		}

		httpsURLs = append(httpsURLs, "https://"+name+httpsPort)
		httpURLs = append(httpURLs, "http://"+name+httpPort)
	}

	httpsURLs = append(httpsURLs, app.GetWebContainerDirectHTTPSURL())
	httpURLs = append(httpURLs, app.GetWebContainerDirectHTTPURL())

	return httpURLs, httpsURLs, append(httpsURLs, httpURLs...)
}

// GetPrimaryURL returns the primary URL that can be used, https or http
func (app *DdevApp) GetPrimaryURL() string {
	httpURLs, httpsURLs, _ := app.GetAllURLs()
	urlList := httpsURLs
	// If no mkcert trusted https, use the httpURLs instead
	if globalconfig.GetCAROOT() == "" {
		urlList = httpURLs
	}
	return urlList[0]
}

// GetWebContainerDirectHTTPURL returns the URL that can be used without the router to get to web container.
func (app *DdevApp) GetWebContainerDirectHTTPURL() string {
	// Get direct address of web container
	dockerIP, err := dockerutil.GetDockerIP()
	if err != nil {
		util.Warning("Unable to get Docker IP: %v", err)
	}
	port, _ := app.GetWebContainerPublicPort()
	return fmt.Sprintf("http://%s:%d", dockerIP, port)
}

// GetWebContainerDirectHTTPSURL returns the URL that can be used without the router to get to web container via https.
func (app *DdevApp) GetWebContainerDirectHTTPSURL() string {
	// Get direct address of web container
	dockerIP, err := dockerutil.GetDockerIP()
	if err != nil {
		util.Warning("Unable to get Docker IP: %v", err)
	}
	port, _ := app.GetWebContainerHTTPSPublicPort()
	return fmt.Sprintf("https://%s:%d", dockerIP, port)
}

// GetWebContainerPublicPort returns the direct-access public tcp port for http
func (app *DdevApp) GetWebContainerPublicPort() (int, error) {

	webContainer, err := app.FindContainerByType("web")
	if err != nil || webContainer == nil {
		return -1, fmt.Errorf("Unable to find web container for app: %s, err %v", app.Name, err)
	}

	for _, p := range webContainer.Ports {
		if p.PrivatePort == 80 {
			return int(p.PublicPort), nil
		}
	}
	return -1, fmt.Errorf("No public port found for private port 80")
}

// GetWebContainerHTTPSPublicPort returns the direct-access public tcp port for https
func (app *DdevApp) GetWebContainerHTTPSPublicPort() (int, error) {

	webContainer, err := app.FindContainerByType("web")
	if err != nil || webContainer == nil {
		return -1, fmt.Errorf("Unable to find https web container for app: %s, err %v", app.Name, err)
	}

	for _, p := range webContainer.Ports {
		if p.PrivatePort == 443 {
			return int(p.PublicPort), nil
		}
	}
	return -1, fmt.Errorf("No public https port found for private port 443")
}

// HostName returns the hostname of a given application.
func (app *DdevApp) HostName() string {
	return app.GetHostname()
}

// AddHostsEntriesIfNeeded will (optionally) add the site URL to the host's /etc/hosts.
func (app *DdevApp) AddHostsEntriesIfNeeded() error {
	dockerIP, err := dockerutil.GetDockerIP()
	if err != nil {
		return fmt.Errorf("could not get Docker IP: %v", err)
	}

	hosts, err := ddevhosts.New()
	if err != nil {
		util.Failed("could not open hostfile: %v", err)
	}

	ipPosition := hosts.GetIPPosition(dockerIP)
	if ipPosition != -1 && runtime.GOOS == "windows" {
		hostsLine := hosts.Lines[ipPosition]
		if len(hostsLine.Hosts) >= 10 {
			util.Error("You have more than 9 entries in your (windows) hostsfile entry for %s", dockerIP)
			util.Error("Please use `ddev hostname --remove-inactive` or edit the hosts file manually")
			util.Error("Please see %s for more information", "https://ddev.readthedocs.io/en/stable/users/troubleshooting/#windows-hosts-file-limited")
		}
	}

	for _, name := range app.GetHostnames() {
		if app.UseDNSWhenPossible && globalconfig.IsInternetActive() {
			// If they have provided "*.<name>" then look up the suffix
			checkName := strings.TrimPrefix(name, "*.")
			hostIPs, err := net.LookupHost(checkName)

			// If we had successful lookup and dockerIP matches
			// with adding to hosts file.
			if err == nil && len(hostIPs) > 0 && hostIPs[0] == dockerIP {
				continue
			}
		}

		// We likely won't hit the hosts.Has() as true because
		// we already did a lookup. But check anyway.
		if hosts.Has(dockerIP, name) {
			continue
		}
		util.Warning("The hostname %s is not currently resolvable, trying to add it to the hosts file", name)
		err = addHostEntry(name, dockerIP)
		if err != nil {
			return err
		}
	}

	return nil
}

// addHostEntry adds an entry to /etc/hosts
// We would have hoped to use DNS or have found the entry already in hosts
// But if it's not, try to add one.
func addHostEntry(name string, ip string) error {
	_, err := osexec.LookPath("sudo")
	if (os.Getenv("DDEV_NONINTERACTIVE") != "") || err != nil {
		util.Warning("You must manually add the following entry to your hosts file:\n%s %s\nOr with root/administrative privileges execute 'ddev hostname %s %s'", ip, name, name, ip)
		if nodeps.IsWSL2() {
			util.Warning("For WSL2, if you use a Windows browser, execute 'sudo ddev hostname %s %s' on Windows", name, ip)
		}
		return nil
	}

	ddevFullpath, err := os.Executable()
	util.CheckErr(err)

	output.UserOut.Printf("ddev needs to add an entry to your hostfile.\nIt will require administrative privileges via the sudo command, so you may be required\nto enter your password for sudo. ddev is about to issue the command:")
	if nodeps.IsWSL2() {
		util.Warning("You are on WSL2, so should also manually execute 'sudo ddev hostname %s %s' on Windows if you use a Windows browser.", name, ip)
	}

	hostnameArgs := []string{ddevFullpath, "hostname", name, ip}
	command := strings.Join(hostnameArgs, " ")
	util.Warning(fmt.Sprintf("    sudo %s", command))
	output.UserOut.Println("Please enter your password if prompted.")
	_, err = exec.RunCommandPipe("sudo", hostnameArgs)
	if err != nil {
		util.Warning("Failed to execute sudo command, you will need to manually execute '%s' with administrative privileges", command)
	}
	return nil
}

// RemoveHostsEntries will remote the site URL from the host's /etc/hosts.
func (app *DdevApp) RemoveHostsEntries() error {
	dockerIP, err := dockerutil.GetDockerIP()
	if err != nil {
		return fmt.Errorf("could not get Docker IP: %v", err)
	}

	hosts, err := goodhosts.NewHosts()
	if err != nil {
		util.Failed("could not open hostfile: %v", err)
	}

	for _, name := range app.GetHostnames() {
		if !hosts.Has(dockerIP, name) {
			continue
		}

		_, err = osexec.LookPath("sudo")
		if os.Getenv("DDEV_NONINTERACTIVE") != "" || err != nil {
			util.Warning("You must manually remove the following entry from your hosts file:\n%s %s\nOr with root/administrative privileges execute 'ddev hostname --remove %s %s", dockerIP, name, name, dockerIP)
			return nil
		}

		ddevFullPath, err := os.Executable()
		util.CheckErr(err)

		output.UserOut.Printf("ddev needs to remove an entry from your hosts file.\nIt will require administrative privileges via the sudo command, so you may be required\nto enter your password for sudo. ddev is about to issue the command:")

		hostnameArgs := []string{ddevFullPath, "hostname", "--remove", name, dockerIP}
		command := strings.Join(hostnameArgs, " ")
		util.Warning(fmt.Sprintf("    sudo %s", command))
		output.UserOut.Println("Please enter your password if prompted.")

		if _, err = exec.RunCommandPipe("sudo", hostnameArgs); err != nil {
			util.Warning("Failed to execute sudo command, you will need to manually execute '%s' with administrative privileges", command)
		}
	}

	return nil
}

// GetActiveAppRoot returns the fully rooted directory of the active app, or an error
func GetActiveAppRoot(siteName string) (string, error) {
	var siteDir string
	var err error

	if siteName == "" {
		siteDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("error determining the current directory: %s", err)
		}
		_, err = CheckForConf(siteDir)
		if err != nil {
			return "", fmt.Errorf("Could not find a project in %s. Have you run 'ddev config'? Please specify a project name or change directories: %s", siteDir, err)
		}
	} else {
		var ok bool

		labels := map[string]string{
			"com.ddev.site-name":         siteName,
			"com.docker.compose.service": "web",
		}

		webContainer, err := dockerutil.FindContainerByLabels(labels)
		if err != nil {
			return "", err
		}
		if webContainer == nil {
			return "", fmt.Errorf("could not find a project named '%s'. Run 'ddev list' to see currently active projects", siteName)
		}

		siteDir, ok = webContainer.Labels["com.ddev.approot"]
		if !ok {
			return "", fmt.Errorf("could not determine the location of %s from container: %s", siteName, dockerutil.ContainerName(*webContainer))
		}
	}
	appRoot, err := CheckForConf(siteDir)
	if err != nil {
		return siteDir, err
	}

	return appRoot, nil
}

// GetActiveApp returns the active App based on the current working directory or running siteName provided.
// To use the current working directory, siteName should be ""
func GetActiveApp(siteName string) (*DdevApp, error) {
	app := &DdevApp{}
	activeAppRoot, err := GetActiveAppRoot(siteName)
	if err != nil {
		return app, err
	}

	// Mostly ignore app.Init() error, since app.Init() fails if no directory found. Some errors should be handled though.
	// We already were successful with *finding* the app, and if we get an
	// incomplete one we have to add to it.
	if err = app.Init(activeAppRoot); err != nil {
		switch err.(type) {
		case webContainerExists, invalidConfigFile, invalidHostname, invalidAppType, invalidPHPVersion, invalidWebserverType, invalidProvider:
			return app, err
		}
	}

	if app.Name == "" {
		err = restoreApp(app, siteName)
		if err != nil {
			return app, err
		}
	}

	return app, nil
}

// restoreApp recreates an AppConfig's Name and returns an error
// if it cannot restore them.
func restoreApp(app *DdevApp, siteName string) error {
	if siteName == "" {
		return fmt.Errorf("error restoring AppConfig: no project name given")
	}
	app.Name = siteName
	return nil
}

// GetProvider returns a pointer to the provider instance interface.
func (app *DdevApp) GetProvider(providerName string) (*Provider, error) {

	var p Provider
	var err error

	if providerName != "" && providerName != nodeps.ProviderDefault {
		p = Provider{
			ProviderType: providerName,
			app:          app,
		}
		err = p.Init(providerName, app)
	}

	app.ProviderInstance = &p
	return app.ProviderInstance, err
}

// GetWorkingDir will determine the appropriate working directory for an Exec/ExecWithTty command
// by consulting with the project configuration. If no dir is specified for the service, an
// empty string will be returned.
func (app *DdevApp) GetWorkingDir(service string, dir string) string {
	// Highest preference is for directories passed into the command directly
	if dir != "" {
		return dir
	}

	// The next highest preference is for directories defined in config.yaml
	if app.WorkingDir != nil {
		if workingDir := app.WorkingDir[service]; workingDir != "" {
			return workingDir
		}
	}

	// The next highest preference is for app type defaults
	return app.DefaultWorkingDirMap()[service]
}

// GetNFSMountVolName returns the docker volume name of the nfs mount volume
func (app *DdevApp) GetNFSMountVolName() string {
	return strings.ToLower("ddev-" + app.Name + "_nfsmount")
}

// StartAppIfNotRunning is intended to replace much-duplicated code in the commands.
func (app *DdevApp) StartAppIfNotRunning() error {
	var err error
	if app.SiteStatus() != SiteRunning {
		err = app.Start()
	}

	return err
}
