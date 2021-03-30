package ddevapp_test

import (
	"fmt"
	"github.com/drud/ddev/pkg/exec"
	"github.com/drud/ddev/pkg/globalconfig"
	"github.com/drud/ddev/pkg/nodeps"
	"github.com/drud/ddev/pkg/util"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/drud/ddev/pkg/ddevapp"
	"github.com/drud/ddev/pkg/testcommon"
	asrt "github.com/stretchr/testify/assert"
)

/**
 * These tests rely on an external test account managed by DRUD. To run them, you'll
 * need to set an environment variable called "DDEV_PLATFORM_API_TOKEN" with credentials for
 * this account. If no such environment variable is present, these tests will be skipped.
 *
 * A valid site must be present which matches the test site and environment name
 * defined in the constants below.
 */

var platformTestSiteID = "lago3j23xu2w6"
var platformTestSiteEnvironment = "master"

// TestPlatformPull ensures we can pull backups from platform.sh for a configured environment.
func TestPlatformPull(t *testing.T) {
	var token string
	if token = os.Getenv("DDEV_PLATFORM_API_TOKEN"); token == "" {
		t.Skipf("No DDEV_PLATFORM_API_TOKEN env var has been set. Skipping %v", t.Name())
	}
	assert := asrt.New(t)
	var err error

	webEnvSave := globalconfig.DdevGlobalConfig.WebEnvironment

	testDir, _ := os.Getwd()

	siteDir := testcommon.CreateTmpDir(t.Name())

	err = os.Chdir(siteDir)
	assert.NoError(err)
	app, err := NewApp(siteDir, true)
	assert.NoError(err)
	app.Name = t.Name()
	app.Type = nodeps.AppTypeDrupal9
	err = app.Stop(true, false)
	require.NoError(t, err)
	err = app.WriteConfig()
	require.NoError(t, err)

	globalconfig.DdevGlobalConfig.WebEnvironment = []string{"PLATFORMSH_CLI_TOKEN=" + token}
	err = globalconfig.WriteGlobalConfig(globalconfig.DdevGlobalConfig)
	assert.NoError(err)

	testcommon.ClearDockerEnv()

	t.Cleanup(func() {
		globalconfig.DdevGlobalConfig.WebEnvironment = webEnvSave
		err = globalconfig.WriteGlobalConfig(globalconfig.DdevGlobalConfig)
		assert.NoError(err)

		err = app.Stop(true, false)
		assert.NoError(err)

		_ = os.Chdir(testDir)
		err = os.RemoveAll(siteDir)
		assert.NoError(err)
	})

	_, err = exec.RunCommand("bash", []string{"-c", fmt.Sprintf("%s >/dev/null", DdevBin)})
	require.NoError(t, err)

	// Build our platform.yaml from the example file
	s, err := os.ReadFile(app.GetConfigPath("providers/platform.yaml.example"))
	require.NoError(t, err)
	x := strings.Replace(string(s), "project_id:", fmt.Sprintf("project_id: "+platformTestSiteID+"\n#project_id:"), 1)
	err = os.WriteFile(app.GetConfigPath("providers/platform.yaml"), []byte(x), 0666)
	assert.NoError(err)
	err = app.WriteConfig()
	require.NoError(t, err)

	provider, err := app.GetProvider("platform")
	require.NoError(t, err)
	err = app.Start()
	require.NoError(t, err)
	err = app.Pull(provider, false, false, false)
	assert.NoError(err)

	assert.FileExists(filepath.Join(app.GetUploadDir(), "victoria-sponge-umami.jpg"))
	out, err := exec.RunCommand("bash", []string{"-c", fmt.Sprintf(`echo 'select COUNT(*) from users_field_data where mail="margaret.hopper@example.com";' | %s mysql -N`, DdevBin)})
	assert.NoError(err)
	assert.True(strings.HasPrefix(out, "1\n"))
}

