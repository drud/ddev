package ddevapp_test

import (
	"bufio"
	"fmt"
	"github.com/drud/ddev/pkg/exec"
	"github.com/drud/ddev/pkg/nodeps"
	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	. "github.com/drud/ddev/pkg/ddevapp"
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/testcommon"
	"github.com/drud/ddev/pkg/util"
	"github.com/drud/ddev/pkg/version"
	"github.com/google/uuid"
	asrt "github.com/stretchr/testify/assert"
)

// TestNewConfig tests functionality around creating a new config, writing it to disk, and reading the resulting config.
func TestNewConfig(t *testing.T) {
	assert := asrt.New(t)
	// Create a temporary directory and change to it for the duration of this test.
	testDir := testcommon.CreateTmpDir(t.Name())

	pwd, _ := os.Getwd()

	// Load a new Config
	app, err := NewApp(testDir, true)
	assert.NoError(err)

	t.Cleanup(func() {
		err = app.Stop(true, false)
		assert.NoError(err)
		err = os.RemoveAll(testDir)
		assert.NoError(err)
		err = os.Chdir(pwd)
		assert.NoError(err)
	})

	// Ensure the config uses specified defaults.
	assert.Equal(app.GetDBImage(), version.GetDBImage(nodeps.MariaDB))
	assert.Equal(app.WebImage, version.GetWebImage())
	assert.Equal(app.DBAImage, version.GetDBAImage())
	app.Name = util.RandString(32)
	app.Type = nodeps.AppTypeDrupal8

	// WriteConfig the app.
	err = app.WriteConfig()
	assert.NoError(err)
	_, err = os.Stat(app.ConfigPath)
	assert.NoError(err)

	loadedConfig, err := NewApp(testDir, true)
	assert.NoError(err)
	assert.Equal(app.Name, loadedConfig.Name)
	assert.Equal(app.Type, loadedConfig.Type)
}

// TestDisasterConfig tests for disaster opportunities (configing wrong directory, home dir, etc).
func TestDisasterConfig(t *testing.T) {
	assert := asrt.New(t)

	pwd, _ := os.Getwd()

	// Make sure we're not allowed to config in home directory.
	tmpDir, _ := homedir.Dir()
	_, err := NewApp(tmpDir, false)
	assert.Error(err)
	assert.Contains(err.Error(), "ddev config is not useful")
	_ = os.Chdir(pwd)

	// Create a temporary directory and change to it for the duration of this test.
	tmpDir = testcommon.CreateTmpDir(t.Name())

	// Load a new Config
	app, err := NewApp(tmpDir, false)
	assert.NoError(err)

	t.Cleanup(func() {
		err = app.Stop(true, false)
		assert.NoError(err)
		err = os.Chdir(pwd)
		assert.NoError(err)
		_ = os.RemoveAll(tmpDir)
	})

	// WriteConfig the app.
	err = app.WriteConfig()
	assert.NoError(err)
	_, err = os.Stat(app.ConfigPath)
	assert.NoError(err)

	subdir := filepath.Join(app.AppRoot, "subdir")
	err = os.Mkdir(subdir, 0777)
	assert.NoError(err)
	err = os.Chdir(subdir)
	assert.NoError(err)
	subdirApp, err := NewApp(subdir, false)
	assert.NoError(err)
	_ = subdirApp

}

// TestAllowedAppType tests the IsAllowedAppType function.
func TestAllowedAppTypes(t *testing.T) {
	assert := asrt.New(t)
	for _, v := range GetValidAppTypes() {
		assert.True(IsValidAppType(v))
	}

	for i := 1; i <= 50; i++ {
		randomType := util.RandString(32)
		assert.False(IsValidAppType(randomType))
	}
}

// TestPrepDirectory ensures the configuration directory can be created with the correct permissions.
func TestPrepDirectory(t *testing.T) {
	assert := asrt.New(t)
	// Create a temporary directory and change to it for the duration of this test.
	testDir := testcommon.CreateTmpDir("TestPrepDirectory")
	defer testcommon.CleanupDir(testDir)
	defer testcommon.Chdir(testDir)()

	app, err := NewApp(testDir, true)
	assert.NoError(err)

	// Prep the directory.
	err = PrepDdevDirectory(filepath.Dir(app.ConfigPath))
	assert.NoError(err)

	// Read directory info an ensure it exists.
	_, err = os.Stat(filepath.Dir(app.ConfigPath))
	assert.NoError(err)
}

// TestHostName tests that the TestSite.Hostname() field returns the hostname as expected.
func TestHostName(t *testing.T) {
	assert := asrt.New(t)
	testDir := testcommon.CreateTmpDir("TestHostName")
	defer testcommon.CleanupDir(testDir)
	defer testcommon.Chdir(testDir)()
	app, err := NewApp(testDir, true)
	assert.NoError(err)
	app.Name = util.RandString(32)

	assert.Equal(app.GetHostname(), strings.ToLower(app.Name+"."+app.ProjectTLD))
}

