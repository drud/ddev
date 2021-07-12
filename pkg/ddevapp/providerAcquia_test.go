package ddevapp_test

import (
	"fmt"
	"github.com/drud/ddev/pkg/exec"
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/globalconfig"
	"github.com/drud/ddev/pkg/nodeps"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/drud/ddev/pkg/ddevapp"
	"github.com/drud/ddev/pkg/testcommon"
	asrt "github.com/stretchr/testify/assert"
)

/**
 * A valid site (with backups) must be present which matches the test site and environment name
 * defined in the constants below.
 */
const acquiaTestSite = "eeamoreno.dev"

// TestAcquiaPull ensures we can pull backups from Acquia
func TestAcquiaPull(t *testing.T) {
	acquiaKey := ""
	acquiaSecret := ""
	sshkey := ""
	if acquiaKey = os.Getenv("DDEV_ACQUIA_API_KEY"); acquiaKey == "" {
		t.Skipf("No DDEV_ACQUIA_KEY env var has been set. Skipping %v", t.Name())
	}
	if acquiaSecret = os.Getenv("DDEV_ACQUIA_API_SECRET"); acquiaSecret == "" {
		t.Skipf("No DDEV_ACQUIA_SECRET env var has been set. Skipping %v", t.Name())
	}
	if sshkey = os.Getenv("DDEV_ACQUIA_SSH_KEY"); sshkey == "" {
		t.Skipf("No DDEV_ACQUIA_SSH_KEY env var has been set. Skipping %v", t.Name())
	}
	sshkey = strings.Replace(sshkey, "<SPLIT>", "\n", -1)

	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)
	origDir, _ := os.Getwd()

	webEnvSave := globalconfig.DdevGlobalConfig.WebEnvironment

	globalconfig.DdevGlobalConfig.WebEnvironment = []string{"ACQUIA_API_KEY=" + acquiaKey, "ACQUIA_API_SECRET=" + acquiaSecret}
	err := globalconfig.WriteGlobalConfig(globalconfig.DdevGlobalConfig)
	assert.NoError(err)

	siteDir := testcommon.CreateTmpDir(t.Name())
	err = os.MkdirAll(filepath.Join(siteDir, "docroot/sites/default"), 0777)
	assert.NoError(err)
	err = os.Chdir(siteDir)
	assert.NoError(err)

	err = setupSSHKey(t, sshkey, filepath.Join(origDir, "testdata", t.Name()))
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
	app.Type = nodeps.AppTypeDrupal9
	app.ComposerVersion = "2"

	_ = app.Stop(true, false)
	err = app.WriteConfig()
	assert.NoError(err)

	testcommon.ClearDockerEnv()

	// Run ddev once to create all the files in .ddev, including the example
	_, err = exec.RunCommand("bash", []string{"-c", fmt.Sprintf("%s >/dev/null", DdevBin)})
	require.NoError(t, err)

	err = app.Start()
	require.NoError(t, err)

	_, err = exec.RunCommand("bash", []string{"-c", fmt.Sprintf("%s composer require drush/drush >/dev/null 2>&1", DdevBin)})
	require.NoError(t, err)

	// Build our acquia.yaml from the example file
	s, err := ioutil.ReadFile(app.GetConfigPath("providers/acquia.yaml.example"))
	require.NoError(t, err)
	x := strings.Replace(string(s), "project_id:", fmt.Sprintf("project_id: %s\n#project_id:", acquiaTestSite), -1)
	err = ioutil.WriteFile(app.GetConfigPath("providers/acquia.yaml"), []byte(x), 0666)
	assert.NoError(err)
	err = app.WriteConfig()
	require.NoError(t, err)

	provider, err := app.GetProvider("acquia")
	require.NoError(t, err)
	err = app.Pull(provider, false, false, false)
	require.NoError(t, err)

	assert.FileExists(filepath.Join(app.GetUploadDir(), "chocolate-brownie-umami.jpg"))
	out, err := exec.RunCommand("bash", []string{"-c", fmt.Sprintf(`echo 'select COUNT(*) from users_field_data where mail="randy@example.com";' | %s mysql -N`, DdevBin)})
	assert.NoError(err)
	assert.True(strings.HasPrefix(out, "1\n"))
}

