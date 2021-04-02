package ddevapp_test

import (
	"fmt"
	"github.com/drud/ddev/pkg/exec"
	"github.com/drud/ddev/pkg/globalconfig"
	"github.com/drud/ddev/pkg/nodeps"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/drud/ddev/pkg/ddevapp"
	"github.com/drud/ddev/pkg/testcommon"
	asrt "github.com/stretchr/testify/assert"
)

/**
 * These tests rely on an external test account managed by DRUD. To run them, you'll
 * need to set an environment variable called "DDEV_DDEVLIVE_API_TOKEN" with credentials for
 * this account. If no such environment variable is present, these tests will be skipped.
 *
 * A valid site (with backups) must be present which matches the test site and environment name
 * defined in the constants below.
 */
const ddevliveTestSite = "ddltest/ddev-live-test-no-delete"

var ddevLiveDBBackupName = ""

// TestDdevLivePull ensures we can pull backups from DDEV-Live
func TestDdevLivePull(t *testing.T) {
	token := ""
	if token = os.Getenv("DDEV_DDEVLIVE_API_TOKEN"); token == "" {
		t.Skipf("No DDEV_DDEVLIVE_API_TOKEN env var has been set. Skipping %v", t.Name())
	}
	if ddevLiveDBBackupName = os.Getenv("DDEV_DDEVLIVE_PULL_BACKUP"); ddevLiveDBBackupName == "" {
		t.Skipf("No DDEV_DDEVLIVE_PULL_BACKUP env var has been set. Skipping %v", t.Name())
	}

	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)
	testDir, _ := os.Getwd()

	webEnvSave := globalconfig.DdevGlobalConfig.WebEnvironment
	// DDEV_LIVE_NO_ANALYTICS will be picked up by the docker-compose and pushed into web
	_ = os.Setenv("DDEV_LIVE_NO_ANALYTICS", "true")
	globalconfig.DdevGlobalConfig.WebEnvironment = []string{"DDEV_LIVE_API_TOKEN=" + token}
	err := globalconfig.WriteGlobalConfig(globalconfig.DdevGlobalConfig)
	require.NoError(t, err)

	siteDir := testcommon.CreateTmpDir(t.Name())
	err = os.MkdirAll(filepath.Join(siteDir, "web/sites/default"), 0777)
	require.NoError(t, err)
	err = os.Chdir(siteDir)
	require.NoError(t, err)

	app, err := NewApp(siteDir, true)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = app.Stop(true, false)
		assert.NoError(err)

		globalconfig.DdevGlobalConfig.WebEnvironment = webEnvSave
		err = globalconfig.WriteGlobalConfig(globalconfig.DdevGlobalConfig)
		assert.NoError(err)

		_ = os.Chdir(testDir)
		err = os.RemoveAll(siteDir)
		assert.NoError(err)
	})

	app.Type = nodeps.AppTypeDrupal8
	app.Hooks = map[string][]YAMLTask{"post-pull": {{"exec-host": "touch hello-post-pull-" + app.Name}}, "pre-pull": {{"exec-host": "touch hello-pre-pull-" + app.Name}}}

	_ = app.Stop(true, false)

	err = app.WriteConfig()
	require.NoError(t, err)

	testcommon.ClearDockerEnv()

	// Run ddev once to create all the files in .ddev, including the example
	_, err = exec.RunCommand("bash", []string{"-c", fmt.Sprintf("%s >/dev/null", DdevBin)})
	require.NoError(t, err)

	err = app.Start()
	require.NoError(t, err)

	// Build our ddev-live.yaml from the example file
	s, err := ioutil.ReadFile(app.GetConfigPath("providers/ddev-live.yaml.example"))
	require.NoError(t, err)
	x := strings.Replace(string(s), "project_id:", fmt.Sprintf("project_id: %s\n#project_id:", ddevliveTestSite), -1)
	x = strings.Replace(x, "database_backup:", fmt.Sprintf("database_backup: %s\n#database_backup: ", ddevLiveDBBackupName), -1)
	x = strings.Replace(x, "# set -x", "set -x", -1)
	err = ioutil.WriteFile(app.GetConfigPath("providers/ddev-live.yaml"), []byte(x), 0666)
	require.NoError(t, err)
	err = app.WriteConfig()
	require.NoError(t, err)

	provider, err := app.GetProvider("ddev-live")
	require.NoError(t, err)
	err = app.Pull(provider, false, false, false)
	require.NoError(t, err)

	assert.FileExists(filepath.Join(app.GetUploadDir(), "chocolate-brownie-umami.jpg"))
	out, err := exec.RunCommand("bash", []string{"-c", fmt.Sprintf(`echo 'select COUNT(*) from users_field_data where mail="nobody@example.com";' | %s mysql -N`, DdevBin)})
	assert.NoError(err)
	assert.True(strings.HasPrefix(out, "1\n"))

	assert.FileExists("hello-pre-pull-" + app.Name)
	assert.FileExists("hello-post-pull-" + app.Name)
	err = os.Remove("hello-pre-pull-" + app.Name)
	assert.NoError(err)
	err = os.Remove("hello-post-pull-" + app.Name)
	assert.NoError(err)
}