// TestWriteDockerComposeYaml tests the writing of a .ddev/docker-compose-* file.
func TestWriteDockerComposeYaml(t *testing.T) {
	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)
	pwd, _ := os.Getwd()
	testDir := testcommon.CreateTmpDir(t.Name())
	defer testcommon.CleanupDir(testDir)
	defer testcommon.Chdir(testDir)()

	app, err := NewApp(testDir, true)
	assert.NoError(err)

	t.Cleanup(func() {
		err = app.Stop(true, false)
		assert.NoError(err)
		err = os.Chdir(pwd)
		assert.NoError(err)
		_ = os.RemoveAll(testDir)
	})

	app.Name = util.RandString(32)
	app.Type = GetValidAppTypes()[0]

	// WriteConfig a config to create/prep necessary directories.
	err = app.WriteConfig()
	assert.NoError(err)

	// After the config has been written and directories exist, the write should work.
	err = app.WriteDockerComposeYAML()
	assert.NoError(err)

	// Ensure we can read from the file and that it's a regular file with the expected name.
	fileinfo, err := os.Stat(app.DockerComposeYAMLPath())
	assert.NoError(err)
	assert.False(fileinfo.IsDir())
	assert.Equal(fileinfo.Name(), filepath.Base(app.DockerComposeYAMLPath()))

	composeBytes, err := os.ReadFile(app.DockerComposeYAMLPath())
	assert.NoError(err)
	contentString := string(composeBytes)
	assert.Contains(contentString, app.Type)
}

// TestConfigCommand tests the interactive config options.
func TestConfigCommand(t *testing.T) {
	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)
	pwd, _ := os.Getwd()

	const apptypePos = 0
	const phpVersionPos = 1
	testMatrix := map[string][]string{
		"drupal6phpversion": {nodeps.AppTypeDrupal6, nodeps.PHP56},
		"drupal7phpversion": {nodeps.AppTypeDrupal7, nodeps.PHPDefault},
		"drupal8phpversion": {nodeps.AppTypeDrupal8, nodeps.PHPDefault},
	}

	for testName, testValues := range testMatrix {
		testDir := testcommon.CreateTmpDir(t.Name() + "_" + testName)

		// Create a docroot folder.
		err := os.Mkdir(filepath.Join(testDir, "docroot"), 0644)
		if err != nil {
			t.Errorf("Could not create docroot directory under %s", testDir)
		}

		// Create the ddevapp we'll use for testing.
		// This will not return an error, since there is no existing configuration.
		app, err := NewApp(testDir, true)
		assert.NoError(err)

		t.Cleanup(func() {
			err = app.Stop(true, false)
			assert.NoError(err)
			err = os.Chdir(pwd)
			assert.NoError(err)
			err = os.RemoveAll(testDir)
			assert.NoError(err)
		})
		// Randomize some values to use for Stdin during testing.
		name := strings.ToLower(util.RandString(16))
		invalidAppType := strings.ToLower(util.RandString(8))

		// Create an example input buffer that writes the sitename, a valid document root,
		// an invalid app type, and finally a valid app type (from test matrix)
		input := fmt.Sprintf("%s\ndocroot\n%s\n%s", name, invalidAppType, testValues[apptypePos])
		scanner := bufio.NewScanner(strings.NewReader(input))
		util.SetInputScanner(scanner)

		restoreOutput := util.CaptureUserOut()
		err = app.PromptForConfig()
		assert.NoError(err, t)
		out := restoreOutput()

		// Ensure we have expected vales in output.
		assert.Contains(out, testDir)
		assert.Contains(out, fmt.Sprintf("'%s' is not a valid project type", invalidAppType))

		// Create an example input buffer that writes an invalid projectname, then a valid-project-name,
		// a valid document root, a valid app type
		input = fmt.Sprintf("invalid_project_name\n%s\ndocroot\n%s", name, testValues[apptypePos])
		scanner = bufio.NewScanner(strings.NewReader(input))
		util.SetInputScanner(scanner)

		restoreOutput = util.CaptureUserOut()
		err = app.PromptForConfig()
		assert.NoError(err)
		out = restoreOutput()

		// Ensure we have expected vales in output.
		assert.Contains(out, testDir)
		assert.Contains(out, "invalid_project_name is not a valid project name")

		// Ensure values were properly set on the app struct.
		assert.Equal(name, app.Name)
		assert.Equal(testValues[apptypePos], app.Type)
		assert.Equal("docroot", app.Docroot)
		assert.EqualValues(testValues[phpVersionPos], app.PHPVersion, "PHP value incorrect for app %v", app)
		err = PrepDdevDirectory(testDir)
		assert.NoError(err)
	}
}

