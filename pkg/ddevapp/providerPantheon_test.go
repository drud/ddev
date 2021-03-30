package ddevapp_test

import (
	"fmt"
	"github.com/drud/ddev/pkg/exec"
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/globalconfig"
	"github.com/drud/ddev/pkg/nodeps"
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
 * need to set an environment variable called "DDEV_PANTHEON_API_TOKEN" with credentials for
 * this account. If no such environment variable is present, these tests will be skipped.
 *
 */

const pantheonTestSiteID = "ddev-test-site-do-not-delete.dev"

// TestPantheonPull ensures we can pull from pantheon.
func TestPantheonPull(t *testing.T) {
	token := ""
	sshkey := ""
	if token = os.Getenv("DDEV_PANTHEON_API_TOKEN"); token == "" {
		t.Skipf("No DDEV_PANTHEON_API_TOKEN env var has been set. Skipping %v", t.Name())
	}
	if sshkey = os.Getenv("DDEV_PANTHEON_SSH_KEY"); sshkey == "" {
		t.Skipf("No DDEV_PANTHEON_SSH_KEY env var has been set. Skipping %v", t.Name())
	}
	sshkey = strings.Replace(sshkey, "<SPLIT>", "\n", -1)

	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)
	origDir, _ := os.Getwd()

	webEnvSave := globalconfig.DdevGlobalConfig.WebEnvironment
	globalconfig.DdevGlobalConfig.WebEnvironment = []string{"TERMINUS_MACHINE_TOKEN=" + token}
	err := globalconfig.WriteGlobalConfig(globalconfig.DdevGlobalConfig)
	assert.NoError(err)

	siteDir := testcommon.CreateTmpDir(t.Name())
	err = os.MkdirAll(filepath.Join(siteDir, "sites/default"), 0777)
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
	app.Type = nodeps.AppTypeDrupal8
	app.Hooks = map[string][]YAMLTask{"post-pull": {{"exec-host": "touch hello-post-pull-" + app.Name}}, "pre-pull": {{"exec-host": "touch hello-pre-pull-" + app.Name}}}

	_ = app.Stop(true, false)

	err = app.WriteConfig()
	assert.NoError(err)

	testcommon.ClearDockerEnv()

	// Run ddev once to create all the files in .ddev, including the example
	_, err = exec.RunCommand("bash", []string{"-c", fmt.Sprintf("%s >/dev/null", DdevBin)})
	require.NoError(t, err)

	// Build our pantheon.yaml from the example file
	s, err := os.ReadFile(app.GetConfigPath("providers/pantheon.yaml.example"))
	require.NoError(t, err)
	x := strings.Replace(string(s), "project:", fmt.Sprintf("project: %s\n#project:", pantheonTestSiteID), 1)
	err = os.WriteFile(app.GetConfigPath("providers/pantheon.yaml"), []byte(x), 0666)
	assert.NoError(err)
	err = app.WriteConfig()
	require.NoError(t, err)

	provider, err := app.GetProvider("pantheon")
	require.NoError(t, err)
	err = app.Start()
	require.NoError(t, err)

	_, err = exec.RunCommand("bash", []string{"-c", fmt.Sprintf("%s composer require drush/drush >/dev/null 2>&1", DdevBin)})
	require.NoError(t, err)

	err = app.Pull(provider, false, false, false)
	require.NoError(t, err)

	assert.FileExists(filepath.Join(app.GetUploadDir(), "2017-07/22-24_tn.jpg"))
	out, err := exec.RunCommand("bash", []string{"-c", fmt.Sprintf(`echo 'select COUNT(*) from users_field_data where mail="admin@example.com";' | %s mysql -N`, DdevBin)})
	assert.NoError(err)
	assert.True(strings.HasPrefix(out, "1\n"))

	assert.FileExists("hello-pre-pull-" + app.Name)
	assert.FileExists("hello-post-pull-" + app.Name)
	err = os.Remove("hello-pre-pull-" + app.Name)
	assert.NoError(err)
	err = os.Remove("hello-post-pull-" + app.Name)
	assert.NoError(err)
}

