package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/drud/ddev/pkg/exec"
	"github.com/drud/ddev/pkg/testcommon"
	"github.com/stretchr/testify/assert"
)

var override = `
version: '3.6'
services:
  web:
    labels:
      my_custom: value_here
`

// TestComposeConfigCmd ensures the compose-config command behaves
// as expected with a with a basic docker-compose.override.yaml.
func TestComposeConfigCmd(t *testing.T) {
	// Create a temporary directory and switch to it.
	tmpdir := testcommon.CreateTmpDir(t.Name())
	defer testcommon.CleanupDir(tmpdir)
	defer testcommon.Chdir(tmpdir)()

	// Create a config
	args := []string{
		"config",
		"--docroot", ".",
		"--project-name", "compose-config",
		"--project-type", "php",
	}

	_, err := exec.RunCommand(DdevBin, args)
	assert.NoError(t, err)

	//nolint: errcheck
	defer exec.RunCommand(DdevBin, []string{"remove", "-RO", "compose-config"})

	// Ensure ddev debug compose-config works as expected
	args = []string{"debug", "compose-config"}
	out, err := exec.RunCommand(DdevBin, args)
	assert.NoError(t, err)
	assert.Contains(t, out, "services")

	// Create a docker-compose.override.yaml
	overrideFile := filepath.Join(tmpdir, ".ddev", "docker-compose.override.yaml")
	err = os.WriteFile(overrideFile, []byte(override), 0644)
	assert.NoError(t, err)

	// Ensure ddev debug compose-config includes override values
	args = []string{"debug", "compose-config"}
	out, err = exec.RunCommand(DdevBin, args)
	assert.NoError(t, err)
	assert.Contains(t, out, "my_custom: value_here")
}