// TestConfigCommandInteractiveCreateDocrootDenied
func TestConfigCommandInteractiveCreateDocrootDenied(t *testing.T) {
	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)

	noninteractiveEnv := "DDEV_NONINTERACTIVE"
	// nolint: errcheck
	defer os.Setenv(noninteractiveEnv, os.Getenv(noninteractiveEnv))
	err := os.Unsetenv(noninteractiveEnv)
	assert.NoError(err)

	testMatrix := map[string][]string{
		"drupal6phpversion": {nodeps.AppTypeDrupal6, nodeps.PHP56},
		"drupal7phpversion": {nodeps.AppTypeDrupal7, nodeps.PHPDefault},
		"drupal8phpversion": {nodeps.AppTypeDrupal8, nodeps.PHPDefault},
	}

	for testName := range testMatrix {
		testDir := testcommon.CreateTmpDir(t.Name() + testName)

		// testcommon.Chdir()() and CleanupDir() checks their own errors (and exit)
		defer testcommon.CleanupDir(testDir)
		defer testcommon.Chdir(testDir)()

		// Create the ddevapp we'll use for testing.
		// This will not return an error, since there is no existing configuration.
		app, err := NewApp(testDir, true)
		assert.NoError(err)

		// Randomize some values to use for Stdin during testing.
		name := uuid.New().String()
		nonexistentDocroot := filepath.Join("does", "not", "exist")

		// Create an example input buffer that writes the sitename, a nonexistent document root,
		// and a "no"
		input := fmt.Sprintf("%s\n%s\nno", name, nonexistentDocroot)
		scanner := bufio.NewScanner(strings.NewReader(input))
		util.SetInputScanner(scanner)

		err = app.PromptForConfig()
		assert.Error(err, t)

		// Ensure we have expected vales in output.
		assert.Contains(err.Error(), "docroot must exist to continue configuration")

		err = PrepDdevDirectory(testDir)
		assert.NoError(err)
	}
}

// TestConfigCommandCreateDocrootAllowed
func TestConfigCommandCreateDocrootAllowed(t *testing.T) {
	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)

	const apptypePos = 0
	const phpVersionPos = 1
	testMatrix := map[string][]string{
		"drupal6phpversion": {nodeps.AppTypeDrupal6, nodeps.PHP56},
		"drupal7phpversion": {nodeps.AppTypeDrupal7, nodeps.PHPDefault},
		"drupal8phpversion": {nodeps.AppTypeDrupal8, nodeps.PHPDefault},
	}

	for testName, testValues := range testMatrix {
		testDir := testcommon.CreateTmpDir(t.Name() + testName)

		// testcommon.Chdir()() and CleanupDir() checks their own errors (and exit)
		defer testcommon.CleanupDir(testDir)
		defer testcommon.Chdir(testDir)()

		// Create the ddevapp we'll use for testing.
		// This will not return an error, since there is no existing configuration.
		app, err := NewApp(testDir, true)
		assert.NoError(err)

		// Randomize some values to use for Stdin during testing.
		name := uuid.New().String()
		nonexistentDocroot := filepath.Join("does", "not", "exist")

		// Create an example input buffer that writes the sitename, a nonexistent document root,
		// a "yes", and a valid apptype
		input := fmt.Sprintf("%s\n%s\nyes\n%s", name, nonexistentDocroot, testValues[apptypePos])
		scanner := bufio.NewScanner(strings.NewReader(input))
		util.SetInputScanner(scanner)

		restoreOutput := util.CaptureUserOut()
		err = app.PromptForConfig()
		assert.NoError(err, t)
		out := restoreOutput()

		// Ensure we have expected vales in output.
		assert.Contains(out, nonexistentDocroot)
		assert.Contains(out, "Created docroot")

		// Ensure values were properly set on the app struct.
		assert.Equal(name, app.Name)
		assert.Equal(testValues[apptypePos], app.Type)
		assert.Equal(nonexistentDocroot, app.Docroot)
		assert.EqualValues(testValues[phpVersionPos], app.PHPVersion)

		err = PrepDdevDirectory(testDir)
		assert.NoError(err)
	}
}

// TestConfigCommandDocrootDetection asserts the default docroot is detected.
func TestConfigCommandDocrootDetection(t *testing.T) {
	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)

	testMatrix := AvailableDocrootLocations()
	for index, testDocrootName := range testMatrix {
		testDir := testcommon.CreateTmpDir(fmt.Sprintf("TestConfigCommand_%v", index))

		// testcommon.Chdir()() and CleanupDir() checks their own errors (and exit)
		defer testcommon.CleanupDir(testDir)
		defer testcommon.Chdir(testDir)()

		// Create a document root folder.
		err := os.MkdirAll(filepath.Join(testDir, filepath.Join(testDocrootName)), 0755)
		if err != nil {
			t.Errorf("Could not create %s directory under %s", testDocrootName, testDir)
		}
		_, err = os.OpenFile(filepath.Join(testDir, filepath.Join(testDocrootName), "index.php"), os.O_RDONLY|os.O_CREATE, 0664)
		assert.NoError(err)

		// Create the ddevapp we'll use for testing.
		// This will not return an error, since there is no existing configuration.
		app, err := NewApp(testDir, true)
		assert.NoError(err)

		// Randomize some values to use for Stdin during testing.
		name := strings.ToLower(util.RandString(16))

		// Create an example input buffer that writes the site name, accepts the
		// default document root and provides a valid app type.
		input := fmt.Sprintf("%s\n\ndrupal8", name)
		scanner := bufio.NewScanner(strings.NewReader(input))
		util.SetInputScanner(scanner)

		restoreOutput := util.CaptureStdOut()
		err = app.PromptForConfig()
		assert.NoError(err, t)
		out := restoreOutput()

		assert.Contains(out, fmt.Sprintf("Docroot Location (%s)", testDocrootName))

		// Ensure values were properly set on the app struct.
		assert.Equal(name, app.Name)
		assert.Equal(nodeps.AppTypeDrupal8, app.Type)
		assert.Equal(testDocrootName, app.Docroot)
		err = PrepDdevDirectory(testDir)
		assert.NoError(err)
	}
}

