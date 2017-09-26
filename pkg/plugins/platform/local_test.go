package platform_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"strings"

	"github.com/drud/ddev/pkg/ddevapp"
	"github.com/drud/ddev/pkg/dockerutil"
	"github.com/drud/ddev/pkg/exec"
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/output"
	"github.com/drud/ddev/pkg/plugins/platform"
	"github.com/drud/ddev/pkg/testcommon"
	"github.com/drud/ddev/pkg/util"
	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
	asrt "github.com/stretchr/testify/assert"
)

var (
	TestSites = []testcommon.TestSite{
		{
			Name:                          "TestMainPkgWordpress",
			SourceURL:                     "https://github.com/drud/wordpress/archive/v0.4.0.tar.gz",
			ArchiveInternalExtractionPath: "wordpress-0.4.0/",
			FilesTarballURL:               "https://github.com/drud/wordpress/releases/download/v0.4.0/files.tar.gz",
			DBTarURL:                      "https://github.com/drud/wordpress/releases/download/v0.4.0/db.tar.gz",
			DocrootBase:                   "htdocs",
		},
		{
			Name:                          "TestMainPkgDrupal8",
			SourceURL:                     "https://ftp.drupal.org/files/projects/drupal-8.4.0.tar.gz",
			ArchiveInternalExtractionPath: "drupal-8.4.0/",
			FilesTarballURL:               "https://github.com/drud/drupal8/releases/download/v0.6.0/files.tar.gz",
			FilesZipballURL:               "https://github.com/drud/drupal8/releases/download/v0.6.0/files.zip",
			DBTarURL:                      "https://github.com/drud/drupal8/releases/download/v0.6.0/db.tar.gz",
			DBZipURL:                      "https://github.com/drud/drupal8/releases/download/v0.6.0/db.zip",
			FullSiteTarballURL:            "https://github.com/drud/drupal8/releases/download/v0.6.0/site.tar.gz",
			AppType:                       "drupal8",
		},
		{
			Name:                          "TestMainPkgDrupalKickstart",
			SourceURL:                     "https://github.com/drud/drupal-kickstart/archive/v0.4.0.tar.gz",
			ArchiveInternalExtractionPath: "drupal-kickstart-0.4.0/",
			FilesTarballURL:               "https://github.com/drud/drupal-kickstart/releases/download/v0.4.0/files.tar.gz",
			DBTarURL:                      "https://github.com/drud/drupal-kickstart/releases/download/v0.4.0/db.tar.gz",
			FullSiteTarballURL:            "https://github.com/drud/drupal-kickstart/releases/download/v0.4.0/site.tar.gz",
			DocrootBase:                   "docroot",
		},
	}
)

func TestMain(m *testing.M) {
	output.LogSetUp()

	// ensure we have docker network
	client := dockerutil.GetDockerClient()
	err := dockerutil.EnsureNetwork(client, dockerutil.NetName)
	if err != nil {
		log.Fatal(err)
	}

	count := len(platform.GetApps()["local"])
	if count > 0 {
		log.Fatalf("Local plugin tests require no sites running. You have %v site(s) running.", count)
	}

	if os.Getenv("GOTEST_SHORT") != "" {
		TestSites = []testcommon.TestSite{TestSites[0]}
	}

	for i := range TestSites {
		err := TestSites[i].Prepare()
		if err != nil {
			log.Fatalf("Prepare() failed on TestSite.Prepare() site=%s, err=%v", TestSites[i].Name, err)
		}

		switchDir := TestSites[i].Chdir()
		runTime := testcommon.TimeTrack(time.Now(), fmt.Sprintf("%s Start", TestSites[i].Name))

		testcommon.ClearDockerEnv()

		app, err := platform.GetPluginApp("local")
		if err != nil {
			log.Fatalf("TestMain startup: Platform.GetPluginApp(local) failed for site %s, err=%s", TestSites[i].Name, err)
		}

		err = app.Init(TestSites[i].Dir)
		if err != nil {
			log.Fatalf("TestMain startup: app.Init() failed on site %s in dir %s, err=%v", TestSites[i].Name, TestSites[i].Dir, err)
		}

		err = app.Start()
		if err != nil {
			log.Fatalf("TestMain startup: app.Start() failed on site %s, err=%v", TestSites[i].Name, err)
		}

		runTime()
		switchDir()
	}

	log.Debugln("Running tests.")
	testRun := m.Run()

	for i, site := range TestSites {
		runTime := testcommon.TimeTrack(time.Now(), fmt.Sprintf("%s Remove", site.Name))

		testcommon.ClearDockerEnv()

		app, err := platform.GetPluginApp("local")
		if err != nil {
			log.Fatalf("TestMain shutdown: Platform.GetPluginApp(local) failed for site %s, err=%s", TestSites[i].Name, err)
		}

		err = app.Init(site.Dir)
		if err != nil {
			log.Fatalf("TestMain shutdown: app.Init() failed on site %s in dir %s, err=%v", TestSites[i].Name, TestSites[i].Dir, err)
		}

		err = app.Down(true)
		if err != nil {
			log.Fatalf("TestMain startup: app.Down() failed on site %s, err=%v", TestSites[i].Name, err)
		}

		runTime()
		site.Cleanup()
	}

	os.Exit(testRun)
}

