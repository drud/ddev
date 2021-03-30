package cmd

import (
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/nodeps"
	"github.com/drud/ddev/pkg/version"
	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/require"
	"testing"

	"os"
	"path/filepath"

	"strings"

	"fmt"

	"github.com/drud/ddev/pkg/ddevapp"
	"github.com/drud/ddev/pkg/exec"
	"github.com/drud/ddev/pkg/testcommon"
	asrt "github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

// TestCmdConfigHooks tests that pre-config and post-config hooks run
func TestCmdConfigHooks(t *testing.T) {
	// Change to the first DevTestSite for the duration of this test.
	site := TestSites[0]
	defer site.Chdir()()
	assert := asrt.New(t)

	app, err := ddevapp.NewApp(site.Dir, true)
	assert.NoError(err)
	app.Hooks = map[string][]ddevapp.YAMLTask{"post-config": {{"exec-host": "touch hello-post-config-" + app.Name}}, "pre-config": {{"exec-host": "touch hello-pre-config-" + app.Name}}}
	err = app.WriteConfig()
	assert.NoError(err)
	// Make sure we get rid of this for other uses
	defer func() {
		app.Hooks = nil
		_ = app.WriteConfig()
	}()

	_, err = exec.RunCommand(DdevBin, []string{"config", "--project-type=" + app.Type})
	assert.NoError(err)

	assert.FileExists("hello-pre-config-" + app.Name)
	assert.FileExists("hello-post-config-" + app.Name)
	err = os.Remove("hello-pre-config-" + app.Name)
	assert.NoError(err)
	err = os.Remove("hello-post-config-" + app.Name)
	assert.NoError(err)
}

// TestConfigDescribeLocation tries out the --show-config-location flag.
func TestConfigDescribeLocation(t *testing.T) {
	assert := asrt.New(t)

	// Create a temporary directory and switch to it.
	tmpdir := testcommon.CreateTmpDir("config-show-location")
	defer testcommon.CleanupDir(tmpdir)
	defer testcommon.Chdir(tmpdir)()

	// Create a config
	args := []string{"config", "--docroot=."}
	out, err := exec.RunCommand(DdevBin, args)
	assert.NoError(err)
	assert.Contains(string(out), "Found a php codebase")

	// Now see if we can detect it
	args = []string{"config", "--show-config-location"}
	out, err = exec.RunCommand(DdevBin, args)
	assert.NoError(err)
	assert.Contains(string(out), tmpdir)

	// Now try it in a directory that doesn't have a config
	tmpdir = testcommon.CreateTmpDir("config_show_location")
	defer testcommon.CleanupDir(tmpdir)
	defer testcommon.Chdir(tmpdir)()

	args = []string{"config", "--show-config-location"}
	out, err = exec.RunCommand(DdevBin, args)
	assert.Error(err)
	assert.Contains(string(out), "No project configuration currently exists")
}

// TestConfigWithSitenameFlagDetectsDocroot tests that the docroot is detected when
// flags like --project-name are passed.
func TestConfigWithSitenameFlagDetectsDocroot(t *testing.T) {
	assert := asrt.New(t)

	// Create a temporary directory and switch to it.
	testDocrootName := "web"
	tmpdir := testcommon.CreateTmpDir(t.Name())
	defer testcommon.CleanupDir(tmpdir)
	defer testcommon.Chdir(tmpdir)()
	// Create a document root folder.

	expectedFile := filepath.Join(tmpdir, testDocrootName, "misc/ahah.js")
	err := os.MkdirAll(filepath.Dir(expectedFile), 0777)
	assert.NoError(err)

	// create index.php that defines docroot
	_, err = os.OpenFile(filepath.Join(tmpdir, testDocrootName, "index.php"), os.O_RDONLY|os.O_CREATE, 0666)
	assert.NoError(err)

	// create the misc/ahah.js that signals drupal6
	_, err = os.OpenFile(expectedFile, os.O_RDONLY|os.O_CREATE, 0666)
	assert.NoError(err)

	// Create a config
	args := []string{"config", "--project-name=config-with-sitename", "--php-version=7.2"}
	out, err := exec.RunCommand(DdevBin, args)
	assert.NoError(err)
	defer func() {
		_, _ = exec.RunCommand(DdevBin, []string{"delete", "-Oy", "config-with-sitename"})
	}()
	assert.Contains(string(out), "Found a drupal6 codebase", nodeps.AppTypeDrupal6)
}

// TestConfigSetValues sets all available configuration values using command flags, then confirms that the
// values have been correctly written to the config file.
func TestConfigSetValues(t *testing.T) {
	var err error
	assert := asrt.New(t)

	// Create a temporary directory and switch to it.
	tmpdir := testcommon.CreateTmpDir(t.Name())
	defer testcommon.CleanupDir(tmpdir)
	defer testcommon.Chdir(tmpdir)()

	// Create an existing docroot
	docroot := "web"
	if err = os.MkdirAll(filepath.Join(tmpdir, docroot), 0755); err != nil {
		t.Errorf("Could not create docroot %s in %s", docroot, tmpdir)
	}

	err = os.Chdir(tmpdir)
	assert.NoError(err)

	// Build config args
	projectName := "my-project-name"
	projectType := nodeps.AppTypeTYPO3
	phpVersion := nodeps.PHP71
	httpPort := "81"
	httpsPort := "444"
	hostDBPort := "60001"
	hostWebserverPort := "60002"
	hostHTTPSPort := "60003"
	xdebugEnabled := true
	noProjectMount := true
	composerVersion := "2.0.0-RC2"
	additionalHostnamesSlice := []string{"abc", "123", "xyz"}
	additionalHostnames := strings.Join(additionalHostnamesSlice, ",")
	additionalFQDNsSlice := []string{"abc.com", "123.pizza", "xyz.co.uk"}
	additionalFQDNs := strings.Join(additionalFQDNsSlice, ",")
	omitContainersSlice := []string{"dba", "ddev-ssh-agent"}
	omitContainers := strings.Join(omitContainersSlice, ",")
	webimageExtraPackagesSlice := []string{"php-bcmath", "php7.3-tidy"}
	webimageExtraPackages := strings.Join(webimageExtraPackagesSlice, ",")
	dbimageExtraPackagesSlice := []string{"netcat", "ncdu"}
	dbimageExtraPackages := strings.Join(dbimageExtraPackagesSlice, ",")

	uploadDir := filepath.Join("custom", "config", "path")
	webserverType := nodeps.WebserverApacheFPM
	webImage := "custom-web-image"
	dbImage := "custom-db-image"
	dbaImage := "custom-dba-image"
	webWorkingDir := "/custom/web/dir"
	dbWorkingDir := "/custom/db/dir"
	dbaWorkingDir := "/custom/dba/dir"
	phpMyAdminPort := "5000"
	mailhogPort := "5001"
	projectTLD := "nowhere.example.com"
	useDNSWhenPossible := false
	timezone := "America/Chicago"
	webEnv := "SOMEENV=some+val"

	args := []string{
		"config",
		"--project-name", projectName,
		"--docroot", docroot,
		"--project-type", projectType,
		"--php-version", phpVersion,
		"--composer-version", composerVersion,
		"--http-port", httpPort,
		"--https-port", httpsPort,
		fmt.Sprintf("--xdebug-enabled=%t", xdebugEnabled),
		fmt.Sprintf("--no-project-mount=%t", noProjectMount),
		"--additional-hostnames", additionalHostnames,
		"--additional-fqdns", additionalFQDNs,
		"--upload-dir", uploadDir,
		"--webserver-type", webserverType,
		"--web-image", webImage,
		"--db-image", dbImage,
		"--dba-image", dbaImage,
		"--web-working-dir", webWorkingDir,
		"--db-working-dir", dbWorkingDir,
		"--dba-working-dir", dbaWorkingDir,
		"--omit-containers", omitContainers,
		"--host-db-port", hostDBPort,
		"--host-webserver-port", hostWebserverPort,
		"--host-https-port", hostHTTPSPort,
		"--webimage-extra-packages", webimageExtraPackages,
		"--dbimage-extra-packages", dbimageExtraPackages,
		"--phpmyadmin-port", phpMyAdminPort,
		"--mailhog-port", mailhogPort,
		"--project-tld", projectTLD,
		"--web-environment", webEnv,
		fmt.Sprintf("--use-dns-when-possible=%t", useDNSWhenPossible),
		"--timezone", timezone,
	}

	out, err := exec.RunCommand(DdevBin, args)
	assert.NoError(err, "error running ddev %v: %v, output=%s", args, err, out)

	configFile := filepath.Join(tmpdir, ".ddev", "config.yaml")
	configContents, err := os.ReadFile(configFile)
	if err != nil {
		t.Errorf("Unable to read %s: %v", configFile, err)
	}

	app := &ddevapp.DdevApp{}
	if err = yaml.Unmarshal(configContents, app); err != nil {
		t.Errorf("Could not unmarshal %s: %v", configFile, err)
	}

	assert.Equal(projectName, app.Name)
	assert.Equal(docroot, app.Docroot)
	assert.Equal(projectType, app.Type)
	assert.Equal(phpVersion, app.PHPVersion)
	assert.Equal(composerVersion, app.ComposerVersion)
	assert.Equal(httpPort, app.RouterHTTPPort)
	assert.Equal(httpsPort, app.RouterHTTPSPort)
	assert.Equal(hostWebserverPort, app.HostWebserverPort)
	assert.Equal(hostDBPort, app.HostDBPort)
	assert.Equal(xdebugEnabled, app.XdebugEnabled)
	assert.Equal(noProjectMount, app.NoProjectMount)
	assert.Equal(additionalHostnamesSlice, app.AdditionalHostnames)
	assert.Equal(additionalFQDNsSlice, app.AdditionalFQDNs)
	assert.Equal(uploadDir, app.UploadDir)
	assert.Equal(webserverType, app.WebserverType)
	assert.Equal(webImage, app.WebImage)
	assert.Equal(dbImage, app.DBImage)
	assert.Equal(dbaImage, app.DBAImage)
	assert.Equal(webWorkingDir, app.WorkingDir["web"])
	assert.Equal(dbWorkingDir, app.WorkingDir["db"])
	assert.Equal(dbaWorkingDir, app.WorkingDir["dba"])
	assert.Equal(webimageExtraPackagesSlice, app.WebImageExtraPackages)
	assert.Equal(dbimageExtraPackagesSlice, app.DBImageExtraPackages)
	assert.Equal(phpMyAdminPort, app.PHPMyAdminPort)
	assert.Equal(mailhogPort, app.MailhogPort)
	assert.Equal(useDNSWhenPossible, app.UseDNSWhenPossible)
	assert.Equal(projectTLD, app.ProjectTLD)
	assert.Equal(timezone, app.Timezone)
	require.NotEmpty(t, app.WebEnvironment)
	assert.Equal(webEnv, app.WebEnvironment[0])

	// Test that container images and working dirs can be unset with default flags
	args = []string{
		"config",
		"--web-image-default",
		"--db-image-default",
		"--dba-image-default",
		"--web-working-dir-default",
		"--db-working-dir-default",
		"--dba-working-dir-default",
	}

	_, err = exec.RunCommand(DdevBin, args)
	assert.NoError(err)

	configContents, err = os.ReadFile(configFile)
	assert.NoError(err, "Unable to read %s: %v", configFile, err)

	app = &ddevapp.DdevApp{}
	err = yaml.Unmarshal(configContents, app)
	assert.NoError(err, "Could not unmarshal %s: %v", configFile, err)

	assert.Equal(app.WebImage, "")
	assert.Equal(app.DBImage, "")
	assert.Equal(app.DBAImage, "")
	assert.Equal(len(app.WorkingDir), 0)

	// Test that all container images and working dirs can each be unset with single default images flag
	args = []string{
		"config",
		"--web-image", webImage,
		"--db-image", dbImage,
		"--dba-image", dbaImage,
		"--web-working-dir", webWorkingDir,
		"--db-working-dir", dbWorkingDir,
		"--dba-working-dir", dbaWorkingDir,
	}

	_, err = exec.RunCommand(DdevBin, args)
	assert.NoError(err)

	args = []string{
		"config",
		"--image-defaults",
		"--working-dir-defaults",
	}

	_, err = exec.RunCommand(DdevBin, args)
	assert.NoError(err)

	configContents, err = os.ReadFile(configFile)
	assert.NoError(err, "Unable to read %s: %v", configFile, err)

	app = &ddevapp.DdevApp{}
	err = yaml.Unmarshal(configContents, app)
	assert.NoError(err, "Could not unmarshal %s: %v", configFile, err)

	assert.Equal(app.WebImage, "")
	assert.Equal(app.DBImage, "")
	assert.Equal(app.DBAImage, "")
	assert.Equal(len(app.WorkingDir), 0)
}

// TestConfigInvalidProjectname tests to make sure that invalid projectnames
// are not accepted and valid names are accepted.
func TestConfigInvalidProjectname(t *testing.T) {
	var err error
	assert := asrt.New(t)

	// Create a temporary directory and switch to it.
	tmpdir := testcommon.CreateTmpDir(t.Name())
	defer testcommon.CleanupDir(tmpdir)
	defer testcommon.Chdir(tmpdir)()

	// Create an existing docroot
	docroot := "web"
	if err = os.MkdirAll(filepath.Join(tmpdir, docroot), 0755); err != nil {
		t.Errorf("Could not create docroot %s in %s", docroot, tmpdir)
	}

	// Test some valid project names
	for _, projName := range []string{"no-spaces-but-hyphens", "UpperAndLower", "should.work.with.dots"} {
		args := []string{
			"config",
			"--project-name", projName,
		}

		out, err := exec.RunCommand(DdevBin, args)
		assert.NoError(err)
		assert.NotContains(out, "is not a valid project name")
		assert.Contains(out, "You may now run 'ddev start'")
		args = []string{
			"stop",
			"--unlist", projName,
		}
		_, _ = exec.RunCommand(DdevBin, args)

		_ = os.Remove(filepath.Join(tmpdir, ".ddev", "config.yaml"))
	}

	// test some invalid project names.
	for _, projName := range []string{"with spaces", "with_underscores", "no,commas-will-make-it"} {
		args := []string{
			"config",
			"--project-name", projName,
		}

		out, err := exec.RunCommand(DdevBin, args)
		assert.Error(err)
		assert.Contains(out, fmt.Sprintf("%s is not a valid project name", projName))
		assert.NotContains(out, "You may now run 'ddev start'")
		_ = os.Remove(filepath.Join(tmpdir, ".ddev", "config.yaml"))
	}

}

// TestCmdDisasterConfig tests to make sure we can't accidentally
// config in homedir, and that config in a subdir is handled correctly
func TestCmdDisasterConfig(t *testing.T) {
	var err error
	assert := asrt.New(t)

	testDir, _ := os.Getwd()
	// Make sure we're not allowed to config in home directory.
	home, _ := homedir.Dir()
	err = os.Chdir(home)
	assert.NoError(err)
	out, err := exec.RunCommand(DdevBin, []string{"config", "--project-type=php"})
	assert.Error(err)
	assert.Contains(out, "not useful in")
	_ = out

	err = os.Chdir(testDir)
	assert.NoError(err)

	// Create a temporary directory and switch to it.
	tmpdir := testcommon.CreateTmpDir(t.Name())
	defer testcommon.CleanupDir(tmpdir)
	defer testcommon.Chdir(tmpdir)()

	// Create a project
	_, err = exec.RunCommand(DdevBin, []string{"config", "--project-type=php"})
	assert.NoError(err)
	subdirName := t.Name() + fileutil.RandomFilenameBase()
	subdir := filepath.Join(tmpdir, subdirName)
	err = os.Mkdir(subdir, 0777)
	assert.NoError(err)
	err = os.Chdir(subdir)
	assert.NoError(err)

	// Make sure that ddev config in a subdir gives a warning
	out, err = exec.RunCommand(DdevBin, []string{"config", "--project-type=php"})
	assert.NoError(err)
	assert.Contains(out, "possible you wanted to")
	assert.Contains(out, fmt.Sprintf("parent directory %s?", tmpdir))
	assert.FileExists(filepath.Join(subdir, ".ddev/config.yaml"))
}

/**
  	Overall expected behavior of `ddev config` and config.yaml is this:
	* If no config.yaml exists when `ddev config` is run, a new one will be created with default
		values except whatever is specified in the ddev config args.
	* If an item exists in config.yaml, it will be rewritten there after a `ddev config` action
	* If the `ddev config` action overrides explicitly the item in config.yaml, the config.yaml
		will be rewritten with the updated value and no other changes.

	In the case of mariadb_version and dbimage there's quite a lot of overlap sadly, but
	the bottom line is that dbimage must win any battles, and if specified either via
	`ddev config --db-image` or in the config.yaml, it wins when the dbimage is chosen for the
	ddevapp. So:
	* `ddev config --mariadb-version` must
		* Never add a dbimage line to the config.yaml
		* Add or update the mariadb_version line in config.yaml
	* `ddev config --db-image` must
		* Not change the mariadb_version (although people may be broken if they do it wrong)
		* Add a line specifying the dbimage
	* ddevapp.NewApp() must:
		* Set the dbimage based on any dbimage found in the config.yaml if there is one
		* Otherwise (default case) set it based on mariadb_version and the default dbimage.
*/
// TestConfigMariaDBVersion checks to make sure that both
// ddev config --mariadb-version and ddev config --dbimage behave correctly,
// either separately or together.
func TestConfigMariaDBVersion(t *testing.T) {

	assert := asrt.New(t)

	pwd, _ := os.Getwd()
	versionsToTest := nodeps.ValidMariaDBVersions

	// Create a temporary directory and switch to it.
	tmpDir := testcommon.CreateTmpDir(t.Name())
	defer testcommon.CleanupDir(tmpDir)
	defer testcommon.Chdir(tmpDir)()

	args := []string{
		"config",
		"--mariadb-version",
	}

	// Use a config file that does not specify mariadb version
	// it should end up with default mariadb version dbimage
	_ = os.RemoveAll(filepath.Join(tmpDir, ".ddev"))
	_ = os.MkdirAll(filepath.Join(tmpDir, ".ddev"), 0777)
	err := fileutil.CopyFile(filepath.Join(pwd, "testdata", t.Name(), "config.yaml.empty"), filepath.Join(tmpDir, ".ddev", "config.yaml"))
	assert.NoError(err)
	app, err := ddevapp.NewApp(tmpDir, false)
	//nolint: errcheck
	defer app.Stop(true, false)
	assert.NoError(err)
	_, err = app.ReadConfig(false)
	assert.NoError(err)
	assert.Equal(nodeps.MariaDBDefaultVersion, app.MariaDBVersion)
	err = app.Start()
	assert.NoError(err)
	assert.EqualValues(version.GetDBImage(nodeps.MariaDB, nodeps.MariaDBDefaultVersion), app.DBImage)
	_ = app.Stop(true, false)

	// Verify behavior with no existing config.yaml. It should
	// just add mariadb_version into the config and nothing else
	// The app.DBImage should end up being the constructed one based
	// on the default and the mariadb_version
	for cmdMariaDBVersion := range versionsToTest {
		_ = os.RemoveAll(filepath.Join(tmpDir, ".ddev"))
		configArgs := append(args, cmdMariaDBVersion, "--project-name", "noconfigyet-"+cmdMariaDBVersion)
		ddevCmd := "ddev " + strings.Join(configArgs, " ")
		out, err := exec.RunCommand(DdevBin, configArgs)
		assert.NoError(err)
		assert.Contains(out, "You may now run 'ddev start'")

		// First test the bare explicit values found in the config.yaml,
		// without the NewApp adjustments
		app := &ddevapp.DdevApp{}
		assert.NoError(err)
		err = app.LoadConfigYamlFile(filepath.Join(tmpDir, ".ddev", "config.yaml"))
		assert.NoError(err)
		assert.Equal(cmdMariaDBVersion, app.MariaDBVersion)
		assert.Empty(app.DBImage, "generated config.yaml dbimage should have been empty for command '%s'", ddevCmd)

		// Now use NewApp() to load, so that we get the full logic of that function.
		app, err = ddevapp.NewApp(tmpDir, false)
		//nolint: errcheck
		defer app.Stop(true, false)
		assert.NoError(err)
		_, err = app.ReadConfig(false)
		assert.NoError(err)
		assert.Equal(cmdMariaDBVersion, app.MariaDBVersion)
		_ = app.Stop(true, false)
	}

	// If we start with a config.yaml specifying basically anything for mariadb_version
	// using ddev config --mariadb-version should overwrite it with the specified version.
	for cmdMariaDBVersion := range versionsToTest {
		for configMariaDBVersion := range versionsToTest {
			_ = os.Remove(filepath.Join(tmpDir, ".ddev", "config.yaml"))

			err := fileutil.CopyFile(filepath.Join(pwd, "testdata/TestConfigMariaDBVersion", "config.yaml."+configMariaDBVersion), filepath.Join(tmpDir, ".ddev", "config.yaml"))
			assert.NoError(err)
			configArgs := append(args, cmdMariaDBVersion, "--project-name", "hasconfig-"+cmdMariaDBVersion)
			out, err := exec.RunCommand(DdevBin, configArgs)
			assert.NoError(err)
			assert.Contains(out, "You may now run 'ddev start'")

			// First test the bare explicit values found in the config.yaml,
			// without the NewApp adjustments
			app := &ddevapp.DdevApp{}
			assert.NoError(err)
			err = app.LoadConfigYamlFile(filepath.Join(tmpDir, ".ddev", "config.yaml"))
			assert.NoError(err)
			assert.Equal(cmdMariaDBVersion, app.MariaDBVersion)
			assert.Empty(app.DBImage)

			// Now test with the logical additions made by NewApp()
			app, err = ddevapp.NewApp(tmpDir, false)
			//nolint: errcheck
			defer app.Stop(true, false)
			assert.NoError(err)
			_, err = app.ReadConfig(false)
			assert.NoError(err)
			assert.Equal(cmdMariaDBVersion, app.MariaDBVersion)
			_ = app.Stop(true, false)
		}
	}

	// If we start with a config.yaml specifying a dbimage, the mariadb_version
	// should be set but the dbimage should then override all other values
	for cmdMariaDBVersion := range versionsToTest {
		for configMariaDBVersion := range versionsToTest {
			_ = os.Remove(filepath.Join(tmpDir, ".ddev", "config.yaml"))

			testConfigFile := filepath.Join(pwd, "testdata/TestConfigMariaDBVersion", "config.yaml.imagespec."+configMariaDBVersion)
			err := fileutil.CopyFile(testConfigFile, filepath.Join(tmpDir, ".ddev", "config.yaml"))
			assert.NoError(err)
			tmpProjectName := "imagespec-" + cmdMariaDBVersion + ".cmdversion." + cmdMariaDBVersion
			configArgs := append(args, cmdMariaDBVersion, "--project-name", tmpProjectName)
			out, err := exec.RunCommand(DdevBin, configArgs)
			assert.NoError(err)
			assert.Contains(out, "You may now run 'ddev start'")

			// First test the bare explicit values found in the config.yaml,
			// without the NewApp adjustments
			app := &ddevapp.DdevApp{}
			assert.NoError(err)
			err = app.LoadConfigYamlFile(filepath.Join(tmpDir, ".ddev", "config.yaml"))
			assert.NoError(err)
			assert.Equal(cmdMariaDBVersion, app.MariaDBVersion)
			// The ddev config --mariadb-version should *not* have changed the dbimg
			// which was in the config.yaml
			assert.Equal("somerandomdbimg-"+nodeps.MariaDB+"-"+configMariaDBVersion, app.DBImage, "ddev %s did not result in respect for existing configured dbimg in file=%s", strings.Join(configArgs, " "))

			// Now test with NewApp's additions, which should leave app.DBImage alone.
			app, err = ddevapp.NewApp(tmpDir, false)
			//nolint: errcheck
			defer app.Stop(true, false)
			assert.NoError(err)
			_, err = app.ReadConfig(false)
			assert.NoError(err)
			assert.Equal(cmdMariaDBVersion, app.MariaDBVersion)
			assert.Equal("somerandomdbimg-"+nodeps.MariaDB+"-"+configMariaDBVersion, app.DBImage)
			_ = app.Stop(true, false)
		}
	}

	// If we specify both the --mariadb-version and the --dbimage flags,
	// both should be set, but the actual NewApp() will result in
	// both items, and an app.Start() would use the dbimage specified.
	for cmdMariaDBVersion := range versionsToTest {
		for cmdDBImageVersion := range versionsToTest {
			_ = os.Remove(filepath.Join(tmpDir, ".ddev", "config.yaml"))

			configArgs := append(args, cmdMariaDBVersion)
			configArgs = append(configArgs, []string{"--db-image", version.GetDBImage(nodeps.MariaDB, cmdDBImageVersion)}...)
			configArgs = append(configArgs, "--project-name", "both-args-"+cmdMariaDBVersion)

			out, err := exec.RunCommand(DdevBin, configArgs)
			assert.NoError(err)
			assert.Contains(out, "You may now run 'ddev start'")

			// First test the bare explicit values found in the config.yaml,
			// without the NewApp adjustments
			app := &ddevapp.DdevApp{}
			//assert.NoError(err)
			err = app.LoadConfigYamlFile(filepath.Join(tmpDir, ".ddev", "config.yaml"))
			assert.NoError(err)
			assert.Equal(cmdMariaDBVersion, app.MariaDBVersion)
			// Loaded app dbimage version should be the explicit value found in
			// config.yaml. Or it should be empty if the cmdDbImage is the default
			if app.DBImage == "" && cmdDBImageVersion == cmdMariaDBVersion {
				// Should have blank dbimage
				// if the specified dbimage is the default for this mariadb version
			} else {
				assert.Equal(version.GetDBImage(nodeps.MariaDB, cmdDBImageVersion), app.DBImage, "Incorrect dbimage '%s' for cmdMariaDBVersion '%s' and cmdDBImageVersion=%s", app.DBImage, cmdMariaDBVersion, cmdDBImageVersion)
			}

			// Now test with NewApp()'s adjustments
			app, err = ddevapp.NewApp(tmpDir, false)
			//nolint: errcheck
			defer app.Stop(true, false)
			assert.NoError(err)
			_, err = app.ReadConfig(false)
			assert.NoError(err)
			assert.Equal(cmdMariaDBVersion, app.MariaDBVersion)

			// app.DBImage should be "" if it's the default for that maria version
			if cmdDBImageVersion == cmdMariaDBVersion {
				assert.Equal("", app.DBImage)
			} else {
				assert.EqualValues(app.DBImage, version.GetDBImage(nodeps.MariaDB, cmdDBImageVersion))
			}
			_ = app.Stop(true, false)
		}
	}
}

// TestConfigMySQLVersion tests various permutations of mysql version
// and mariadb version, etc.
func TestConfigMySQLVersion(t *testing.T) {
	assert := asrt.New(t)
	versionsToTest := nodeps.ValidMySQLVersions

	pwd, _ := os.Getwd()

	// Create a temporary directory and switch to it.
	testDir := testcommon.CreateTmpDir(t.Name())
	_ = os.Chdir(testDir)

	t.Cleanup(func() {
		err := os.Chdir(pwd)
		assert.NoError(err)
		err = os.RemoveAll(testDir)
		assert.NoError(err)
	})

	err := os.MkdirAll(filepath.Join(testDir, ".ddev"), 0777)
	require.NoError(t, err)

	args := []string{
		"config",
		"--mysql-version=5.6",
		"--mariadb-version=10.1",
	}

	// Try conflicting configurations
	_ = os.RemoveAll(filepath.Join(testDir, ".ddev"))
	configArgs := append(args, "--project-name=conflicting-db-versions")
	out, err := exec.RunCommand(DdevBin, configArgs)
	assert.Error(err)
	assert.Contains(out, "mysql-version cannot be set if mariadb-version is already set. mariadb-version is set to 10.1")

	for cmdMySQLVersion := range versionsToTest {
		for cmdDBImageVersion := range versionsToTest {
			args := []string{
				"config",
			}
			_ = os.RemoveAll(filepath.Join(testDir, ".ddev"))
			projectName := "mysqlversion-" + cmdMySQLVersion + "-dbimageversion-" + cmdDBImageVersion
			configArgs := append(args, []string{
				"--project-name=" + projectName,
				"--db-image=" + version.GetDBImage(nodeps.MySQL, cmdDBImageVersion),
				"--mysql-version=" + cmdMySQLVersion,
				`--mariadb-version=`,
			}...)
			ddevCmd := strings.Join(configArgs, " ")
			out, err = exec.RunCommand(DdevBin, configArgs)
			assert.NoError(err, "failed to run ddevcmd=%s, out=%s", ddevCmd, out)
			assert.Contains(out, "You may now run 'ddev start'")

			app, err := ddevapp.NewApp(testDir, false)
			assert.NoError(err)
			// If the two versions are equal, we expect the app.DBImage to be empty
			// because it's identical to the image we'd get with just app.MySQLVersion
			if cmdMySQLVersion == cmdDBImageVersion {
				assert.EqualValues("", app.DBImage)
			} else {
				assert.EqualValues(version.GetDBImage(nodeps.MySQL, cmdDBImageVersion), app.DBImage)
			}
			_, _ = exec.RunCommand(DdevBin, []string{"remove", "-RO"})
		}
	}

}

// TestMariaMysqlConflicts tests the various ways that mysql_version
// and mariadb_version can interact.
func TestMariaMysqlConflicts(t *testing.T) {
	assert := asrt.New(t)
	pwd, _ := os.Getwd()

	// Create a temporary directory and switch to it.
	testDir := testcommon.CreateTmpDir(t.Name())
	_ = os.Chdir(testDir)
	t.Cleanup(func() {
		err := os.Chdir(pwd)
		assert.NoError(err)
		err = os.RemoveAll(testDir)
		assert.NoError(err)
	})

	_ = os.MkdirAll(filepath.Join(testDir, ".ddev"), 0777)

	// Use a config file that does not specify mariadb version
	// but does specify mysql_version
	err := fileutil.CopyFile(filepath.Join(pwd, "testdata", t.Name(), "config.yaml.mysql8only"), filepath.Join(testDir, ".ddev", "config.yaml"))
	assert.NoError(err)
	app, err := ddevapp.NewApp(testDir, false)
	assert.NoError(err)
	assert.Equal(nodeps.MySQL80, app.MySQLVersion)
	assert.Empty(app.MariaDBVersion)

	// Use a config file that specifies both but with empty mariadb_version
	err = fileutil.CopyFile(filepath.Join(pwd, "testdata", t.Name(), "config.yaml.mysqlwithemptymaria"), filepath.Join(testDir, ".ddev", "config.yaml"))
	assert.NoError(err)
	app, err = ddevapp.NewApp(testDir, false)
	assert.NoError(err)
	assert.Equal(nodeps.MySQL80, app.MySQLVersion)
	assert.Empty(app.MariaDBVersion)

	// Use a config file that specifies both but with empty mysql_version
	err = fileutil.CopyFile(filepath.Join(pwd, "testdata", t.Name(), "config.yaml.mariawithemptymysql"), filepath.Join(testDir, ".ddev", "config.yaml"))
	assert.NoError(err)
	app, err = ddevapp.NewApp(testDir, false)
	assert.NoError(err)
	assert.Equal(nodeps.MariaDBDefaultVersion, app.MariaDBVersion)
	assert.Empty(app.MySQLVersion)

	// Use a config file that specifies neither.
	err = fileutil.CopyFile(filepath.Join(pwd, "testdata", t.Name(), "config.yaml.nodbspecified"), filepath.Join(testDir, ".ddev", "config.yaml"))
	assert.NoError(err)
	app, err = ddevapp.NewApp(testDir, false)
	assert.NoError(err)
	assert.Equal(nodeps.MariaDBDefaultVersion, app.MariaDBVersion)
	assert.Empty(app.MySQLVersion)
}

//TestConfigGitignore checks that our gitignore is ignoring the right things.
func TestConfigGitignore(t *testing.T) {
	assert := asrt.New(t)

	// Create a temporary directory and switch to it.
	tmpDir := testcommon.CreateTmpDir(t.Name())
	defer testcommon.CleanupDir(tmpDir)
	defer testcommon.Chdir(tmpDir)()

	_, err := exec.RunCommand(DdevBin, []string{"config", "--project-type=php"})
	assert.NoError(err)
	defer func() {
		_, err = exec.RunCommand(DdevBin, []string{"delete", "-Oy"})
		assert.NoError(err)
		_, err = exec.RunCommand("bash", []string{"-c", fmt.Sprintf("rm -f ~/.ddev/commands/web/%s ~/.ddev/homeadditions/%s", t.Name(), t.Name())})
		assert.NoError(err)
	}()

	_, err = exec.RunCommand("git", []string{"init"})
	assert.NoError(err)
	_, err = exec.RunCommand("git", []string{"add", "."})
	assert.NoError(err)
	out, err := exec.RunCommand("git", []string{"status"})
	assert.NoError(err)

	// git status should have one new file, config.yaml
	assert.Contains(out, "new file:   .ddev/config.yaml")
	// .ddev/config.yaml should be the only new file, remove it and check
	out = strings.ReplaceAll(out, "new file:   .ddev/config.yaml", "")
	assert.NotContains(out, "new file:")

	_, err = exec.RunCommand("bash", []string{"-c", fmt.Sprintf("touch ~/.ddev/commands/web/%s ~/.ddev/homeadditions/%s", t.Name(), t.Name())})
	assert.NoError(err)

	_, err = exec.RunCommand(DdevBin, []string{"start", "-y"})
	assert.NoError(err)
	statusOut, err := exec.RunCommand("bash", []string{"-c", "git status"})
	assert.NoError(err)
	_, err = exec.RunCommand("bash", []string{"-c", "git status | grep 'Untracked files'"})
	assert.Error(err, "Untracked files were found where we didn't expect them: %s", statusOut)
}