// TestConfigCommandDocrootDetection asserts the default docroot is detected and has index.php.
// The `web` docroot check is before `docroot` this verifies the directory with an
// existing index.php is selected.
func TestConfigCommandDocrootDetectionIndexVerification(t *testing.T) {
	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)

	testDir := testcommon.CreateTmpDir("TestConfigCommand_testDocrootIndex")

	// testcommon.Chdir()() and CleanupDir() checks their own errors (and exit)
	defer testcommon.CleanupDir(testDir)
	defer testcommon.Chdir(testDir)()

	// Create a document root folder.
	err := os.MkdirAll(filepath.Join(testDir, filepath.Join("web")), 0755)
	if err != nil {
		t.Errorf("Could not create %s directory under %s", "web", testDir)
	}
	err = os.MkdirAll(filepath.Join(testDir, filepath.Join("docroot")), 0755)
	if err != nil {
		t.Errorf("Could not create %s directory under %s", "docroot", testDir)
	}
	_, err = os.OpenFile(filepath.Join(testDir, "docroot", "index.php"), os.O_RDONLY|os.O_CREATE, 0664)
	assert.NoError(err)

	// Create the ddevapp we'll use for testing.
	// This will not return an error, since there is no existing configuration.
	app, err := NewApp(testDir, true)
	assert.NoError(err)

	// Randomize some values to use for Stdin during testing.
	name := strings.ToLower(util.RandString(16))

	// Create an example input buffer that writes the site name, accepts the
	// default document root and provides a valid app type.
	input := fmt.Sprintf("%s\n\ndrupal8", name)
	scanner := bufio.NewScanner(strings.NewReader(input))
	util.SetInputScanner(scanner)

	restoreOutput := util.CaptureStdOut()
	err = app.PromptForConfig()
	assert.NoError(err, t)
	out := restoreOutput()

	assert.Contains(out, fmt.Sprintf("Docroot Location (%s)", "docroot"))

	// Ensure values were properly set on the app struct.
	assert.Equal(name, app.Name)
	assert.Equal(nodeps.AppTypeDrupal8, app.Type)
	assert.Equal("docroot", app.Docroot)
	err = PrepDdevDirectory(testDir)
	assert.NoError(err)
}

// TestReadConfig tests reading config values from file and fallback to defaults for values not exposed.
func TestReadConfig(t *testing.T) {
	assert := asrt.New(t)

	// This closely resembles the values one would have from NewApp()
	app := &DdevApp{
		ConfigPath: filepath.Join("testdata", "config.yaml"),
		AppRoot:    "testdata",
		Name:       "TestRead",
	}

	_, err := app.ReadConfig(false)
	if err != nil {
		t.Fatalf("Unable to c.ReadConfig(), err: %v", err)
	}

	// Values not defined in file, we should still have default values
	assert.Equal(app.Name, "TestRead")

	// Values defined in file, we should have values from file
	assert.Equal(app.Type, nodeps.AppTypeDrupal8)
	assert.Equal(app.Docroot, "test")
	assert.Equal(app.WebImage, "test/testimage:latest")
}

// TestReadConfigCRLF tests reading config values from a file with Windows
// CRLF line endings in it.
func TestReadConfigCRLF(t *testing.T) {
	assert := asrt.New(t)

	// This closely resembles the values one would have from NewApp()
	app := &DdevApp{
		ConfigPath: filepath.Join("testdata", t.Name(), ".ddev", "config.yaml"),
		AppRoot:    filepath.Join("testdata", t.Name()),
		Name:       t.Name(),
	}

	_, err := app.ReadConfig(false)
	if err != nil {
		t.Fatalf("Unable to c.ReadConfig(), err: %v", err)
	}

	// Values not defined in file, we should still have default values
	assert.Equal(app.Name, t.Name())

	// Values defined in file, we should have values from file
	assert.Equal(app.Docroot, "public")
}

