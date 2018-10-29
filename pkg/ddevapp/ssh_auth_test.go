package ddevapp_test

import (
	"fmt"
	"github.com/drud/ddev/pkg/ddevapp"
	"github.com/drud/ddev/pkg/exec"
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/testcommon"
	"github.com/drud/ddev/pkg/util"
	"github.com/drud/ddev/pkg/version"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	asrt "github.com/stretchr/testify/assert"
)

// TestSSHAuth tests basic ssh authentication
func TestSSHAuth(t *testing.T) {
	assert := asrt.New(t)
	testDir, _ := os.Getwd()
	app := &ddevapp.DdevApp{}

	runTime := testcommon.TimeTrack(time.Now(), fmt.Sprintf("TestDdevRestoreSnapshot"))

	//  Add a docker-compose service that has ssh server and mounted authorized_keys
	// Use d7 only for this test, the key thing is the database interaction
	site := FullTestSites[0]
	// If running this with GOTEST_SHORT we have to create the directory, tarball etc.
	if site.Dir == "" || !fileutil.FileExists(site.Dir) {
		err := site.Prepare()
		if err != nil {
			t.Fatalf("Prepare() failed on TestSite.Prepare() site=%s, err=%v", site.Name, err)
		}
	}

	switchDir := site.Chdir()
	testcommon.ClearDockerEnv()

	err := app.Init(site.Dir)
	if err != nil {
		if app.SiteStatus() != ddevapp.SiteRunning {
			t.Fatalf("app.Init() failed on site %s in dir %s, err=%v", site.Name, site.Dir, err)
		}
	}
	destDdev := filepath.Join(app.AppRoot, ".ddev")
	srcDdev := filepath.Join(testDir, "testdata", "TestSSHAuth", ".ddev")
	err = fileutil.CopyDir(filepath.Join(srcDdev, ".ssh"), filepath.Join(destDdev, ".ssh"))
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(destDdev, ".ssh"), 0700)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(destDdev, ".ssh", "authorized_keys"), 0600)
	require.NoError(t, err)
	err = fileutil.CopyFile(filepath.Join(srcDdev, "docker-compose.sshserver.yaml"), filepath.Join(destDdev, "docker-compose.sshserver.yaml"))
	require.NoError(t, err)

	//nolint: errcheck
	defer fileutil.PurgeDirectory(filepath.Join(destDdev, ".ssh"))
	//nolint: errcheck
	defer os.Remove(filepath.Join(destDdev, "docker-compose.sshserver.yaml"))

	// Make absolutely sure the ssh-agent is created from scratch.
	err = ddevapp.RemoveSSHAgentContainer()
	require.NoError(t, err)

	err = app.Start()
	if err != nil {
		t.Fatalf("TestMain startup: app.Start() failed on site %s, err=%v", site.Name, err)
	}

	err = ddevapp.EnsureSSHAgentContainer()
	require.NoError(t, err)

	// Try a simple ssh (with no auth set up), it should fail with "Permission denied"
	_, stderr, err := app.Exec("web", "bash", "-c", "ssh -o BatchMode=yes -o StrictHostKeyChecking=false root@test-ssh-server pwd")
	assert.Error(err)
	assert.Contains(stderr, "Permission denied")

	// Add password/key to auth. This is an unfortunate perversion of using docker run directly, copied from
	// ddev auth ssh command, and with an expect script to provide the passphrase.
	_, _, uidStr, _ := util.GetContainerUIDGid()
	err = exec.RunInteractiveCommand("docker", []string{"run", "-t", "--rm", "--volumes-from=" + ddevapp.SSHAuthName, "-v", filepath.Join(destDdev, ".ssh") + ":/tmp/.ssh", "-u", uidStr, version.SSHAuthImage + ":" + version.SSHAuthTag, "//test.expect.passphrase"})
	require.NoError(t, err)

	// Try ssh, should succeed
	stdout, _, err := app.Exec("web", "bash", "-c", "ssh -o StrictHostKeyChecking=false root@test-ssh-server pwd")
	stdout = strings.Trim(stdout, "\n")
	assert.Equal(stdout, "/root")
	assert.NoError(err)
	err = app.Down(true, false)
	assert.NoError(err)

	runTime()
	switchDir()
}