// TestLocalStart tests the functionality that is called when "ddev start" is executed
func TestLocalStart(t *testing.T) {
	assert := asrt.New(t)
	app, err := platform.GetPluginApp("local")
	assert.NoError(err)

	for _, site := range TestSites {
		switchDir := site.Chdir()
		runTime := testcommon.TimeTrack(time.Now(), fmt.Sprintf("%s LocalStart", site.Name))

		err = app.Init(site.Dir)
		assert.NoError(err)

		// ensure docker-compose.yaml exists inside .ddev site folder
		composeFile := fileutil.FileExists(app.DockerComposeYAMLPath())
		assert.True(composeFile)

		for _, containerType := range [3]string{"web", "db", "dba"} {
			containerName, err := getContainerByType(containerType, app)
			assert.NoError(err)
			check, err := testcommon.ContainerCheck(containerName, "running")
			assert.NoError(err)
			assert.True(check, "Container check on %s failed", containerType)
		}

		runTime()
		switchDir()
	}

	// try to start a site of same name at different path
	another := TestSites[0]
	err = another.Prepare()
	if err != nil {
		assert.FailNow("TestLocalStart: Prepare() failed on another.Prepare(), err=%v", err)
		return
	}

	err = app.Init(another.Dir)
	assert.Error(err)
	assert.Contains(err.Error(), fmt.Sprintf("container in running state already exists for %s that was created at %s", TestSites[0].Name, TestSites[0].Dir))
	testcommon.CleanupDir(another.Dir)
}

// TestStartWithoutDdev makes sure we don't have a regression where lack of .ddev
// causes a panic.
func TestStartWithoutDdevConfig(t *testing.T) {
	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)
	testDir := testcommon.CreateTmpDir("TestStartWithoutDdevConfig")

	// testcommon.Chdir()() and CleanupDir() check their own errors (and exit)
	defer testcommon.CleanupDir(testDir)
	defer testcommon.Chdir(testDir)()

	err := os.MkdirAll(testDir+"/sites/default", 0777)
	assert.NoError(err)
	err = os.Chdir(testDir)
	assert.NoError(err)

	_, err = platform.GetActiveApp("")
	assert.Error(err)
	assert.Contains(err.Error(), "unable to determine")
}

// TestGetApps tests the GetApps function to ensure it accurately returns a list of running applications.
func TestGetApps(t *testing.T) {
	assert := asrt.New(t)
	apps := platform.GetApps()
	assert.Equal(len(apps["local"]), len(TestSites))

	for _, site := range TestSites {
		var found bool
		for _, siteInList := range apps["local"] {
			if site.Name == siteInList.GetName() {
				found = true
				break
			}
		}
		assert.True(found, "Found site %s in list", site.Name)
	}
}