// TestConfigValidate tests validation of configuration values.
func TestConfigValidate(t *testing.T) {
	assert := asrt.New(t)

	//pwd, err := os.Getwd()
	//assert.NoError(err)

	site := TestSites[0]
	app, err := NewApp(site.Dir, false)
	assert.NoError(err)

	t.Cleanup(func() {
		err = app.Stop(true, false)
		assert.NoError(err)
	})

	appName := app.Name
	appType := app.Type

	err = app.ValidateConfig()
	if err != nil {
		t.Fatalf("Failed to app.ValidateConfig(), err=%v", err)
	}

	app.Name = "Invalid!"
	err = app.ValidateConfig()
	assert.Error(err)
	assert.Contains(err.Error(), "not a valid project name")

	app.Name = appName
	app.Type = "potato"
	err = app.ValidateConfig()
	assert.Error(err)
	assert.Contains(err.Error(), "invalid app type")

	app.Type = appType
	app.PHPVersion = "1.1"
	err = app.ValidateConfig()
	assert.Error(err)
	assert.Contains(err.Error(), "unsupported PHP")

	app.PHPVersion = nodeps.PHPDefault
	app.WebserverType = "server"
	err = app.ValidateConfig()
	assert.Error(err)
	assert.Contains(err.Error(), "unsupported webserver type")

	app.WebserverType = nodeps.WebserverDefault
	app.AdditionalHostnames = []string{"good", "b@d"}
	err = app.ValidateConfig()
	assert.Error(err)
	assert.Contains(err.Error(), "invalid hostname")

	app.AdditionalHostnames = []string{}
	app.AdditionalFQDNs = []string{"good.com", "b@d.com"}
	err = app.ValidateConfig()
	assert.Error(err)
	assert.Contains(err.Error(), "invalid hostname")

	app.AdditionalFQDNs = []string{}
	// Timezone validation isn't possible on Windows.
	if runtime.GOOS != "windows" {
		app.Timezone = "xxx"
		err = app.ValidateConfig()
		assert.Error(err)
		app.Timezone = "America/Chicago"
		err = app.ValidateConfig()
		assert.NoError(err)
	}

	// Make sure that wildcards work
	app.AdditionalHostnames = []string{"x", "*.any"}
	err = app.ValidateConfig()
	assert.NoError(err)
	err = app.Start()
	assert.NoError(err)
	staticURI := site.Safe200URIWithExpectation.URI
	_, _, err = testcommon.GetLocalHTTPResponse(t, "http://x.ddev.site/"+staticURI)
	assert.NoError(err)
	_, _, err = testcommon.GetLocalHTTPResponse(t, "http://somethjingrandom.any.ddev.site/"+staticURI)
	assert.NoError(err)

	// Make sure that a bare "*" in the additional_hostnames does *not* work
	app.AdditionalHostnames = []string{"x", "*"}
	err = app.ValidateConfig()
	assert.Error(err)
}

// TestWriteConfig tests writing config values to file
func TestWriteConfig(t *testing.T) {
	assert := asrt.New(t)

	pwd, _ := os.Getwd()

	projDir, err := filepath.Abs(testcommon.CreateTmpDir("TestWriteConfig"))
	require.NoError(t, err)

	err = fileutil.CopyDir("./testdata/TestWriteConfig/.ddev", filepath.Join(projDir, ".ddev"))
	require.NoError(t, err)

	app, err := NewApp(projDir, true)
	assert.NoError(err)
	err = os.Chdir(projDir)
	assert.NoError(err)

	t.Cleanup(func() {
		err = app.Stop(true, false)
		assert.NoError(err)
		err = os.Chdir(pwd)
		assert.NoError(err)
		err = os.RemoveAll(projDir)
		assert.NoError(err)
	})

	// The default NewApp read should read config overrides, so we should have "config.extra.yaml"
	// as the APIVersion
	assert.Equal("drupal9", app.Type)

	// However, if we ReadConfig() without includeOverrides, we should get "php" as the type
	_, err = app.ReadConfig(false)
	assert.NoError(err)
	assert.Equal("php", app.Type)

	err = app.WriteConfig()
	assert.NoError(err)

	// Now read the config we just wrote; it should have type php because ignored overrides.
	_, err = app.ReadConfig(false)
	assert.NoError(err)
	// app.WriteConfig() writes the version.DdevVersion to the updated config.yaml
	assert.Equal("php", app.Type)
}

// TestConfigOverrideDetection tests to make sure we tell them about config overrides.
func TestConfigOverrideDetection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping because unreliable on Windows")
	}
	testDataDdevDir := filepath.Join("testdata", t.Name(), ".ddev")

	assert := asrt.New(t)
	app := &DdevApp{}
	testDir, _ := os.Getwd()

	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()

	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s ConfigOverrideDetection", site.Name))
	runTime()

	// Copy test overrides into the project .ddev directory
	for _, item := range []string{"nginx", "nginx_full", "apache", "php", "mysql"} {
		_ = os.RemoveAll(filepath.Join(site.Dir, ".ddev", item))
		err := fileutil.CopyDir(filepath.Join(testDir, testDataDdevDir, item), filepath.Join(site.Dir, ".ddev", item))
		assert.NoError(err)
	}

	testcommon.ClearDockerEnv()
	err := app.Init(site.Dir)
	assert.NoError(err)

	t.Cleanup(func() {
		_ = app.Stop(true, false)
		for _, item := range []string{"apache", "php", "mysql", "nginx", "nginx_full"} {
			f := app.GetConfigPath(item)
			err = os.RemoveAll(f)
			if err != nil {
				t.Logf("failed to delete %s: %v", f, err)
			}
		}
	})

	stdoutFunc, err := util.CaptureOutputToFile()
	assert.NoError(err)
	startErr := app.StartAndWait(2)
	stdout := stdoutFunc()

	var logs string
	if startErr != nil {
		logs, _ = GetErrLogsFromApp(app, startErr)
	}

	require.NoError(t, startErr, "app.StartAndWait() did not succeed: output:\n=====\n%s\n===== logs:\n========= logs =======\n%s\n========\n", stdout, logs)

	assert.Contains(stdout, "collation.cnf")
	assert.Contains(stdout, "my-php.ini")

	switch app.WebserverType {
	case nodeps.WebserverNginxFPM:
		assert.Contains(stdout, "nginx-site.conf")
		assert.NotContains(stdout, "apache-site.conf")
		assert.Contains(stdout, "junker99.conf")
	default:
		assert.Contains(stdout, "apache-site.conf")
		assert.NotContains(stdout, "nginx-site.conf")
	}
	assert.Contains(stdout, "Custom configuration takes effect")
	runTime()
}