// TestAcquiaPush ensures we can push to acquia for a configured environment.
func TestAcquiaPush(t *testing.T) {
	acquiaKey := ""
	acquiaSecret := ""
	sshkey := ""
	if acquiaKey = os.Getenv("DDEV_ACQUIA_API_KEY"); acquiaKey == "" {
		t.Skipf("No DDEV_ACQUIA_KEY env var has been set. Skipping %v", t.Name())
	}
	if acquiaSecret = os.Getenv("DDEV_ACQUIA_API_SECRET"); acquiaSecret == "" {
		t.Skipf("No DDEV_ACQUIA_SECRET env var has been set. Skipping %v", t.Name())
	}
	if sshkey = os.Getenv("DDEV_ACQUIA_SSH_KEY"); sshkey == "" {
		t.Skipf("No DDEV_ACQUIA_SSH_KEY env var has been set. Skipping %v", t.Name())
	}
	sshkey = strings.Replace(sshkey, "<SPLIT>", "\n", -1)

	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)
	origDir, _ := os.Getwd()

	webEnvSave := globalconfig.DdevGlobalConfig.WebEnvironment

	globalconfig.DdevGlobalConfig.WebEnvironment = []string{"ACQUIA_API_KEY=" + acquiaKey, "ACQUIA_API_SECRET=" + acquiaSecret}
	err := globalconfig.WriteGlobalConfig(globalconfig.DdevGlobalConfig)
	assert.NoError(err)

	siteDir := testcommon.CreateTmpDir(t.Name())

	// We have to have a d8 codebase for drush to work right
	d8code := FullTestSites[1]
	// If running this with GOTEST_SHORT we have to create the directory, tarball etc.
	if d8code.Dir == "" || !fileutil.FileExists(d8code.Dir) {
		err := d8code.Prepare()
		require.NoError(t, err)
		app, err := NewApp(d8code.Dir, false)
		require.NoError(t, err)
		_ = app.Stop(true, false)
	}
	_ = os.Remove(siteDir)
	err = fileutil.CopyDir(d8code.Dir, siteDir)
	require.NoError(t, err)
	err = os.Chdir(siteDir)
	require.NoError(t, err)

	err = setupSSHKey(t, sshkey, filepath.Join(origDir, "testdata", t.Name()))
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
	app.Type = nodeps.AppTypeDrupal9
	app.Hooks = map[string][]YAMLTask{"post-push": {{"exec-host": "touch hello-post-push-" + app.Name}}, "pre-push": {{"exec-host": "touch hello-pre-push-" + app.Name}}}
	_ = app.Stop(true, false)

	_ = app.Stop(true, false)
	err = app.WriteConfig()
	require.NoError(t, err)

	testcommon.ClearDockerEnv()

	// Run ddev once to create all the files in .ddev, including the example
	_, err = exec.RunCommand("bash", []string{"-c", fmt.Sprintf("%s >/dev/null", DdevBin)})
	require.NoError(t, err)

	// Build our acquia.yaml from the example file
	s, err := ioutil.ReadFile(app.GetConfigPath("providers/acquia.yaml.example"))
	require.NoError(t, err)
	x := strings.Replace(string(s), "project_id:", fmt.Sprintf("project_id: %s\n#project_id:", acquiaTestSite), -1)
	err = ioutil.WriteFile(app.GetConfigPath("providers/acquia.yaml"), []byte(x), 0666)
	assert.NoError(err)
	err = app.WriteConfig()
	require.NoError(t, err)

	provider, err := app.GetProvider("acquia")
	require.NoError(t, err)
	err = app.Start()
	require.NoError(t, err)

	// Make sure we have drush
	_, err = exec.RunCommand("bash", []string{"-c", fmt.Sprintf("%s composer require drush/drush >/dev/null 2>&1", DdevBin)})
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

	// Test that the database row was added
	out, _, err := app.Exec(&ExecOpts{
		Cmd: fmt.Sprintf(`echo 'SELECT title FROM %s WHERE title="%s"' | drush @%s --alias-path=~/.drush sql-cli --extra=-N`, t.Name(), tval, acquiaTestSite),
	})
	require.NoError(t, err)
	assert.Contains(out, tval)

	// Test that the file arrived there (by rsyncing it back)
	out, _, err = app.Exec(&ExecOpts{
		Cmd: fmt.Sprintf("drush --alias-path=~/.drush rsync -y @%s:%%files/%s /tmp && cat /tmp/%s", acquiaTestSite, fName, fName),
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