// TestLocalImportDB tests the functionality that is called when "ddev import-db" is executed
func TestLocalImportDB(t *testing.T) {
	assert := asrt.New(t)
	app, err := platform.GetPluginApp("local")
	assert.NoError(err)
	testDir, _ := os.Getwd()

	for _, site := range TestSites {
		switchDir := site.Chdir()
		runTime := testcommon.TimeTrack(time.Now(), fmt.Sprintf("%s LocalImportDB", site.Name))

		testcommon.ClearDockerEnv()
		err = app.Init(site.Dir)
		assert.NoError(err)

		// Test simple db loads.
		for _, file := range []string{"users.sql", "users.sql.gz", "users.sql.tar", "users.sql.tar.gz", "users.sql.tgz", "users.sql.zip"} {
			path := filepath.Join(testDir, "testdata", file)
			err = app.ImportDB(path, "")
			assert.NoError(err, "Failed to app.ImportDB path: %s err: %v", path, err)
		}

		if site.DBTarURL != "" {
			_, cachedArchive, err := testcommon.GetCachedArchive(site.Name, site.Name+"_siteTarArchive", "", site.DBTarURL)
			assert.NoError(err)
			err = app.ImportDB(cachedArchive, "")
			assert.NoError(err)

			out, _, err := app.Exec("db", "mysql", "-e", "SHOW TABLES;")
			assert.NoError(err)

			assert.Contains(out, "Tables_in_db")
			assert.False(strings.Contains(out, "Empty set"))

			assert.NoError(err)
		}

		if site.DBZipURL != "" {
			_, cachedArchive, err := testcommon.GetCachedArchive(site.Name, site.Name+"_siteZipArchive", "", site.DBZipURL)

			assert.NoError(err)
			err = app.ImportDB(cachedArchive, "")
			assert.NoError(err)

			out, _, err := app.Exec("db", "mysql", "-e", "SHOW TABLES;")
			assert.NoError(err)

			assert.Contains(out, "Tables_in_db")
			assert.False(strings.Contains(out, "Empty set"))
		}

		if site.FullSiteTarballURL != "" {
			_, cachedArchive, err := testcommon.GetCachedArchive(site.Name, site.Name+"_FullSiteTarballURL", "", site.FullSiteTarballURL)
			assert.NoError(err)

			err = app.ImportDB(cachedArchive, "data.sql")
			assert.NoError(err, "Failed to find data.sql at root of tarball %s", cachedArchive)
		}

		runTime()
		switchDir()
	}
}

// TestLocalImportFiles tests the functionality that is called when "ddev import-files" is executed
func TestLocalImportFiles(t *testing.T) {
	assert := asrt.New(t)
	app, err := platform.GetPluginApp("local")
	assert.NoError(err)

	for _, site := range TestSites {
		switchDir := site.Chdir()
		runTime := testcommon.TimeTrack(time.Now(), fmt.Sprintf("%s LocalImportFiles", site.Name))

		testcommon.ClearDockerEnv()
		err = app.Init(site.Dir)
		assert.NoError(err)

		if site.FilesTarballURL != "" {
			_, tarballPath, err := testcommon.GetCachedArchive(site.Name, "local-tarballs-files", "", site.FilesTarballURL)
			assert.NoError(err)
			err = app.ImportFiles(tarballPath, "")
			assert.NoError(err)
		}

		if site.FilesZipballURL != "" {
			_, zipballPath, err := testcommon.GetCachedArchive(site.Name, "local-zipballs-files", "", site.FilesZipballURL)
			assert.NoError(err)
			err = app.ImportFiles(zipballPath, "")
			assert.NoError(err)
		}

		if site.FullSiteTarballURL != "" {
			_, siteTarPath, err := testcommon.GetCachedArchive(site.Name, "local-site-tar", "", site.FullSiteTarballURL)
			assert.NoError(err)
			err = app.ImportFiles(siteTarPath, "docroot/sites/default/files")
			assert.NoError(err)
		}

		runTime()
		switchDir()
	}
}