// TestPHPOverrides tests to make sure that PHP overrides work in all webservers.
func TestPHPOverrides(t *testing.T) {
	assert := asrt.New(t)
	app := &DdevApp{}
	tDir, err := os.Getwd()
	require.NoError(t, err)

	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()

	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s %s", site.Name, t.Name()))

	// Copy test overrides into the project .ddev directory
	err = fileutil.CopyDir(filepath.Join(tDir, "testdata/TestPHPOverrides/.ddev/php"), filepath.Join(site.Dir, ".ddev/php"))
	assert.NoError(err)
	err = fileutil.CopyFile(filepath.Join(tDir, "testdata/TestPHPOverrides/phpinfo.php"), filepath.Join(site.Dir, site.Docroot, "phpinfo.php"))
	assert.NoError(err)

	testcommon.ClearDockerEnv()
	err = app.Init(site.Dir)
	assert.NoError(err)
	_ = app.Stop(true, false)

	// And when we're done, we have to clean those out again.
	t.Cleanup(func() {
		runTime()
		_ = app.Stop(true, false)

		err = os.RemoveAll(filepath.Join(site.Dir, ".ddev/php"))
		if err != nil {
			t.Logf("failed to remove .ddev/php: %v", err)
		}
		err = os.RemoveAll(filepath.Join(site.Dir, "phpinfo.php"))
		if err != nil {
			t.Logf("failed to remove phpinfo.php: %v", err)
		}
	})

	startErr := app.StartAndWait(5)
	if startErr != nil {
		logs, _ := GetErrLogsFromApp(app, startErr)
		t.Logf("failed app.StartAndWait(): %v", startErr)
		t.Fatalf("============== logs from app.StartAndWait() ==============\n%s\n", logs)
	}

	_, _ = testcommon.EnsureLocalHTTPContent(t, "http://"+app.GetHostname()+"/phpinfo.php", `max_input_time</td><td class="v">999`)
	err = app.Stop(true, false)
	assert.NoError(err)

}

// TestExtraPackages tests to make sure that *extra_packages config.yaml directives
// work (and are overridden by *-build/Dockerfile).
func TestExtraPackages(t *testing.T) {
	assert := asrt.New(t)
	app := &DdevApp{}

	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()

	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s %s", site.Name, t.Name()))

	err := app.Init(site.Dir)
	assert.NoError(err)
	err = app.Stop(true, false)
	assert.NoError(err)

	// Make sure no "-built" items are still hanging around
	command := fmt.Sprintf("docker rmi -f %s-%s-built %s-%s-built", app.WebImage, app.Name, app.GetDBImage(), app.Name)
	_, _ = exec.RunCommand("bash", []string{"-c", command})

	t.Cleanup(func() {
		runTime()
		_ = app.Stop(true, false)
		app.WebImageExtraPackages = nil
		app.DBImageExtraPackages = nil
		app.PHPVersion = nodeps.PHPDefault
		_ = app.WriteConfig()
		_ = fileutil.RemoveContents(app.GetConfigPath("web-build"))
		_ = fileutil.RemoveContents(app.GetConfigPath("db-build"))
		command := fmt.Sprintf("docker rmi -f %s-%s-built %s-%s-built >/dev/null 2>&1", app.WebImage, app.Name, app.GetDBImage(), app.Name)
		_, _ = exec.RunCommand("bash", []string{"-c", command})
	})

	// Start and make sure that the packages don't exist already
	err = app.Start()
	assert.NoError(err)

	// Test db container to make sure no ncdu in there at beginning
	_, _, err = app.Exec(&ExecOpts{
		Service: "db",
		Cmd:     "command -v ncdu 2>/dev/null",
	})
	assert.Error(err)
	assert.Contains(err.Error(), "exit status 1")

	addedPackage := "tidy"
	addedPackageTitle := "Tidy"

	_, _, err = app.Exec(&ExecOpts{
		Service: "web",
		Cmd:     fmt.Sprintf("dpkg -s php%s-%s >/dev/null 2>&1", app.PHPVersion, addedPackage),
	})
	assert.Error(err)
	assert.Contains(err.Error(), "exit status 1")

	// Now add the packages and start again, they should be in there
	app.WebImageExtraPackages = []string{"php" + app.PHPVersion + "-" + addedPackage}
	app.DBImageExtraPackages = []string{"ncdu"}
	err = app.Start()
	assert.NoError(err)

	stdout, stderr, err := app.Exec(&ExecOpts{
		Service: "web",
		Cmd:     "dpkg -s php" + app.PHPVersion + "-" + addedPackage,
	})
	assert.NoError(err, "dpkg -s php%s-%s failed", app.PHPVersion, addedPackage, stdout, stderr)

	stdout, stderr, err = app.Exec(&ExecOpts{
		Service: "web",
		Cmd:     fmt.Sprintf("php -i | grep  '%s support =. enabled'", addedPackageTitle),
	})
	assert.NoError(err, "failed to grep for %s support, stdout=%s, stderr=%s", addedPackage, stdout, stderr)

	stdout, _, err = app.Exec(&ExecOpts{
		Service: "db",
		Cmd:     "command -v ncdu",
	})
	assert.NoError(err)
	assert.Equal("/usr/bin/ncdu", strings.Trim(stdout, "\n"))
}

