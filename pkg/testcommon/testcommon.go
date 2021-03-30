package testcommon

import (
	"crypto/tls"
	"github.com/docker/docker/pkg/homedir"
	"github.com/drud/ddev/pkg/ddevapp"
	"github.com/drud/ddev/pkg/globalconfig"
	"github.com/drud/ddev/pkg/output"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"

	"path"

	"fmt"

	"github.com/drud/ddev/pkg/archive"
	"github.com/drud/ddev/pkg/dockerutil"
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/util"
	"github.com/pkg/errors"
	asrt "github.com/stretchr/testify/assert"
	"net/http"
	"net/url"
	"testing"
)

// URIWithExpect pairs a URI like "/readme.html" with some substring content "should be found in URI"
type URIWithExpect struct {
	URI    string
	Expect string
}

// TestSite describes a site for testing, with name, URL of tarball, and optional dir.
type TestSite struct {
	// Name is the generic name of the site, and is used as the default dir.
	Name string
	// SourceURL is the URL of the source code tarball to be used for building the site.
	SourceURL string
	// ArchiveExtractionPath is the relative path within the tarball which should be extracted, ending with /
	ArchiveInternalExtractionPath string
	// FullSiteTarballURL is the URL of the tarball of a full site archive used for testing import.
	FullSiteTarballURL string
	// FilesTarballURL is the URL of the tarball of file uploads used for testing file import.
	FilesTarballURL string
	// FilesZipballURL is the URL of the zipball of file uploads used for testing file import.
	FilesZipballURL string
	// DBTarURL is the URL of the database dump tarball used for testing database import.
	DBTarURL string
	// DBZipURL is the URL of an optional zip-style db dump.
	DBZipURL string
	// Dir is the rooted full path of the test site
	Dir string
	// HTTPProbeURI is the URI that can be probed to look for a working web container
	HTTPProbeURI string
	// Docroot is the subdirectory witin the site that is the root/index.php
	Docroot string
	// Type is the type of application. This can be specified when a config file is not present
	// for a test site.
	Type string
	// Safe200URIWithExpectation provides a static URI with contents that it can be expected to contain.
	Safe200URIWithExpectation URIWithExpect
	// DynamicURI provides a dynamic (after db load) URI with contents we can expect.
	DynamicURI URIWithExpect
	// FilesImageURI is URI to a file loaded by import-files that is a jpg.
	FilesImageURI string
	// FullSiteArchiveExtPath is the path that should be extracted from inside an archive when
	// importing the files from a full site archive
	FullSiteArchiveExtPath string
}

// Prepare downloads and extracts a site codebase to a temporary directory.
func (site *TestSite) Prepare() error {
	testDir := CreateTmpDir(site.Name)
	site.Dir = testDir

	err := os.Setenv("DDEV_NONINTERACTIVE", "true")
	util.CheckErr(err)

	cachedSrcDir, _, err := GetCachedArchive(site.Name, site.Name+"_siteArchive", site.ArchiveInternalExtractionPath, site.SourceURL)

	if err != nil {
		site.Cleanup()
		return fmt.Errorf("Failed to GetCachedArchive, err=%v", err)
	}
	// We must copy into a directory that does not yet exist :(
	err = os.Remove(site.Dir)
	util.CheckErr(err)

	output.UserOut.Printf("Copying directory %s to %s\n", cachedSrcDir, site.Dir)
	if runtime.GOOS != "windows" {
		// Simple cp -r is far, far faster than our fileutil.CopyDir
		cmd := exec.Command("bash", "-c", fmt.Sprintf(`cp -rp %s %s`, cachedSrcDir, site.Dir))
		err = cmd.Run()
	} else {
		err = fileutil.CopyDir(cachedSrcDir, site.Dir)
	}
	if err != nil {
		site.Cleanup()
		return fmt.Errorf("Failed to CopyDir from %s to %s, err=%v", cachedSrcDir, site.Dir, err)
	}
	output.UserOut.Println("Copying complete")

	// Create an app. Err is ignored as we may not have
	// a config file to read in from a test site.
	app, err := ddevapp.NewApp(site.Dir, true)
	if err != nil {
		return err
	}
	// Set app name to the name we define for test sites. We'll
	// ignore app name defined in config file if present.
	app.Name = site.Name
	app.Docroot = site.Docroot
	app.Type = app.DetectAppType()
	if app.Type != site.Type {
		return errors.Errorf("Detected apptype (%s) does not match provided apptype (%s)", app.Type, site.Type)
	}

	err = app.ConfigFileOverrideAction()
	util.CheckErr(err)

	err = app.WriteConfig()
	if err != nil {
		return errors.Errorf("Failed to write site config for site %s, dir %s, err: %v", app.Name, app.GetAppRoot(), err)
	}

	return nil
}

// Chdir will change to the directory for the site specified by TestSite.
func (site *TestSite) Chdir() func() {
	return Chdir(site.Dir)
}