// TestLocalExec tests the execution of commands inside a docker container of a site.
func TestLocalExec(t *testing.T) {
	assert := asrt.New(t)
	app, err := platform.GetPluginApp("local")
	assert.NoError(err)

	for _, site := range TestSites {
		switchDir := site.Chdir()
		runTime := testcommon.TimeTrack(time.Now(), fmt.Sprintf("%s LocalExec", site.Name))

		err := app.Init(site.Dir)
		assert.NoError(err)

		out, _, err := app.Exec("web", "pwd")
		assert.NoError(err)
		assert.Contains(out, "/var/www/html")

		_, _, err = app.Exec("db", "mysql", "-e", "DROP DATABASE db;")
		assert.NoError(err)
		_, _, err = app.Exec("db", "mysql", "information_schema", "-e", "CREATE DATABASE db;")
		assert.NoError(err)

		switch app.GetType() {
		case "drupal7":
			fallthrough
		case "drupal8":
			out, _, err = app.Exec("web", "drush", "status")
			assert.NoError(err)
		case "wordpress":
			out, _, err = app.Exec("web", "wp", "--info")
			assert.NoError(err)
		default:
		}

		assert.Regexp("/etc/php.*cli/php.ini", out)

		runTime()
		switchDir()

	}
}

// TestLocalLogs tests the container log output functionality.
func TestLocalLogs(t *testing.T) {
	assert := asrt.New(t)

	app, err := platform.GetPluginApp("local")
	assert.NoError(err)

	for _, site := range TestSites {
		switchDir := site.Chdir()
		runTime := testcommon.TimeTrack(time.Now(), fmt.Sprintf("%s LocalLogs", site.Name))

		err := app.Init(site.Dir)
		assert.NoError(err)

		stdout := testcommon.CaptureUserOut()
		err = app.Logs("web", false, false, "")
		assert.NoError(err)
		out := stdout()
		assert.Contains(out, "Server started")

		stdout = testcommon.CaptureUserOut()
		err = app.Logs("db", false, false, "")
		assert.NoError(err)
		out = stdout()
		assert.Contains(out, "Database initialized")

		stdout = testcommon.CaptureUserOut()
		err = app.Logs("db", false, false, "2")
		assert.NoError(err)
		out = stdout()
		assert.Contains(out, "MySQL init process done. Ready for start up.")

		runTime()
		switchDir()
	}
}

// TestProcessHooks tests execution of commands defined in config
func TestProcessHooks(t *testing.T) {
	assert := asrt.New(t)

	for _, site := range TestSites {
		cleanup := site.Chdir()
		runTime := testcommon.TimeTrack(time.Now(), fmt.Sprintf("%s ProcessHooks", site.Name))

		testcommon.ClearDockerEnv()
		conf, err := ddevapp.NewConfig(site.Dir, ddevapp.DefaultProviderName)
		assert.NoError(err)

		conf.Commands = map[string][]ddevapp.Command{
			"hook-test": {
				{
					Exec: "pwd",
				},
				{
					ExecHost: "pwd",
				},
			},
		}

		l := &platform.LocalApp{
			AppConfig: conf,
		}

		stdout := testcommon.CaptureUserOut()
		err = l.ProcessHooks("hook-test")
		assert.NoError(err)
		out := stdout()

		assert.Contains(out, "--- Running exec command: pwd ---")
		assert.Contains(out, "--- Running host command: pwd ---")

		runTime()
		cleanup()

	}
}

// TestLocalStop tests the functionality that is called when "ddev stop" is executed
func TestLocalStop(t *testing.T) {
	assert := asrt.New(t)

	app, err := platform.GetPluginApp("local")
	assert.NoError(err)

	for _, site := range TestSites {
		switchDir := site.Chdir()
		runTime := testcommon.TimeTrack(time.Now(), fmt.Sprintf("%s LocalStop", site.Name))

		testcommon.ClearDockerEnv()
		err := app.Init(site.Dir)
		assert.NoError(err)

		err = app.Stop()
		assert.NoError(err)

		for _, containerType := range [3]string{"web", "db", "dba"} {
			containerName, err := getContainerByType(containerType, app)
			assert.NoError(err)
			check, err := testcommon.ContainerCheck(containerName, "exited")
			assert.NoError(err)
			assert.True(check, containerType, "container has exited")
		}

		runTime()
		switchDir()
	}
}

