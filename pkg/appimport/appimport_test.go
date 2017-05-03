package appimport

import (
	"fmt"
	"path"
	"testing"

	"strings"

	"os"

	"log"

	"github.com/drud/ddev/pkg/testcommon"
	"github.com/drud/ddev/pkg/util"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/assert"
)

const netName = "ddev_default"

var (
	testArchivePath = path.Join(testcommon.CreateTmpDir("appimport"), "db.tar.gz")
	cwd             string
)

func TestMain(m *testing.M) {
	testFile, err := os.Create(testArchivePath)
	if err != nil {
		log.Fatalf("failed to create test file: %v", err)
	}
	err = testFile.Close()
	if err != nil {
		log.Fatalf("failed to create test file: %v", err)
	}

	cwd, err = os.Getwd()
	if err != nil {
		log.Fatalf("failed to get cwd: %s", err)
	}

	fmt.Println("Running tests.")
	testRun := m.Run()

	err = os.RemoveAll(path.Dir(testArchivePath))
	util.CheckErr(err)

	os.Exit(testRun)
}

// TestValidateAsset tests validation of asset paths.
func TestValidateAsset(t *testing.T) {
	assert := assert.New(t)

	// test tilde expansion
	userDir, err := homedir.Dir()
	testDir := path.Join(userDir, "testpath")
	assert.NoError(err)
	err = os.Mkdir(testDir, 0755)
	assert.NoError(err)

	testPath, err := ValidateAsset("~/testpath", "files")
	assert.NoError(err)
	assert.Contains(testPath, userDir)
	assert.False(strings.Contains(testPath, "~"))
	err = os.Remove(testDir)
	assert.NoError(err)

	// test a relative path
	testPath, err = ValidateAsset("../../vendor", "files")
	assert.NoError(err)
	upTwo := strings.TrimSuffix(cwd, "/pkg/appimport")
	assert.Contains(testPath, upTwo)

	// archive
	_, err = ValidateAsset(testArchivePath, "db")
	assert.Error(err)
	assert.Equal(err.Error(), "is archive")

	// db no sql
	_, err = ValidateAsset("appimport.go", "db")
	assert.Contains(err.Error(), "provided path is not a .sql file or archive")
	assert.Error(err)

	// files not a directory
	_, err = ValidateAsset("appimport.go", "files")
	assert.Error(err)
	assert.Contains(err.Error(), "provided path is not a directory or archive")
}