// TestTimezoneConfig tests to make sure setting timezone config takes effect in the container.
func TestTimezoneConfig(t *testing.T) {
	assert := asrt.New(t)
	app := &DdevApp{}

	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()

	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s %s", t.Name(), site.Name))

	err := app.Init(site.Dir)
	assert.NoError(err)
	err = app.Stop(true, false)
	assert.NoError(err)

	t.Cleanup(func() {
		err = app.Stop(true, false)
		assert.NoError(err)
	})

	err = app.Start()
	assert.NoError(err)

	// Without timezone set, we should find Etc/UTC
	stdout, _, err := app.Exec(&ExecOpts{
		Service: "web",
		Cmd:     "printf \"timezone=$(date +%Z)\n\" && php -r 'print \"phptz=\" . date_default_timezone_get();'",
	})
	assert.NoError(err)
	assert.Equal("timezone=UTC\nphptz=UTC", stdout)

	// Make sure db container is also working
	stdout, _, err = app.Exec(&ExecOpts{
		Service: "db",
		Cmd:     "echo -n timezone=$(date +%Z)",
	})
	assert.NoError(err)
	assert.Equal("timezone=UTC", stdout)

	// With timezone set, we the correct timezone operational
	app.Timezone = "Europe/Paris"
	err = app.Start()
	require.NoError(t, err)
	stdout, _, err = app.Exec(&ExecOpts{
		Service: "web",
		Cmd:     "printf \"timezone=$(date +%Z)\n\" && php -r 'print \"phptz=\" . date_default_timezone_get();'",
	})
	assert.NoError(err)
	assert.Regexp(regexp.MustCompile("timezone=CES?T\nphptz=Europe/Paris"), stdout)

	// Make sure db container is also working with CET
	stdout, _, err = app.Exec(&ExecOpts{
		Service: "db",
		Cmd:     "echo -n timezone=$(date +%Z)",
	})
	assert.NoError(err)
	assert.Regexp(regexp.MustCompile("timezone=CES?T"), stdout)

	runTime()
}

// TestComposerVersionConfig tests to make sure setting composer version takes effect in the container.
func TestComposerVersionConfig(t *testing.T) {
	assert := asrt.New(t)
	app := &DdevApp{}
	testVersion := "2.0.0-RC2"

	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()

	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s %s", t.Name(), site.Name))

	err := app.Init(site.Dir)
	assert.NoError(err)
	err = app.Stop(true, false)
	assert.NoError(err)

	t.Cleanup(func() {
		app.ComposerVersion = ""
		err = app.WriteConfig()
		assert.NoError(err)
		err = app.Stop(true, false)
		assert.NoError(err)
	})

	app.ComposerVersion = testVersion
	err = app.Start()
	assert.NoError(err)

	// Without timezone set, we should find Etc/UTC
	stdout, _, err := app.Exec(&ExecOpts{
		Service: "web",
		Cmd:     "composer --version | awk '{print $3;}'",
	})
	assert.NoError(err)
	assert.Equal(testVersion, strings.Trim(stdout, "\n"))

	runTime()
}

// TestCustomBuildDockerfiles tests to make sure that custom web-build and db-build
// Dockerfiles work properly
func TestCustomBuildDockerfiles(t *testing.T) {
	assert := asrt.New(t)
	app := &DdevApp{}

	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()

	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s TestCustomBuildDockerfiles", site.Name))

	err := app.Init(site.Dir)
	assert.NoError(err)
	err = app.Stop(true, false)
	assert.NoError(err)

	defer func() {
		_ = os.RemoveAll(app.GetConfigPath("web-build"))
		_ = os.RemoveAll(app.GetConfigPath("db-build"))
		_ = app.Stop(true, false)
	}()

	// Create simple dockerfiles that just touch /var/tmp/added-by-<container>txt
	for _, item := range []string{"web", "db"} {
		err = WriteImageDockerfile(app.GetConfigPath(item+"-build/Dockerfile"), []byte(`
ARG BASE_IMAGE
FROM $BASE_IMAGE
RUN touch /var/tmp/`+"added-by-"+item+".txt"))
		assert.NoError(err)
	}
	// Start and make sure that the packages don't exist already
	err = app.Start()
	assert.NoError(err)

	// Make sure that the expected in-container file has been created
	for _, item := range []string{"web", "db"} {
		_, _, err = app.Exec(&ExecOpts{
			Service: item,
			Cmd:     "ls /var/tmp/added-by-" + item + ".txt",
		})
		assert.NoError(err)
	}

	err = app.Stop(true, false)
	assert.NoError(err)

	runTime()
}

