package cmd

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"

	"github.com/drud/ddev/pkg/exec"
	asrt "github.com/stretchr/testify/assert"
)

// TestCmdExecBadArgs run `ddev exec` without the proper args
func TestCmdExecBadArgs(t *testing.T) {
	// Change to the first DevTestSite for the duration of this test.
	defer TestSites[0].Chdir()()
	assert := asrt.New(t)

	args := []string{"exec"}
	out, err := exec.RunCommand(DdevBin, args)
	assert.Error(err)
	assert.Contains(string(out), "Usage:")
}

// TestCmdExec runs a number of exec commands to verify behavior
func TestCmdExec(t *testing.T) {

	assert := asrt.New(t)
	site := TestSites[0]
	cleanup := site.Chdir()
	defer cleanup()

	// Test default invocation
	args := []string{"exec", "pwd"}
	out, err := exec.RunCommand(DdevBin, args)
	assert.NoError(err)
	assert.Contains(out, "/var/www/html")

	// Test specifying service
	args = []string{"-s", "db", "exec", "pwd"}
	out, err = exec.RunCommand(DdevBin, args)
	assert.NoError(err)
	assert.Contains(out, "/")

	// Test specifying working directory
	args = []string{"exec", "-d", "/bin", "pwd"}
	out, err = exec.RunCommand(DdevBin, args)
	assert.NoError(err)
	assert.Contains(out, "/bin")

	// Test specifying service and working directory
	args = []string{"exec", "-s", "db", "-d", "/var", "pwd"}
	out, err = exec.RunCommand(DdevBin, args)
	assert.NoError(err)
	assert.Contains(out, "/var")

	// Test sudo
	args = []string{"exec", "sudo", "whoami"}
	out, err = exec.RunCommand(DdevBin, args)
	assert.NoError(err)
	assert.Contains(out, "root")

	// Test that an nonexistant working directory generates an error
	args = []string{"exec", "-d", "/does/not/exist", "pwd"}
	out, err = exec.RunCommand(DdevBin, args)
	assert.Error(err)
	assert.Contains(out, "no such file or directory")

	args = []string{"exec", "ls >/var/www/html/TestCmdExec-${OSTYPE}.txt"}
	_, err = exec.RunCommand(DdevBin, args)
	assert.NoError(err)
	assert.FileExists(filepath.Join(site.Dir, "TestCmdExec-linux-gnu.txt"))

	args = []string{"exec", "ls >/dev/null && touch /var/www/html/TestCmdExec-touch-all-in-one.txt"}
	_, err = exec.RunCommand(DdevBin, args)
	assert.NoError(err)
	assert.FileExists(filepath.Join(site.Dir, "TestCmdExec-touch-all-in-one.txt"))

	args = []string{"exec", "true", "&&", "touch", "/var/www/html/TestCmdExec-touch-separate-args.txt"}
	_, err = exec.RunCommand(DdevBin, args)
	assert.NoError(err)
	assert.FileExists(filepath.Join(site.Dir, "TestCmdExec-touch-separate-args.txt"))

	// Make sure we can pipe things into ddev exec and have them work in stdin inside container
	filename := t.Name() + "_junk.txt"
	_, err = exec.RunCommand("sh", []string{"-c", fmt.Sprintf("printf 'This file was piped into ddev exec' | %s exec 'cat >/var/www/html/%s'", DdevBin, filename)})
	assert.NoError(err)
	require.FileExists(t, filepath.Join(site.Dir, filename))

	content, err := os.ReadFile(filepath.Join(site.Dir, filename))
	assert.NoError(err)
	assert.Equal("This file was piped into ddev exec", string(content))
}