// TestPlatformPush ensures we can push to platform.sh for a configured environment.
func TestPlatformPush(t *testing.T) {
	var token string
	if token = os.Getenv("DDEV_PLATFORM_API_TOKEN"); token == "" {
		t.Skipf("No DDEV_PLATFORM_API_TOKEN env var has been set. Skipping %v", t.Name())
	}

	assert := asrt.New(t)
	origDir, _ := os.Getwd()

	webEnvSave := globalconfig.DdevGlobalConfig.WebEnvironment

	globalconfig.DdevGlobalConfig.WebEnvironment = []string{"PLATFORMSH_CLI_TOKEN=" + token}
	err := globalconfig.WriteGlobalConfig(globalconfig.DdevGlobalConfig)
	assert.NoError(err)

	siteDir := testcommon.CreateTmpDir(t.Name())

	err = os.Chdir(siteDir)
	require.NoError(t, err)

	app, err := NewApp(siteDir, true)
	assert.NoError(err)

	t.Cleanup(func() {
		err = app.Stop(true, false)
		assert.NoError(err)

		globalconfig.DdevGlobalConfig.WebEnvironment = webEnvSave
		err = globalconfig.WriteGlobalConfig(globalconfig.DdevGlobalConfig)
		assert.NoError(err)

		_ = os.Chdir(origDir)
		err = os.RemoveAll(siteDir)
		assert.NoError(err)
	})

	app.Name = t.Name()
	app.Type = nodeps.AppTypeDrupal8
	app.Hooks = map[string][]YAMLTask{"post-push": {{"exec-host": "touch hello-post-push-" + app.Name}}, "pre-push": {{"exec-host": "touch hello-pre-push-" + app.Name}}}
	_ = app.Stop(true, false)

	err = app.WriteConfig()
	require.NoError(t, err)

	testcommon.ClearDockerEnv()

	// Run ddev once to create all the files in .ddev, including the example
	_, err = exec.RunCommand("bash", []string{"-c", fmt.Sprintf("%s >/dev/null", DdevBin)})
	require.NoError(t, err)

	// Build our platform.yaml from the example file
	s, err := os.ReadFile(app.GetConfigPath("providers/platform.yaml.example"))
	require.NoError(t, err)
	x := strings.Replace(string(s), "project_id:", fmt.Sprintf("project_id: %s\n#project_id:", platformTestSiteID), -1)
	err = os.WriteFile(app.GetConfigPath("providers/platform.yaml"), []byte(x), 0666)
	assert.NoError(err)
	err = app.WriteConfig()
	require.NoError(t, err)

	provider, err := app.GetProvider("platform")
	require.NoError(t, err)
	err = app.Start()
	require.NoError(t, err)

	// For this dummy site, do a pull to populate the database+files to begin with
	err = app.Pull(provider, false, false, false)
	require.NoError(t, err)

	// Create database and files entries that we can verify after push
	tval := nodeps.RandomString(10)
	_, _, err = app.Exec(&ExecOpts{
		Cmd: fmt.Sprintf(`mysql -e 'CREATE TABLE IF NOT EXISTS %s ( title VARCHAR(255) NOT NULL ); INSERT INTO %s VALUES("%s");'`, t.Name(), t.Name(), tval),
	})
	require.NoError(t, err)
	fName := tval + ".txt"
	fContent := []byte(tval)
	err = os.WriteFile(filepath.Join(siteDir, "sites/default/files", fName), fContent, 0644)
	assert.NoError(err)

	err = app.Push(provider, false, false)
	require.NoError(t, err)

	// Test that the database row was added
	out, _, err := app.Exec(&ExecOpts{
		Cmd: fmt.Sprintf(`echo 'SELECT title FROM %s WHERE title="%s";' | platform db:sql --project="%s" --environment="%s"`, t.Name(), tval, platformTestSiteID, platformTestSiteEnvironment),
	})
	require.NoError(t, err)
	assert.Contains(out, tval)

	// Test that the file arrived there (by rsyncing it back)
	tmpRsyncDir := filepath.Join("/tmp", t.Name()+util.RandString(5))
	out, _, err = app.Exec(&ExecOpts{
		Cmd: fmt.Sprintf(`platform mount:download --yes --quiet --project="%s" --environment="%s" --mount=web/sites/default/files --target=%s && cat %s/%s && rm -rf %s`, platformTestSiteID, platformTestSiteEnvironment, tmpRsyncDir, tmpRsyncDir, fName, tmpRsyncDir),
	})
	require.NoError(t, err)
	assert.Contains(out, tval)

	assert.FileExists("hello-pre-push-" + app.Name)
	assert.FileExists("hello-post-push-" + app.Name)
	err = os.Remove("hello-pre-push-" + app.Name)
	assert.NoError(err)
	err = os.Remove("hello-post-push-" + app.Name)
	assert.NoError(err)
}