// TestLocalStopMissingDirectory tests that the 'ddev stop' command works properly on sites with missing directories or ddev configs.
func TestLocalStopMissingDirectory(t *testing.T) {
	assert := asrt.New(t)

	site := TestSites[0]
	testcommon.ClearDockerEnv()
	app, err := platform.GetPluginApp("local")
	assert.NoError(err)
	err = app.Init(site.Dir)
	assert.NoError(err)

	// Restart the site since it was stopped in the previous test.
	if app.SiteStatus() != platform.SiteRunning {
		err = app.Start()
		assert.NoError(err)
	}

	tempPath := testcommon.CreateTmpDir("site-copy")
	siteCopyDest := filepath.Join(tempPath, "site")
	defer removeAllErrCheck(tempPath, assert)

	// Move the site directory to a temp location to mimic a missing directory.
	err = os.Rename(site.Dir, siteCopyDest)
	assert.NoError(err)

	out := app.Stop()
	assert.Error(out)
	assert.Contains(out.Error(), "If you would like to continue using ddev to manage this site please restore your files to that directory.")
	// Move the site directory back to its original location.
	err = os.Rename(siteCopyDest, site.Dir)
	assert.NoError(err)

}

// TestDescribe tests that the describe command works properly on a stopped site.
func TestDescribe(t *testing.T) {
	assert := asrt.New(t)
	app, err := platform.GetPluginApp("local")
	assert.NoError(err)

	for _, site := range TestSites {
		switchDir := site.Chdir()

		testcommon.ClearDockerEnv()
		err := app.Init(site.Dir)
		assert.NoError(err)

		// It should already be running, but start does no harm.
		err = app.Start()
		assert.NoError(err)

		desc, err := app.Describe()
		assert.NoError(err)
		assert.EqualValues(desc["status"], platform.SiteRunning)
		assert.EqualValues(app.GetName(), desc["name"])
		assert.EqualValues(platform.RenderHomeRootedDir(app.AppRoot()), desc["approot"])
		// Now stop it and test behavior.
		err = app.Stop()
		assert.NoError(err)

		desc, err = app.Describe()
		assert.NoError(err)
		assert.EqualValues(desc["status"], platform.SiteStopped)
		switchDir()
	}
}

// TestDescribeMissingDirectory tests that the describe command works properly on sites with missing directories or ddev configs.
func TestDescribeMissingDirectory(t *testing.T) {
	assert := asrt.New(t)
	site := TestSites[0]
	tempPath := testcommon.CreateTmpDir("site-copy")
	siteCopyDest := filepath.Join(tempPath, "site")
	defer removeAllErrCheck(tempPath, assert)

	app, err := platform.GetPluginApp("local")
	assert.NoError(err)
	err = app.Init(site.Dir)
	assert.NoError(err)

	// Move the site directory to a temp location to mimick a missing directory.
	err = os.Rename(site.Dir, siteCopyDest)
	assert.NoError(err)

	out, err := app.Describe()
	assert.NoError(err)
	assert.Contains(out, platform.SiteDirMissing, "Output did not include the phrase 'app directory missing' when describing a site with missing directories.")
	// Move the site directory back to its original location.
	err = os.Rename(siteCopyDest, site.Dir)
	assert.NoError(err)
}