// TestPantheonPush ensures we can push to pantheon for a configured environment.
func TestPantheonPush(t *testing.T) {
	token := ""
	sshkey := ""
	if token = os.Getenv("DDEV_PANTHEON_API_TOKEN"); token == "" {
		t.Skipf("No DDEV_PANTHEON_API_TOKEN env var has been set. Skipping %v", t.Name())
	}
	if sshkey = os.Getenv("DDEV_PANTHEON_SSH_KEY"); sshkey == "" {
		t.Skipf("No DDEV_PANTHEON_SSH_KEY env var has been set. Skipping %v", t.Name())
	}
	sshkey = strings.Replace(sshkey, "<SPLIT>", "\n", -1)

	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)
	origDir, _ := os.Getwd()

	webEnvSave := globalconfig.DdevGlobalConfig.WebEnvironment
	globalconfig.DdevGlobalConfig.WebEnvironment = []string{"TERMINUS_MACHINE_TOKEN=" + token}
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
	app.Type = nodeps.AppTypeDrupal8
	app.Hooks = map[string][]YAMLTask{"post-push": {{"exec-host": "touch hello-post-push-" + app.Name}}, "pre-push": {{"exec-host": "touch hello-pre-push-" + app.Name}}}
	_ = app.Stop(true, false)

	err = app.WriteConfig()
	require.NoError(t, err)

	testcommon.ClearDockerEnv()

	// Run ddev once to create all the files in .ddev, including the example
	_, err = exec.RunCommand("bash", []string{"-c", fmt.Sprintf("%s >/dev/null", DdevBin)})
	require.NoError(t, err)

	// Build our pantheon.yaml from the example file
	s, err := os.ReadFile(app.GetConfigPath("providers/pantheon.yaml.example"))
	require.NoError(t, err)
	x := strings.Replace(string(s), "project:", fmt.Sprintf("project: %s\n#project:", pantheonTestSiteID), 1)
	err = os.WriteFile(app.GetConfigPath("providers/pantheon.yaml"), []byte(x), 0666)
	assert.NoError(err)
	err = app.WriteConfig()
	require.NoError(t, err)

	provider, err := app.GetProvider("pantheon")
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
	err = os.WriteFile(filepath.Join(siteDir, "sites/default/files", fName), fContent, 0644)
	assert.NoError(err)

	err = app.Push(provider, false, false)
	require.NoError(t, err)

	// Test that the database row was added
	out, _, err := app.Exec(&ExecOpts{
		Cmd: fmt.Sprintf(`echo 'SELECT title FROM %s WHERE title="%s"' | drush @%s sql-cli --extra=-N`, t.Name(), tval, pantheonTestSiteID),
	})
	require.NoError(t, err)
	assert.Contains(out, tval)

	// Test that the file arrived there (by rsyncing it back)
	out, _, err = app.Exec(&ExecOpts{
		Cmd: fmt.Sprintf("drush rsync -y @%s:%%files/%s /tmp && cat /tmp/%s", pantheonTestSiteID, fName, fName),
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

// setupSSHKey takes a privatekey string and turns it into a file and then does `ddev auth ssh`
func setupSSHKey(t *testing.T, privateKey string, expectScriptDir string) error {
	// Provide an ssh key for `ddev auth ssh`
	err := os.Mkdir("sshtest", 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join("sshtest", "id_rsa_test"), []byte(privateKey), 0600)
	require.NoError(t, err)
	out, err := exec.RunCommand("expect", []string{filepath.Join(expectScriptDir, "ddevauthssh.expect"), DdevBin, "./sshtest"})
	require.NoError(t, err)
	require.Contains(t, string(out), "Identity added:")
	return nil
}