// TestDdevLivePush ensures we can push to ddev-live.
func TestDdevLivePush(t *testing.T) {
	token := ""
	if token = os.Getenv("DDEV_DDEVLIVE_API_TOKEN"); token == "" {
		t.Skipf("No DDEV_DDEVLIVE_API_TOKEN env var has been set. Skipping %v", t.Name())
	}
	if ddevLiveDBBackupName = os.Getenv("DDEV_DDEVLIVE_PULL_BACKUP"); ddevLiveDBBackupName == "" {
		t.Skipf("No DDEV_DDEVLIVE_PULL_BACKUP env var has been set. Skipping %v", t.Name())
	}

	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)
	origDir, _ := os.Getwd()

	webEnvSave := globalconfig.DdevGlobalConfig.WebEnvironment
	// DDEV_LIVE_NO_ANALYTICS will be picked up by the docker-compose and pushed into web
	_ = os.Setenv("DDEV_LIVE_NO_ANALYTICS", "true")
	globalconfig.DdevGlobalConfig.WebEnvironment = []string{"DDEV_LIVE_API_TOKEN=" + token}
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

	// Build our ddev-live.yaml from the example file
	s, err := ioutil.ReadFile(app.GetConfigPath("providers/ddev-live.yaml.example"))
	require.NoError(t, err)
	x := strings.Replace(string(s), "project_id:", fmt.Sprintf("project_id: %s\n#project_id:", ddevliveTestSite), -1)
	x = strings.Replace(x, "database_backup:", fmt.Sprintf("database_backup: %s\n#database_backup: ", ddevLiveDBBackupName), -1)
	err = ioutil.WriteFile(app.GetConfigPath("providers/ddev-live.yaml"), []byte(x), 0666)
	assert.NoError(err)
	err = app.WriteConfig()
	require.NoError(t, err)

	provider, err := app.GetProvider("ddev-live")
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
	err = ioutil.WriteFile(filepath.Join(siteDir, "sites/default/files", fName), fContent, 0644)
	assert.NoError(err)

	err = app.Push(provider, false, false)
	require.NoError(t, err)

	// TODO: Unfortunately ddev-live exec doesn't provide the capabilities to do anything
	// like this query. Quote marks seem to be dropped, etc
	// Test that the database row was added
	//out, _, err := app.Exec(&ExecOpts{
	//	Cmd: fmt.Sprintf(`ddev-live exec %s -- echo 'SELECT title FROM %s WHERE title="%s";' | drush sql-cli`, ddevliveTestSite, t.Name(), tval),
	//})
	//require.NoError(t, err)
	//assert.Contains(out, tval)

	// Test that the file arrived there (by execing a cat of it)
	out, _, err := app.Exec(&ExecOpts{
		Cmd: fmt.Sprintf(`ddev-live exec %s -- cat %s `, ddevliveTestSite, path.Join("sites/default/files", fName)),
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