// TestRouterPortsCheck makes sure that we can detect if the ports are available before starting the router.
func TestRouterPortsCheck(t *testing.T) {
	assert := asrt.New(t)

	// First, stop any sites that might be running
	app, err := platform.GetPluginApp("local")
	assert.NoError(err)

	// Stop all sites, which should get the router out of there.
	for _, site := range TestSites {
		switchDir := site.Chdir()

		testcommon.ClearDockerEnv()
		err := app.Init(site.Dir)
		assert.NoError(err)

		err = app.Stop()
		assert.NoError(err)

		switchDir()
	}

	// Now start one site, it's hard to get router to behave without one site.
	site := TestSites[0]
	testcommon.ClearDockerEnv()

	app, err = platform.GetActiveApp(site.Name)
	if err != nil {
		t.Fatalf("Failed to GetActiveApp(%s), err:%v", site.Name, err)
	}
	err = app.Start()
	assert.NoError(err, "app.Start(%s) failed, err: %v", app.GetName(), err)

	// Stop the router using code from platform.StopRouter().
	// platform.StopRouter can't be used here because it checks to see if containers are running
	// and doesn't do its job as a result.
	dest := platform.RouterComposeYAMLPath()
	_, _, err = dockerutil.ComposeCmd([]string{dest}, "-p", platform.RouterProjectName, "down", "-v")
	assert.NoError(err, "Failed to stop router using docker-compose, err=%v", err)

	// Occupy port 80 using docker busybox trick, then see if we can start router.
	// This is done with docker so that we don't have to use explicit sudo
	containerId, err := exec.RunCommand("sh", []string{"-c", "docker run -d -p80:80 --rm busybox:latest sleep 100 2>/dev/null"})
	if err != nil {
		t.Fatalf("Failed to run docker command to occupy port 80, err=%v output=%v", err, containerId)
	}
	containerId = strings.TrimSpace(containerId)

	// Now try to start the router. It should fail because the port is occupied.
	err = platform.StartDdevRouter()
	assert.Error(err, "Failure: router started even though port 80 was occupied")

	// Remove our dummy busybox docker container.
	out, err := exec.RunCommand("docker", []string{"rm", "-f", containerId})
	assert.NoError(err, "Failed to docker rm the port-occupier container, err=%v output=%v", err, out)
}


// TestCleanupWithoutCompose ensures app containers can be properly cleaned up without a docker-compose config file present.
func TestCleanupWithoutCompose(t *testing.T) {
	assert := asrt.New(t)
	// Skip test because we can't rename folders while they're in use if running on Windows.
	if runtime.GOOS == "windows" {
		log.Println("Skipping test TestCleanupWithoutCompose on Windows")
		t.Skip()
	}

	site := TestSites[0]

	revertDir := site.Chdir()
	app, err := platform.GetPluginApp("local")
	assert.NoError(err)

	testcommon.ClearDockerEnv()
	err = app.Init(site.Dir)
	assert.NoError(err)

	// Ensure we have a site started so we have something to cleanup
	err = app.Start()
	assert.NoError(err)
	// Setup by creating temp directory and nesting a folder for our site.
	tempPath := testcommon.CreateTmpDir("site-copy")
	siteCopyDest := filepath.Join(tempPath, "site")
	defer removeAllErrCheck(tempPath, assert)

	// Move site directory to a temp directory to mimick a missing directory.
	err = os.Rename(site.Dir, siteCopyDest)
	assert.NoError(err)

	// Call the Down command()
	// Notice that we set the removeData parameter to true.
	// This gives us added test coverage over sites with missing directories
	// by ensuring any associated database files get cleaned up as well.
	err = app.Down(true)
	assert.NoError(err)

	for _, containerType := range [3]string{"web", "db", "dba"} {
		_, err := getContainerByType(containerType, app)
		assert.Error(err)
	}

	// Ensure there are no volumes associated with this project
	client := dockerutil.GetDockerClient()
	volumes, err := client.ListVolumes(docker.ListVolumesOptions{})
	assert.NoError(err)
	for _, volume := range volumes {
		assert.False(volume.Labels["com.docker.compose.project"] == "ddev"+strings.ToLower(app.GetName()))
	}

	// Cleanup the global site database dirs. This does the work instead of running site.Cleanup()
	// because site.Cleanup() removes site directories that we'll need in other tests.
	dir := filepath.Join(util.GetGlobalDdevDir(), site.Name)
	err = os.RemoveAll(dir)
	assert.NoError(err)

	revertDir()
	// Move the site directory back to its original location.
	err = os.Rename(siteCopyDest, site.Dir)
	assert.NoError(err)
}

