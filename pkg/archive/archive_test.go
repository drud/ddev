package archive_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/drud/ddev/pkg/archive"
	"github.com/drud/ddev/pkg/testcommon"
	asrt "github.com/stretchr/testify/assert"
)

// TestUnarchive tests unzip/tar/tar.gz/tgz functionality, including the starting extraction-skip directory
func TestUnarchive(t *testing.T) {

	// testUnarchiveDir is the directory we may want to use to start extracting.
	testUnarchiveDir := "dir2/"

	assert := asrt.New(t)

	for _, suffix := range []string{"zip", "tar", "tar.gz", "tgz"} {
		source := filepath.Join("testdata", t.Name(), "testfile"+"."+suffix)
		exDir := testcommon.CreateTmpDir("testfile" + suffix)

		// default function to untar
		unarchiveFunc := archive.Untar
		if suffix == "zip" {
			unarchiveFunc = archive.Unzip
		}

		err := unarchiveFunc(source, exDir, "")
		assert.NoError(err)

		// Make sure that our base extraction directory is there
		finfo, err := os.Stat(filepath.Join(exDir, testUnarchiveDir))
		assert.NoError(err)
		assert.True(err == nil && finfo.IsDir())
		finfo, err = os.Stat(filepath.Join(exDir, testUnarchiveDir, "dir2_file.txt"))
		assert.NoError(err)
		assert.True(err == nil && !finfo.IsDir())

		err = os.RemoveAll(exDir)
		assert.NoError(err)

		// Now do the unarchive with an extraction root
		exDir = testcommon.CreateTmpDir("testfile" + suffix + "2")

		err = unarchiveFunc(source, exDir, testUnarchiveDir)
		assert.NoError(err)

		// Only the dir2_file should remain
		finfo, err = os.Stat(filepath.Join(exDir, "dir2_file.txt"))
		assert.NoError(err)
		assert.True(err == nil && !finfo.IsDir())

		err = os.RemoveAll(exDir)
		assert.NoError(err)
	}
}

// TestArchiveTar tests creation of a simple tarball
func TestArchiveTar(t *testing.T) {
	assert := asrt.New(t)
	pwd, _ := os.Getwd()
	tarballFile, err := os.CreateTemp("", t.Name())
	assert.NoError(err)

	err = archive.Tar(filepath.Join(pwd, "testdata", t.Name()), tarballFile.Name())
	assert.NoError(err)

	tmpDir := testcommon.CreateTmpDir(t.Name())

	t.Cleanup(
		func() {
			// Could not figure out what causes this not to be removable
			//err = os.Remove(tarballFile.Name())
			//assert.NoError(err)
			err = os.RemoveAll(tmpDir)
			assert.NoError(err)
		})
	err = archive.Untar(tarballFile.Name(), tmpDir, "")
	assert.NoError(err)

	assert.FileExists(filepath.Join(tmpDir, "root.txt"))
	assert.FileExists(filepath.Join(tmpDir, "subdir1/subdir1.txt"))
}
