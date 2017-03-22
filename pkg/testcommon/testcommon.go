package testcommon

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/drud/drud-go/utils/system"
)

// TestSite describes a site for testing, with name, URL of tarball, and optional dir.
type TestSite struct {
	// Name is the generic name of the site, and is used as the default dir.
	Name string
	// URL is the URL of the tarball to be used for building the site.
	URL string
	Dir string
}

func (site *TestSite) archivePath() string {
	return filepath.Join(os.TempDir(), site.Name+".tar.gz")
}

// Prepare downloads and extracts a site codebase to a temporary directory.
func (site *TestSite) Prepare() {
	testDir, err := ioutil.TempDir("", site.Name)
	if err != nil {
		log.Fatalf("Could not create temporary directory %s for site %s", testDir, site.Name)
	}
	site.Dir = testDir
	fmt.Printf("Prepping test for %s.", site.Name)
	os.Setenv("DRUD_NONINTERACTIVE", "true")

	system.DownloadFile(site.archivePath(), site.URL)
	system.RunCommand("tar",
		[]string{
			"-xzf",
			site.archivePath(),
			"--strip", "1",
			"-C",
			site.Dir,
		})
}

// Chdir will change to the directory for the site specified by TestSite.
func (site *TestSite) Chdir() func() {
	return Chdir(site.Dir)
}

// Cleanup removes the archive and codebase extraction for a site after a test run has completed.
func (site *TestSite) Cleanup() {
	os.Remove(site.archivePath())
	os.RemoveAll(site.Dir)
}

// CreateTmpDir creates a temporary directory and returns its path as a string.
func CreateTmpDir() string {
	testDir, err := ioutil.TempDir("", "")
	if err != nil {
		log.Fatalf("Could not create temporary directory %s: %v", testDir, err)
	}

	return testDir
}

// Chdir will change to the directory for the site specified by TestSite.
// It returns an anonymous function which will return to the original working directory when called.
func Chdir(path string) func() {
	curDir, _ := os.Getwd()
	err := os.Chdir(path)
	if err != nil {
		log.Fatalf("Could not change to directory %s: %v\n", path, err)
	}

	return func() { os.Chdir(curDir) }
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// RandString returns a random string of given length n.
func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