// TestGetappsEmpty ensures that GetApps returns an empty list when no applications are running.
func TestGetAppsEmpty(t *testing.T) {
	assert := asrt.New(t)

	// Ensure test sites are removed
	for _, site := range TestSites {
		app, err := platform.GetPluginApp("local")
		assert.NoError(err)

		switchDir := site.Chdir()

		testcommon.ClearDockerEnv()
		err = app.Init(site.Dir)
		assert.NoError(err)

		err = app.Down(true)
		assert.NoError(err)

		switchDir()
	}

	apps := platform.GetApps()
	assert.Equal(len(apps["local"]), 0, "Expected to find no apps but found %d apps=%v", len(apps["local"]), apps["local"])
}

// TestRouterNotRunning ensures the router is shut down after all sites are stopped.
func TestRouterNotRunning(t *testing.T) {
	assert := asrt.New(t)
	containers, err := dockerutil.GetDockerContainers(false)
	assert.NoError(err)

	for _, container := range containers {
		assert.NotEqual(dockerutil.ContainerName(container), "ddev-router", "ddev-router was not supposed to be running but it was")
	}
}

// TestListWithoutDir prevents regression where ddev list panics if one of the
// sites found is missing a directory
func TestListWithoutDir(t *testing.T) {
	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)
	testcommon.ClearDockerEnv()
	packageDir, _ := os.Getwd()

	// startCount is the count of apps at the start of this adventure
	apps := platform.GetApps()
	startCount := len(apps["local"])

	testDir := testcommon.CreateTmpDir("TestStartWithoutDdevConfig")
	defer testcommon.CleanupDir(testDir)

	err := os.MkdirAll(testDir+"/sites/default", 0777)
	assert.NoError(err)
	err = os.Chdir(testDir)
	assert.NoError(err)

	config, err := ddevapp.NewConfig(testDir, "")
	assert.NoError(err)
	config.Name = "junk"
	config.AppType = "drupal7"
	err = config.Write()
	assert.NoError(err)

	// Do a start on the configured site.
	app, err := platform.GetActiveApp("")
	assert.NoError(err)
	err = app.Start()
	assert.NoError(err)

	// Make sure we move out of the directory for Windows' sake
	garbageDir := testcommon.CreateTmpDir("RestingHere")
	defer testcommon.CleanupDir(garbageDir)

	err = os.Chdir(garbageDir)
	assert.NoError(err)

	testcommon.CleanupDir(testDir)

	apps = platform.GetApps()

	assert.EqualValues(len(apps["local"]), startCount+1)

	// Make a whole table and make sure our app directory missing shows up.
	// This could be done otherwise, but we'd have to go find the site in the
	// array first.
	table := platform.CreateAppTable()
	for _, site := range apps["local"] {
		desc, err := site.Describe()
		if err != nil {
			t.Fatalf("Failed to describe site %s: %v", site.GetName(), err)
		}

		platform.RenderAppRow(table, desc)
	}
	assert.Contains(table.String(), fmt.Sprintf("app directory missing: %s", testDir))

	err = app.Down(true)
	assert.NoError(err)

	// Change back to package dir. Lots of things will have to be cleaned up
	// in defers, and for windows we have to not be sitting in them.
	err = os.Chdir(packageDir)
	assert.NoError(err)
}

// getContainerByType builds a container name given the type (web/db/dba) and the app
func getContainerByType(containerType string, app platform.App) (string, error) {
	container, err := app.FindContainerByType(containerType)
	if err != nil {
		return "", err
	}
	name := dockerutil.ContainerName(container)
	return name, nil
}

func removeAllErrCheck(path string, assert *asrt.Assertions) {
	err := os.RemoveAll(path)
	assert.NoError(err)
}