// TestConfigLoadingOrder verifies that configs load in lexicographical order
// AFTER config.yaml
func TestConfigLoadingOrder(t *testing.T) {
	assert := asrt.New(t)
	pwd, _ := os.Getwd()

	projDir, err := filepath.Abs(testcommon.CreateTmpDir(t.Name()))
	require.NoError(t, err)

	err = fileutil.CopyDir("./testdata/TestConfigLoadingOrder/.ddev", filepath.Join(projDir, ".ddev"))
	require.NoError(t, err)

	app, err := NewApp(projDir, true)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = os.Chdir(pwd)
		assert.NoError(err)
		err = app.Stop(true, false)
		assert.NoError(err)
		err = os.RemoveAll(projDir)
		assert.NoError(err)
	})
	err = os.Chdir(app.AppRoot)
	assert.NoError(err)
	_, err = app.ReadConfig(true)
	assert.NoError(err)
	assert.Equal("config.yaml", app.WebImage)

	matches, err := filepath.Glob(filepath.Join(projDir, ".ddev/linkedconfigs/config.*.y*ml"))
	assert.NoError(err)

	// First make sure that each possible item in .ddev/linkedconfigs can override by itself
	for _, item := range matches {
		linkedMatch := filepath.Join(app.AppConfDir(), filepath.Base(item))
		assert.NoError(err)
		err = os.Symlink(item, linkedMatch)
		assert.NoError(err)
		_, err = app.ReadConfig(true)
		assert.NoError(err)
		assert.Equal(filepath.Base(item), app.WebImage)
		err = os.Remove(linkedMatch)
		assert.NoError(err)
	}

	// Now make sure that the matches are in lexicographical order and each larger one can override the others
	orderedMatches := matches
	sort.Strings(orderedMatches)
	assert.Equal(matches, orderedMatches)

	for _, item := range matches {
		linkedMatch := filepath.Join(app.AppConfDir(), filepath.Base(item))
		assert.NoError(err)
		err = os.Symlink(item, linkedMatch)
		assert.NoError(err)
		_, err = app.ReadConfig(true)
		assert.Equal(filepath.Base(item), app.WebImage)
	}

	// Now we still have all those linked overrides, but do a ReadConfig() without allowing them
	// and verify that they don't get loaded
	_, err = app.ReadConfig(false)
	assert.NoError(err)
	assert.Equal("config.yaml", app.WebImage)

}

func TestPkgConfigMariaDBVersion(t *testing.T) {
	// NewApp from scratch
	// NewApp with config.yaml with mariadb_version and no dbimage
	// NewApp with config.yaml with dbimage and no mariadb_version
	// NewApp with both dbimage and
	assert := asrt.New(t)

	testDir, _ := os.Getwd()

	// Create a temporary directory and switch to it.
	tmpDir := testcommon.CreateTmpDir(t.Name())
	defer testcommon.CleanupDir(tmpDir)
	defer testcommon.Chdir(tmpDir)()

	systemTempDir, _ := testcommon.OsTempDir()

	targetBase := filepath.Join(systemTempDir, "TestPkgConfigMariaDBVersion")
	_ = os.RemoveAll(targetBase)
	err := fileutil.CopyDir(filepath.Join(testDir, "testdata", "TestPkgConfigMariaDBVersion"), targetBase)
	require.NoError(t, err)

	mariaDBVersions := nodeps.ValidMariaDBVersions
	for v := range mariaDBVersions {
		for _, configType := range []string{"dbimage", "mariadb-version"} {
			app := &DdevApp{}
			appRoot := filepath.Join(targetBase, configType+"-"+v)
			err = app.LoadConfigYamlFile(filepath.Join(appRoot, ".ddev", "config.yaml"))
			assert.NoError(err)
			if configType == "dbimage" {
				assert.Equal("somedbimage:"+v, app.DBImage)
			}
			if configType == "mariadb-version" {
				assert.Equal(v, app.MariaDBVersion)
			}

			app, err = NewApp(appRoot, false)
			assert.NoError(err)
			if configType == "dbimage" {
				assert.Equal("somedbimage:"+v, app.DBImage, "NewApp() failed to respect existing dbimage")
			}
			if configType == "mariadb-version" {
				assert.Equal(v, app.MariaDBVersion)
				assert.Equal(version.GetDBImage(nodeps.MariaDB, v), app.GetDBImage(), "dbimage derived from app.MariaDBVersion was incorrect")
			}

		}
	}

}