// Cleanup removes the archive and codebase extraction for a site after a test run has completed.
func (site *TestSite) Cleanup() {
	// CleanupDir checks its own errors.
	CleanupDir(site.Dir)

	_ = globalconfig.RemoveProjectInfo(site.Name)
	siteData := filepath.Join(globalconfig.GetGlobalDdevDir(), site.Name)
	if fileutil.FileExists(siteData) {
		CleanupDir(siteData)
	}
}

// CleanupDir removes a directory specified by string.
func CleanupDir(dir string) {
	err := os.RemoveAll(dir)
	if err != nil {
		log.Warn(fmt.Sprintf("Failed to remove directory %s, err: %v", dir, err))
	}
}

// OsTempDir gets os.TempDir() (usually provided by $TMPDIR) but expands any symlinks found within it.
// This wrapper function can prevent problems with docker-for-mac trying to use /var/..., which is not typically
// shared/mounted. It will be expanded via the /var symlink to /private/var/...
func OsTempDir() (string, error) {
	dirName := os.TempDir()
	tmpDir, err := filepath.EvalSymlinks(dirName)
	if err != nil {
		return "", err
	}
	tmpDir = filepath.Clean(tmpDir)
	return tmpDir, nil
}

// CreateTmpDir creates a temporary directory and returns its path as a string.
func CreateTmpDir(prefix string) string {
	baseTmpDir := filepath.Join(homedir.Get(), "tmp", "ddevtest")
	_ = os.MkdirAll(baseTmpDir, 0755)
	fullPath, err := os.MkdirTemp(baseTmpDir, prefix)
	if err != nil {
		log.Fatalf("Failed to create temp directory %s, err=%v", fullPath, err)
	}
	// Make the tmpdir fully writeable/readable, NFS problems
	_ = os.Chmod(fullPath, 0777)
	return fullPath
}

// Chdir will change to the directory for the site specified by TestSite.
// It returns an anonymous function which will return to the original working directory when called.
func Chdir(path string) func() {
	curDir, _ := os.Getwd()
	err := os.Chdir(path)
	if err != nil {
		log.Errorf("Could not change to directory %s: %v\n", path, err)
	}

	return func() {
		err := os.Chdir(curDir)
		if err != nil {
			log.Errorf("Failed to change directory to original dir=%s, err=%v", curDir, err)
		}
	}
}

// ClearDockerEnv unsets env vars set in platform DockerEnv() so that
// they can be set by another test run.
func ClearDockerEnv() {
	envVars := []string{
		"COMPOSE_PROJECT_NAME",
		"COMPOSE_CONVERT_WINDOWS_PATHS",
		"DDEV_SITENAME",
		"DDEV_DBIMAGE",
		"DDEV_WEBIMAGE",
		"DDEV_APPROOT",
		"DDEV_HOST_WEBSERVER_PORT",
		"DDEV_HOST_HTTPS_PORT",
		"DDEV_DOCROOT",
		"DDEV_HOSTNAME",
		"DDEV_PHP_VERSION",
		"DDEV_WEBSERVER_TYPE",
		"DDEV_PROJECT_TYPE",
		"DDEV_ROUTER_HTTP_PORT",
		"DDEV_ROUTER_HTTPS_PORT",
		"DDEV_HOST_DB_PORT",
		"DDEV_HOST_WEBSERVER_PORT",
		"DDEV_PHPMYADMIN_PORT",
		"DDEV_PHPMYADMIN_HTTPS_PORT",
		"DDEV_MAILHOG_PORT",
		"COLUMNS",
		"LINES",
		"DDEV_XDEBUG_ENABLED",
		"IS_DDEV_PROJECT",
	}
	for _, env := range envVars {
		err := os.Unsetenv(env)
		if err != nil {
			log.Printf("failed to unset %s: %v\n", env, err)
		}
	}
}

// ContainerCheck determines if a given container name exists and matches a given state
func ContainerCheck(checkName string, checkState string) (bool, error) {
	// ensure we have docker network
	client := dockerutil.GetDockerClient()
	err := dockerutil.EnsureNetwork(client, dockerutil.NetName)
	if err != nil {
		log.Fatal(err)
	}

	containers, err := dockerutil.GetDockerContainers(true)
	if err != nil {
		log.Fatal(err)
	}

	for _, container := range containers {
		name := dockerutil.ContainerName(container)
		if name == checkName {
			if container.State == checkState {
				return true, nil
			}
			return false, errors.New("container " + name + " returned " + container.State)
		}
	}

	return false, errors.New("unable to find container " + checkName)
}

