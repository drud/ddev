package fileutil_test

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/testcommon"
	asrt "github.com/stretchr/testify/assert"
)

var testFileLocation = "testdata/regular_file"

// TestCopyDir tests copying a directory.
func TestCopyDir(t *testing.T) {
	assert := asrt.New(t)
	sourceDir := testcommon.CreateTmpDir("TestCopyDir_source")
	targetDir := testcommon.CreateTmpDir("TestCopyDir_target")

	subdir := filepath.Join(sourceDir, "some_content")
	err := os.Mkdir(subdir, 0755)
	assert.NoError(err)

	// test source not a directory
	err = fileutil.CopyDir(testFileLocation, sourceDir)
	assert.Error(err)
	if err != nil {
		assert.Contains(err.Error(), fmt.Sprintf("CopyDir: source directory %s is not a directory", filepath.Join(testFileLocation)))
	}

	// test destination exists
	err = fileutil.CopyDir(sourceDir, targetDir)
	assert.Error(err)
	if err != nil {
		assert.Contains(err.Error(), fmt.Sprintf("CopyDir: destination %s already exists", filepath.Join(targetDir)))
	}
	err = os.RemoveAll(subdir)
	assert.NoError(err)

	// copy a directory and validate that we find files elsewhere
	err = os.RemoveAll(targetDir)
	assert.NoError(err)

	file, err := os.Create(filepath.Join(sourceDir, "touch1.txt"))
	assert.NoError(err)
	_ = file.Close()
	file, err = os.Create(filepath.Join(sourceDir, "touch2.txt"))
	assert.NoError(err)
	_ = file.Close()

	err = fileutil.CopyDir(sourceDir, targetDir)
	assert.NoError(err)
	assert.True(fileutil.FileExists(filepath.Join(targetDir, "touch1.txt")))
	assert.True(fileutil.FileExists(filepath.Join(targetDir, "touch2.txt")))

	err = os.RemoveAll(sourceDir)
	assert.NoError(err)
	err = os.RemoveAll(targetDir)
	assert.NoError(err)

}

// TestCopyFile tests copying a file.
func TestCopyFile(t *testing.T) {
	assert := asrt.New(t)
	tmpTargetDir := testcommon.CreateTmpDir("TestCopyFile")
	tmpTargetFile := filepath.Join(tmpTargetDir, filepath.Base(testFileLocation))

	err := fileutil.CopyFile(testFileLocation, tmpTargetFile)
	assert.NoError(err)

	file, err := os.Stat(tmpTargetFile)
	assert.NoError(err)

	if err != nil {
		assert.False(file.IsDir())
	}
	err = os.RemoveAll(tmpTargetDir)
	assert.NoError(err)
}

// TestPurgeDirectory tests removal of directory contents without removing
// the directory itself.
func TestPurgeDirectory(t *testing.T) {
	assert := asrt.New(t)
	tmpPurgeDir := testcommon.CreateTmpDir("TestPurgeDirectory")
	tmpPurgeFile := filepath.Join(tmpPurgeDir, "regular_file")
	tmpPurgeSubFile := filepath.Join(tmpPurgeDir, "subdir", "regular_file")

	err := fileutil.CopyFile(testFileLocation, tmpPurgeFile)
	assert.NoError(err)

	err = os.Mkdir(filepath.Join(tmpPurgeDir, "subdir"), 0755)
	assert.NoError(err)

	err = fileutil.CopyFile(testFileLocation, tmpPurgeSubFile)
	assert.NoError(err)

	err = os.Chmod(tmpPurgeSubFile, 0444)
	assert.NoError(err)

	err = fileutil.PurgeDirectory(tmpPurgeDir)
	assert.NoError(err)

	assert.True(fileutil.FileExists(tmpPurgeDir))
	assert.False(fileutil.FileExists(tmpPurgeFile))
	assert.False(fileutil.FileExists(tmpPurgeSubFile))
}

// TestFgrepStringInFile tests the FgrepStringInFile utility function.
func TestFgrepStringInFile(t *testing.T) {
	assert := asrt.New(t)
	result, err := fileutil.FgrepStringInFile("testdata/fgrep_has_positive_contents.txt", "some needle we're looking for")
	assert.NoError(err)
	assert.True(result)
	result, err = fileutil.FgrepStringInFile("testdata/fgrep_negative_contents.txt", "some needle we're looking for")
	assert.NoError(err)
	assert.False(result)
}

// TestListFilesInDir makes sure the files we have in testfiles are properly enumerated
func TestListFilesInDir(t *testing.T) {
	assert := asrt.New(t)

	fileList, err := fileutil.ListFilesInDir("testdata/testfiles/")
	assert.NoError(err)
	assert.True(len(fileList) == 2)
	assert.Contains(fileList[0], "one.txt")
	assert.Contains(fileList[1], "two.txt")
}