// GetCachedArchive returns a directory populated with the contents of the specified archive, either from cache or
// from downloading and creating cache.
// siteName is the site.Name used for storage
// prefixString is the prefix used to disambiguate downloads and extracts
// internalExtractionPath is the place in the archive to start extracting
// sourceURL is the actual URL to download.
// Returns the extracted path, the tarball path (both possibly cached), and an error value.
func GetCachedArchive(siteName string, prefixString string, internalExtractionPath string, sourceURL string) (string, string, error) {
	uniqueName := prefixString + "_" + path.Base(sourceURL)
	testCache := filepath.Join(globalconfig.GetGlobalDdevDir(), "testcache", siteName)
	archiveFullPath := filepath.Join(testCache, "tarballs", uniqueName)
	_ = os.MkdirAll(filepath.Dir(archiveFullPath), 0777)
	extractPath := filepath.Join(testCache, prefixString)

	// Check to see if we have it cached, if so just return it.
	dStat, dErr := os.Stat(extractPath)
	aStat, aErr := os.Stat(archiveFullPath)
	if dErr == nil && dStat.IsDir() && aErr == nil && !aStat.IsDir() {
		return extractPath, archiveFullPath, nil
	}

	output.UserOut.Printf("Downloading %s", archiveFullPath)
	_ = os.MkdirAll(extractPath, 0777)
	err := util.DownloadFile(archiveFullPath, sourceURL, false)
	if err != nil {
		return extractPath, archiveFullPath, fmt.Errorf("Failed to download url=%s into %s, err=%v", sourceURL, archiveFullPath, err)
	}

	output.UserOut.Printf("Downloaded %s into %s", sourceURL, archiveFullPath)

	if filepath.Ext(archiveFullPath) == ".zip" {
		err = archive.Unzip(archiveFullPath, extractPath, internalExtractionPath)
	} else {
		err = archive.Untar(archiveFullPath, extractPath, internalExtractionPath)
	}
	if err != nil {
		_ = fileutil.PurgeDirectory(extractPath)
		_ = os.RemoveAll(extractPath)
		_ = os.RemoveAll(archiveFullPath)
		return extractPath, archiveFullPath, fmt.Errorf("archive extraction of %s failed err=%v", archiveFullPath, err)
	}
	return extractPath, archiveFullPath, nil
}

// GetLocalHTTPResponse takes a URL and optional timeout in seconds,
// hits the local docker for it, returns result
// Returns error (with the body) if not 200 status code.
func GetLocalHTTPResponse(t *testing.T, rawurl string, timeoutSecsAry ...int) (string, *http.Response, error) {
	var timeoutSecs = 30
	if len(timeoutSecsAry) > 0 {
		timeoutSecs = timeoutSecsAry[0]
	}
	timeoutTime := time.Duration(timeoutSecs) * time.Second
	assert := asrt.New(t)

	u, err := url.Parse(rawurl)
	if err != nil {
		t.Fatalf("Failed to parse url %s: %v", rawurl, err)
	}
	port := u.Port()

	dockerIP, err := dockerutil.GetDockerIP()
	assert.NoError(err)

	fakeHost := u.Hostname()
	// Add the port if there is one.
	u.Host = dockerIP
	if port != "" {
		u.Host = u.Host + ":" + port
	}
	localAddress := u.String()

	// use ServerName: fakeHost to verify basic usage of certificate.
	// This technique is from https://stackoverflow.com/a/47169975/215713
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{ServerName: fakeHost},
	}

	// Do not follow redirects, https://stackoverflow.com/a/38150816/215713
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: transport,
		Timeout:   timeoutTime,
	}

	req, err := http.NewRequest("GET", localAddress, nil)

	if err != nil {
		return "", nil, fmt.Errorf("Failed to NewRequest GET %s: %v", localAddress, err)
	}
	req.Host = fakeHost

	resp, err := client.Do(req)
	if err != nil {
		return "", resp, err
	}

	//nolint: errcheck
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp, fmt.Errorf("unable to ReadAll resp.body: %v", err)
	}
	bodyString := string(bodyBytes)
	if resp.StatusCode != 200 {
		return bodyString, resp, fmt.Errorf("http status code was %d, not 200", resp.StatusCode)
	}
	return bodyString, resp, nil
}

// EnsureLocalHTTPContent will verify a URL responds with a 200 and expected content string
func EnsureLocalHTTPContent(t *testing.T, rawurl string, expectedContent string, timeoutSeconds ...int) (*http.Response, error) {
	var httpTimeout = 30
	if len(timeoutSeconds) > 0 {
		httpTimeout = timeoutSeconds[0]
	}
	assert := asrt.New(t)

	body, resp, err := GetLocalHTTPResponse(t, rawurl, httpTimeout)
	// We see intermittent php-fpm SIGBUS failures, only on macOS.
	// That results in a 502/503. If we get a 502/503 on macOS, try again.
	// It seems to be a 502 with nginx-fpm and a 503 with apache-fpm
	if runtime.GOOS == "darwin" && resp != nil && (resp.StatusCode == 502 || resp.StatusCode == 503) {
		t.Logf("Received %d error on macOS, retrying GetLocalHTTPResponse", resp.StatusCode)
		time.Sleep(time.Second)
		body, resp, err = GetLocalHTTPResponse(t, rawurl, httpTimeout)
	}
	assert.NoError(err, "GetLocalHTTPResponse returned err on rawurl %s: %v", rawurl, err)
	assert.Contains(body, expectedContent, "request %s got resp=%v, body:\n========\n%s\n==========\n", rawurl, resp, body)
	return resp, err
}

// PortPair is for tests to use naming portsets for tests
type PortPair struct {
	HTTPPort  string
	HTTPSPort string
}