// TestReplaceStringInFile tests the ReplaceStringInFile utility function.
func TestReplaceStringInFile(t *testing.T) {
	assert := asrt.New(t)
	tmp, err := os.MkdirTemp("", "")
	assert.NoError(err)
	newFilePath := filepath.Join(tmp, "newfile.txt")
	err = fileutil.ReplaceStringInFile("some needle we're looking for", "specialJUNKPattern", "testdata/fgrep_has_positive_contents.txt", newFilePath)
	assert.NoError(err)
	found, err := fileutil.FgrepStringInFile(newFilePath, "specialJUNKPattern")
	assert.NoError(err)
	assert.True(found)
}

// TestFindSimulatedXsymSymlinks tests FindSimulatedXsymSymlinks
func TestFindSimulatedXsymSymlinks(t *testing.T) {
	assert := asrt.New(t)
	testDir, _ := os.Getwd()
	targetDir := filepath.Join(testDir, "testdata", "symlinks")
	links, err := fileutil.FindSimulatedXsymSymlinks(targetDir)
	assert.NoError(err)
	assert.Len(links, 8)
}

// TestReplaceSimulatedXsymSymlinks tries a number of symlinks to make
// sure we can parse and replace symlinks.
func TestReplaceSimulatedXsymSymlinks(t *testing.T) {
	testDir, _ := os.Getwd()
	assert := asrt.New(t)
	if runtime.GOOS == "windows" && !fileutil.CanCreateSymlinks() {
		t.Skip("Skipping on Windows because test machine can't create symlnks")
	}
	sourceDir := filepath.Join(testDir, "testdata", "symlinks")
	targetDir := testcommon.CreateTmpDir("TestReplaceSimulated")
	//nolint: errcheck
	defer os.RemoveAll(targetDir)
	err := os.Chdir(targetDir)
	assert.NoError(err)

	// Make sure we leave the testDir as we found it..
	//nolint: errcheck
	defer os.Chdir(testDir)
	// CopyDir skips real symlinks, but we only care about simulated ones, so it's OK
	err = fileutil.CopyDir(sourceDir, filepath.Join(targetDir, "symlinks"))
	assert.NoError(err)
	links, err := fileutil.FindSimulatedXsymSymlinks(targetDir)
	assert.NoError(err)
	assert.Len(links, 8)
	err = fileutil.ReplaceSimulatedXsymSymlinks(links)
	assert.NoError(err)

	for _, link := range links {
		fi, err := os.Stat(link.LinkLocation)
		assert.NoError(err)
		linkFi, err := os.Lstat(link.LinkLocation)
		assert.NoError(err)
		if err == nil && fi != nil && !fi.IsDir() {
			// Read the symlink as a file. It should resolve with the actual content of target
			contents, err := os.ReadFile(link.LinkLocation)
			assert.NoError(err)
			expectedContent := "textfile " + filepath.Base(link.LinkTarget) + "\n"
			assert.Equal(expectedContent, string(contents))
		}
		// Now stat the link and make sure it's a link and points where it should
		if linkFi.Mode()&os.ModeSymlink != 0 {
			targetFile, err := os.Readlink(link.LinkLocation)
			assert.NoError(err)
			assert.Equal(link.LinkTarget, targetFile)
			_ = targetFile
		}
	}

}

// TestIsSameFile tests the IsSameFile utility function.
func TestIsSameFile(t *testing.T) {
	assert := asrt.New(t)

	tmpDir, err := testcommon.OsTempDir()
	assert.NoError(err)

	dirSymlink, err := filepath.Abs(filepath.Join(tmpDir, fileutil.RandomFilenameBase()))
	require.NoError(t, err)
	testdataAbsolute, err := filepath.Abs("testdata")
	require.NoError(t, err)
	err = os.Symlink(testdataAbsolute, dirSymlink)
	assert.NoError(err)
	//nolint: errcheck
	defer os.Remove(dirSymlink)

	// At this point, dirSymLink and "testdata" should be equivalent
	isSame, err := fileutil.IsSameFile("testdata", dirSymlink)
	assert.NoError(err)
	assert.True(isSame)
	// Test with files that are equivalent (through symlink)
	isSame, err = fileutil.IsSameFile("testdata/testfiles/one.txt", filepath.Join(dirSymlink, "testfiles", "one.txt"))
	assert.NoError(err)
	assert.True(isSame)

	// Test files that are *not* equivalent.
	isSame, err = fileutil.IsSameFile("testdata/testfiles/one.txt", "testdata/testfiles/two.txt")
	assert.NoError(err)
	assert.False(isSame)
}
