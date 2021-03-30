package ddevapp_test

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/drud/ddev/pkg/nodeps"
	"github.com/drud/ddev/pkg/version"

	"github.com/drud/ddev/pkg/globalconfig"
	"github.com/stretchr/testify/require"

	"github.com/drud/ddev/pkg/archive"
	"github.com/drud/ddev/pkg/ddevapp"
	"github.com/drud/ddev/pkg/dockerutil"
	"github.com/drud/ddev/pkg/exec"
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/output"
	"github.com/drud/ddev/pkg/testcommon"
	"github.com/drud/ddev/pkg/util"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/google/uuid"
	"github.com/lunixbochs/vtclean"
	log "github.com/sirupsen/logrus"
	asrt "github.com/stretchr/testify/assert"
)

var (
	DdevBin   = "ddev"
	TestSites = []testcommon.TestSite{
		{
			Name:                          "TestPkgWordpress",
			SourceURL:                     "https://github.com/drud/wordpress/archive/v0.4.0.tar.gz",
			ArchiveInternalExtractionPath: "wordpress-0.4.0/",
			FilesTarballURL:               "https://github.com/drud/ddev_test_tarballs/releases/download/v1.0/wordpress_files.tar.gz",
			DBTarURL:                      "https://github.com/drud/ddev_test_tarballs/releases/download/v1.0/wordpress_db.tar.gz",
			Docroot:                       "htdocs",
			Type:                          nodeps.AppTypeWordPress,
			Safe200URIWithExpectation:     testcommon.URIWithExpect{URI: "/readme.html", Expect: "Welcome. WordPress is a very special project to me."},
			DynamicURI:                    testcommon.URIWithExpect{URI: "/", Expect: "this post has a photo"},
			FilesImageURI:                 "/wp-content/uploads/2017/04/pexels-photo-265186-1024x683.jpeg",
		},
		{
			Name:                          "TestPkgDrupal8",
			SourceURL:                     "https://ftp.drupal.org/files/projects/drupal-8.9.13.tar.gz",
			ArchiveInternalExtractionPath: "drupal-8.9.13/",
			FilesTarballURL:               "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/d8_umami.files.tar.gz",
			FilesZipballURL:               "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/d8_umami.files.zip",
			DBTarURL:                      "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/d8_umami.sql.tar.gz",
			DBZipURL:                      "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/d8_umami.sql.zip",
			FullSiteTarballURL:            "",
			Type:                          nodeps.AppTypeDrupal8,
			Docroot:                       "",
			Safe200URIWithExpectation:     testcommon.URIWithExpect{URI: "/README.txt", Expect: "Drupal is an open source content management platform"},
			DynamicURI:                    testcommon.URIWithExpect{URI: "/node/2", Expect: "Vegan chocolate and nut brownies"},
			FilesImageURI:                 "/sites/default/files/vegan-chocolate-nut-brownies.jpg",
		},
		{
			Name:                          "TestPkgDrupal7", // Drupal D7
			SourceURL:                     "https://ftp.drupal.org/files/projects/drupal-7.61.tar.gz",
			ArchiveInternalExtractionPath: "drupal-7.61/",
			FilesTarballURL:               "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/d7test-7.59.files.tar.gz",
			DBTarURL:                      "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/d7test-7.59-db.tar.gz",
			FullSiteTarballURL:            "",
			Docroot:                       "",
			Type:                          nodeps.AppTypeDrupal7,
			Safe200URIWithExpectation:     testcommon.URIWithExpect{URI: "/README.txt", Expect: "Drupal is an open source content management platform"},
			DynamicURI:                    testcommon.URIWithExpect{URI: "/node/1", Expect: "D7 test project, kittens edition"},
			FilesImageURI:                 "/sites/default/files/field/image/kittens-large.jpg",
			FullSiteArchiveExtPath:        "docroot/sites/default/files",
		},
		{
			Name:                          "TestPkgDrupal6",
			SourceURL:                     "https://ftp.drupal.org/files/projects/drupal-6.38.tar.gz",
			ArchiveInternalExtractionPath: "drupal-6.38/",
			DBTarURL:                      "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/drupal6.38_db.tar.gz",
			FullSiteTarballURL:            "",
			FilesTarballURL:               "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/drupal6_files.tar.gz",
			Docroot:                       "",
			Type:                          nodeps.AppTypeDrupal6,
			Safe200URIWithExpectation:     testcommon.URIWithExpect{URI: "/CHANGELOG.txt", Expect: "Drupal 6.38, 2016-02-24"},
			DynamicURI:                    testcommon.URIWithExpect{URI: "/node/2", Expect: "This is a story. The story is somewhat shaky"},
			FilesImageURI:                 "/sites/default/files/garland_logo.jpg",
		},
		{
			Name:                          "TestPkgBackdrop",
			SourceURL:                     "https://github.com/backdrop/backdrop/archive/1.11.0.tar.gz",
			ArchiveInternalExtractionPath: "backdrop-1.11.0/",
			DBTarURL:                      "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/backdrop_db.11.0.tar.gz",
			FilesTarballURL:               "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/backdrop_files.11.0.tar.gz",
			FullSiteTarballURL:            "",
			Docroot:                       "",
			Type:                          nodeps.AppTypeBackdrop,
			Safe200URIWithExpectation:     testcommon.URIWithExpect{URI: "/README.md", Expect: "Backdrop is a full-featured content management system"},
			DynamicURI:                    testcommon.URIWithExpect{URI: "/posts/first-post-all-about-kittens", Expect: "Lots of kittens are a good thing"},
			FilesImageURI:                 "/files/styles/large/public/field/image/kittens-large.jpg",
		},
		{
			Name:                          "TestPkgTypo3",
			SourceURL:                     "https://github.com/drud/typo3-v9-test/archive/v0.2.2.tar.gz",
			ArchiveInternalExtractionPath: "typo3-v9-test-0.2.2/",
			DBTarURL:                      "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/typo3_v9.5_introduction_db.tar.gz",
			FilesTarballURL:               "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/typo3_v9.5_introduction_files.tar.gz",
			FullSiteTarballURL:            "",
			Docroot:                       "public",
			Type:                          nodeps.AppTypeTYPO3,
			Safe200URIWithExpectation:     testcommon.URIWithExpect{URI: "/README.txt", Expect: "junk readme simply for reading"},
			DynamicURI:                    testcommon.URIWithExpect{URI: "/index.php?id=65", Expect: "Boxed Content"},
			FilesImageURI:                 "/fileadmin/introduction/images/streets/nikita-maru-70928.jpg",
		},
		{
			Name:                          "testpkgmagento",
			SourceURL:                     "https://github.com/OpenMage/magento-mirror/archive/1.9.4.3.tar.gz",
			ArchiveInternalExtractionPath: "magento-mirror-1.9.4.3/",
			DBTarURL:                      "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/TestPkgMagento_db_secure_url.tar.gz",
			FilesTarballURL:               "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/magento_upload_files.tgz",
			FullSiteTarballURL:            "",
			Docroot:                       "",
			Type:                          nodeps.AppTypeMagento,
			Safe200URIWithExpectation:     testcommon.URIWithExpect{URI: "/LICENSE.txt", Expect: `Open Software License ("OSL")`},
			DynamicURI:                    testcommon.URIWithExpect{URI: "/", Expect: "This is a demo store"},
			FilesImageURI:                 "/media/wrapping/Chrysanthemum.jpg",
		},
		// Note that testpkgmagento2 code is enormous and makes this really, really slow.
		{
			Name: "testpkgmagento2",
			// echo "This is a junk" >pub/junk.txt && tar -czf .tarballs/testpkgmagento2_code_no_media.magento2.4_try_4.tgz --exclude=.ddev --exclude=var --exclude=pub/media --exclude=.tarballs --exclude=app/etc/env.php .
			SourceURL:                     "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/testpkgmagento2_code_no_media.magento2.4_try_4.tgz",
			ArchiveInternalExtractionPath: "",
			// ddev export-db --gzip=false --file=.tarballs/db.sql && tar -czf .tarballs/testpkgmagento2.magento2.4.db_try_4.tgz -C .tarballs db.sql
			DBTarURL: "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/testpkgmagento2.magento2.4.db_try_4.tgz",
			// tar -czf .tarballs/testpkgmagento2_files.magento2.4_try_4.tgz -C pub/media .
			FilesTarballURL:           "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/testpkgmagento2_files.magento2.4_try_4.tgz",
			FullSiteTarballURL:        "",
			Docroot:                   "pub",
			Type:                      nodeps.AppTypeMagento2,
			Safe200URIWithExpectation: testcommon.URIWithExpect{URI: "/junk.txt", Expect: `This is a junk`},
			DynamicURI:                testcommon.URIWithExpect{URI: "/index.php/unicycle.html", Expect: "Unicycle"},
			FilesImageURI:             "/media/catalog/product/r/a/randy_4th_of_july_unicycle.jpg",
		},
		{
			Name:                          "TestPkgDrupal9",
			SourceURL:                     "https://ftp.drupal.org/files/projects/drupal-9.1.4.tar.gz",
			ArchiveInternalExtractionPath: "drupal-9.1.4/",
			FilesTarballURL:               "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/d9_umami_files.tgz",
			FilesZipballURL:               "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/d9_umami_files.zip",
			DBTarURL:                      "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/d9_umami_sql.tar.gz",
			DBZipURL:                      "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/d9_umami.sql.zip",
			FullSiteTarballURL:            "",
			Type:                          nodeps.AppTypeDrupal9,
			Docroot:                       "",
			Safe200URIWithExpectation:     testcommon.URIWithExpect{URI: "/README.txt", Expect: "Drupal is an open source content management platform"},
			DynamicURI:                    testcommon.URIWithExpect{URI: "/node/1", Expect: "Deep mediterranean quiche"},
			FilesImageURI:                 "/sites/default/files/mediterranean-quiche-umami.jpg",
		},
		{
			Name:                          "TestPkgLaravel",
			SourceURL:                     "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/ddev-lumen-testapp.tar.gz",
			ArchiveInternalExtractionPath: "ddev-lumen-testapp/",
			FilesTarballURL:               "",
			FilesZipballURL:               "",
			DBTarURL:                      "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/ddev-lumen-testapp_sql.tar.gz",
			DBZipURL:                      "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/ddev-lumen-testapp_sql.zip",
			FullSiteTarballURL:            "",
			Type:                          nodeps.AppTypeLaravel,
			Docroot:                       "public",
			Safe200URIWithExpectation:     testcommon.URIWithExpect{URI: "/", Expect: "Laravel Components"},
			DynamicURI:                    testcommon.URIWithExpect{URI: "/api/status-code/200", Expect: "indicates that the request has succeeded."},
			FilesImageURI:                 "/images/200.jpg",
		},
		{
			Name:                          "testpkgshopware6",
			SourceURL:                     "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/shopware6_code.tgz",
			ArchiveInternalExtractionPath: "",
			FilesTarballURL:               "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/shopware6_files.tgz",
			DBTarURL:                      "https://github.com/drud/ddev_test_tarballs/releases/download/v1.1/shopware6_db.tgz",
			FullSiteTarballURL:            "",
			Type:                          nodeps.AppTypeShopware6,
			Docroot:                       "public",
			Safe200URIWithExpectation:     testcommon.URIWithExpect{URI: "/maintenance.html", Expect: "Our website is currently undergoing maintenance"},
			DynamicURI:                    testcommon.URIWithExpect{URI: "/Main-product-with-properties/SWDEMO10007.1", Expect: "Main product with properties"},
			FilesImageURI:                 "/media/2f/b0/e2/1603218072/hemd_600x600.jpg",
		},
	}

	FullTestSites = TestSites
)

func init() {
	// Make sets DDEV_BINARY_FULLPATH when building the executable
	if os.Getenv("DDEV_BINARY_FULLPATH") != "" {
		DdevBin = os.Getenv("DDEV_BINARY_FULLPATH")
	}
}

func TestMain(m *testing.M) {
	output.LogSetUp()

	// Since this may be first time ddev has been used, we need the
	// ddev_default network available.
	dockerutil.EnsureDdevNetwork()

	// Avoid having sudo try to add to /etc/hosts.
	// This is normally done by Testsite.Prepare()
	_ = os.Setenv("DDEV_NONINTERACTIVE", "true")

	// If GOTEST_SHORT is an integer, then use it as index for a single usage
	// in the array. Any value can be used, it will default to just using the
	// first site in the array.
	gotestShort := os.Getenv("GOTEST_SHORT")
	if gotestShort != "" {
		useSite := 0
		if site, err := strconv.Atoi(gotestShort); err == nil && site >= 0 && site < len(TestSites) {
			useSite = site
		}
		TestSites = []testcommon.TestSite{TestSites[useSite]}
	}

	// testRun is the exit result we'll provide.
	// Start with a clean exit result, it will be changed if we have trouble.
	testRun := 0
	err := globalconfig.ReadGlobalConfig()
	if err != nil {
		log.Fatalf("could not read globalconfig: %v", err)
	}

	for i, site := range TestSites {
		app := &ddevapp.DdevApp{Name: site.Name}
		_ = app.Stop(true, false)
		_ = globalconfig.RemoveProjectInfo(site.Name)

		err := TestSites[i].Prepare()
		if err != nil {
			log.Fatalf("Prepare() failed on TestSite.Prepare() site=%s, err=%v", TestSites[i].Name, err)
		}

		switchDir := TestSites[i].Chdir()

		testcommon.ClearDockerEnv()

		app = &ddevapp.DdevApp{}
		err = app.Init(TestSites[i].Dir)
		if err != nil {
			testRun = -1
			log.Errorf("TestMain startup: app.Init() failed on site %s in dir %s, err=%v", TestSites[i].Name, TestSites[i].Dir, err)
			continue
		}
		err = app.WriteConfig()
		if err != nil {
			testRun = -1
			log.Errorf("TestMain startup: app.WriteConfig() failed on site %s in dir %s, err=%v", TestSites[i].Name, TestSites[i].Dir, err)
			continue
		}
		for _, volume := range []string{app.Name + "-mariadb"} {
			err = dockerutil.RemoveVolume(volume)
			if err != nil {
				log.Errorf("TestMain startup: Failed to delete volume %s: %v", volume, err)
			}
		}

		switchDir()
	}

	if testRun == 0 {
		log.Debugln("Running tests.")
		testRun = m.Run()
	}

	for i, site := range TestSites {
		testcommon.ClearDockerEnv()

		app := &ddevapp.DdevApp{}

		err := app.Init(site.Dir)
		if err != nil {
			log.Fatalf("TestMain shutdown: app.Init() failed on site %s in dir %s, err=%v", TestSites[i].Name, TestSites[i].Dir, err)
		}

		if app.SiteStatus() != ddevapp.SiteStopped {
			err = app.Stop(true, false)
			if err != nil {
				log.Fatalf("TestMain shutdown: app.Stop() failed on site %s, err=%v", TestSites[i].Name, err)
			}
		}
		site.Cleanup()
	}

	os.Exit(testRun)
}

// TestDdevStart tests the functionality that is called when "ddev start" is executed
func TestDdevStart(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}

	// Make sure this leaves us in the original test directory
	testDir, _ := os.Getwd()
	//nolint: errcheck
	defer os.Chdir(testDir)

	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()

	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s DdevStart", site.Name))

	err := app.Init(site.Dir)
	assert.NoError(err)

	err = app.Start()
	assert.NoError(err)

	// Make sure the -built docker image exists before stop
	webBuilt := version.GetWebImage() + "-" + site.Name + "-built"
	dbBuilt := version.GetWebImage() + "-" + site.Name + "-built"
	exists, err := dockerutil.ImageExistsLocally(webBuilt)
	assert.NoError(err)
	assert.True(exists)

	//nolint: errcheck
	defer app.Stop(true, false)

	// ensure .ddev/.ddev-docker-compose* exists inside .ddev site folder
	composeFile := fileutil.FileExists(app.DockerComposeYAMLPath())
	assert.True(composeFile)

	for _, containerType := range [3]string{"web", "db", "dba"} {
		containerName, err := constructContainerName(containerType, app)
		assert.NoError(err)
		check, err := testcommon.ContainerCheck(containerName, "running")
		assert.NoError(err)
		assert.True(check, "Container check on %s failed", containerType)
	}

	if util.IsCommandAvailable("mysql") {
		dbPort, err := app.GetPublishedPort("db")
		assert.NoError(err)

		dockerIP, _ := dockerutil.GetDockerIP()
		out, err := exec.RunCommand("mysql", []string{"--user=db", "--password=db", "--port=" + strconv.Itoa(dbPort), "--database=db", "--host=" + dockerIP, "-e", "SELECT 1;"})
		assert.NoError(err)
		assert.Contains(out, "1")
	} else {
		fmt.Print("TestDddevStart skipping check for local mysql connection because mysql command not in path")
	}

	err = app.Stop(true, false)
	assert.NoError(err)

	// Make sure the -built docker images do not exist after stop with removeData
	for _, imageName := range []string{webBuilt, dbBuilt} {
		exists, err = dockerutil.ImageExistsLocally(imageName)
		assert.NoError(err)
		assert.False(exists, "image %s should not have existed but still exists (while testing %s)", app.Name)
	}

	runTime()
	switchDir()

	// Start up TestSites[0] again with a post-start hook
	// When run the first time, it should execute the hook, second time it should not
	err = os.Chdir(site.Dir)
	assert.NoError(err)
	err = app.Init(site.Dir)
	app.Hooks = map[string][]ddevapp.YAMLTask{"post-start": {{"exec": "echo hello"}}}

	assert.NoError(err)
	stdoutFunc, err := util.CaptureOutputToFile()
	assert.NoError(err)
	promptOutFunc := util.CaptureUserOut()
	err = app.Start()
	assert.NoError(err)
	//nolint: errcheck
	defer app.Stop(true, false)
	out := stdoutFunc()
	UOut := promptOutFunc()
	assert.Contains(UOut, "Running task: Exec command 'echo hello' in container/service 'web'")
	assert.Contains(out, "hello\n")

	// try to start a site of same name at different path
	another := site
	tmpDir := testcommon.CreateTmpDir("another")
	copyDir := filepath.Join(tmpDir, "copy")
	err = fileutil.CopyDir(site.Dir, copyDir)
	assert.NoError(err)
	another.Dir = copyDir
	//nolint: errcheck
	defer os.RemoveAll(copyDir)

	badapp := &ddevapp.DdevApp{}

	err = badapp.Init(copyDir)
	//nolint: errcheck
	defer badapp.Stop(true, false)
	if err == nil {
		logs, logErr := app.CaptureLogs("web", false, "")
		require.Error(t, err, "did not receive err from badapp.Init, logErr=%v, logs:\n======================= logs from app webserver =================\n%s\n============ end logs =========\n", logErr, logs)
	}
	if err != nil {
		assert.Contains(err.Error(), fmt.Sprintf("a project (web container) in running state already exists for %s that was created at %s", TestSites[0].Name, TestSites[0].Dir))
	}

	// Try to start a site of same name at an equivalent but different path. It should work.
	tmpDir, err = testcommon.OsTempDir()
	assert.NoError(err)
	symlink := filepath.Join(tmpDir, fileutil.RandomFilenameBase())
	err = os.Symlink(app.AppRoot, symlink)
	assert.NoError(err)
	//nolint: errcheck
	defer os.Remove(symlink)
	symlinkApp := &ddevapp.DdevApp{}

	err = symlinkApp.Init(symlink)
	assert.NoError(err)
	//nolint: errcheck
	defer symlinkApp.Stop(true, false)
	// Make sure that GetActiveApp() also fails when trying to start app of duplicate name in current directory.
	switchDir = another.Chdir()
	defer switchDir()

	_, err = ddevapp.GetActiveApp("")
	assert.Error(err)
	if err != nil {
		assert.Contains(err.Error(), fmt.Sprintf("a project (web container) in running state already exists for %s that was created at %s", TestSites[0].Name, TestSites[0].Dir))
	}
	testcommon.CleanupDir(another.Dir)
}

// TestDdevStartMultipleHostnames tests start with multiple hostnames
func TestDdevStartMultipleHostnames(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}

	for _, site := range TestSites {
		runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s DdevStartMultipleHostnames", site.Name))
		testcommon.ClearDockerEnv()

		err := app.Init(site.Dir)
		assert.NoError(err)

		// site.Name is explicitly added because if not removed in GetHostNames() it will cause ddev-router failure
		// "a" is repeated for the same reason; a user error of this type should not cause a failure; GetHostNames()
		// should uniqueify them.
		app.AdditionalHostnames = []string{"sub1." + site.Name, "sub2." + site.Name, "subname.sub3." + site.Name, site.Name, site.Name, site.Name}

		// sub1.<sitename>.ddev.site and sitename.ddev.site are deliberately included to prove they don't
		// cause ddev-router failures"
		// Note that these AdditionalFQDNs require sudo privileges, which the test runners
		// don't typically have.
		app.AdditionalFQDNs = []string{"one.example.com", "two.example.com", "a.one.example.com", site.Name + "." + app.ProjectTLD, "sub1." + site.Name + "." + app.ProjectTLD}

		err = app.WriteConfig()
		assert.NoError(err)

		err = app.StartAndWait(5)
		assert.NoError(err)
		if err != nil && strings.Contains(err.Error(), "db container failed") {
			container, err := app.FindContainerByType("db")
			assert.NoError(err)
			out, err := exec.RunCommand("docker", []string{"logs", container.Names[0]})
			assert.NoError(err)
			t.Logf("DB Logs after app.Start: \n%s\n== END DB LOGS ==", out)
		}

		// ensure .ddev/docker-compose*.yaml exists inside .ddev site folder
		composeFile := fileutil.FileExists(app.DockerComposeYAMLPath())
		assert.True(composeFile)

		for _, containerType := range [3]string{"web", "db", "dba"} {
			containerName, err := constructContainerName(containerType, app)
			assert.NoError(err)
			check, err := testcommon.ContainerCheck(containerName, "running")
			assert.NoError(err)
			assert.True(check, "Container check on %s failed", containerType)
		}

		_, _, urls := app.GetAllURLs()
		t.Logf("Testing these URLs: %v", urls)
		_, _, allURLs := app.GetAllURLs()
		for _, url := range allURLs {
			_, _ = testcommon.EnsureLocalHTTPContent(t, url+site.Safe200URIWithExpectation.URI, site.Safe200URIWithExpectation.Expect)
		}

		out, err := exec.RunCommand(DdevBin, []string{"list"})
		assert.NoError(err)
		t.Logf("=========== output of ddev list ==========\n%s\n============", out)

		// Multiple projects can't run at the same time with the fqdns, so we need to clean
		// up these for tests that run later.
		app.AdditionalFQDNs = []string{}
		app.AdditionalHostnames = []string{}
		err = app.WriteConfig()
		assert.NoError(err)

		err = app.Stop(true, false)
		assert.NoError(err)

		runTime()
	}
}

// TestDdevStartUnmanagedSettings start and config with disable_settings_management
func TestDdevStartUnmanagedSettings(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}

	// Make sure this leaves us in the original test directory
	testDir, _ := os.Getwd()
	//nolint: errcheck
	defer os.Chdir(testDir)

	// Use Drupal8 only, mostly for the composer example
	site := FullTestSites[1]
	// If running this with GOTEST_SHORT we have to create the directory, tarball etc.
	if site.Dir == "" || !fileutil.FileExists(site.Dir) {
		app := &ddevapp.DdevApp{Name: site.Name}
		_ = app.Stop(true, false)
		_ = globalconfig.RemoveProjectInfo(site.Name)

		err := site.Prepare()
		require.NoError(t, err)
		// nolint: errcheck
		defer os.RemoveAll(site.Dir)
	}
	switchDir := site.Chdir()
	defer switchDir()

	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s DdevStart", site.Name))
	defer runTime()

	err := app.Init(site.Dir)
	assert.NoError(err)

	// Previous tests may have left settings files
	_ = os.Remove(app.SiteSettingsPath)
	_ = os.Remove(app.SiteDdevSettingsFile)

	// On initial init, settings files should not exist
	assert.False(fileutil.FileExists(app.SiteSettingsPath))
	assert.False(fileutil.FileExists(app.SiteDdevSettingsFile))

	app.DisableSettingsManagement = true
	err = app.WriteConfig()
	assert.NoError(err)

	// After config, they should still not exist, because we had DisableSettingsManagement
	assert.False(fileutil.FileExists(app.SiteSettingsPath))
	assert.False(fileutil.FileExists(app.SiteDdevSettingsFile))

	err = app.Start()
	assert.NoError(err)
	//nolint: errcheck
	defer app.Stop(true, false)

	// After start, they should still not exist, because we had DisableSettingsManagement
	assert.False(fileutil.FileExists(app.SiteSettingsPath))
	assert.False(fileutil.FileExists(app.SiteDdevSettingsFile))

	app.DisableSettingsManagement = false
	err = app.WriteConfig()
	assert.NoError(err)
	_, err = app.CreateSettingsFile()
	assert.NoError(err)

	// Now with DisableSettingsManagement=false, both should exist after config/settings creation
	assert.FileExists(app.SiteSettingsPath)
	assert.FileExists(app.SiteDdevSettingsFile)

	_ = os.Remove(filepath.Join(app.SiteSettingsPath))
	_ = os.Remove(filepath.Join(app.SiteDdevSettingsFile))

	assert.False(fileutil.FileExists(app.SiteSettingsPath))
	assert.False(fileutil.FileExists(app.SiteDdevSettingsFile))

	err = app.Start()
	assert.NoError(err)
	//nolint: errcheck
	defer app.Stop(true, false)

	// Now with DisableSettingsManagement=false, start should have created both
	assert.FileExists(app.SiteSettingsPath)
	assert.FileExists(app.SiteDdevSettingsFile)

}

// TestDdevNoProjectMount tests running without the app file mount.
func TestDdevNoProjectMount(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}

	// Make sure this leaves us in the original test directory
	testDir, _ := os.Getwd()
	//nolint: errcheck
	defer os.Chdir(testDir)

	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()

	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s %s", t.Name(), site.Name))
	defer runTime()

	err := app.Init(site.Dir)
	assert.NoError(err)

	app.NoProjectMount = true
	err = app.WriteConfig()
	assert.NoError(err)

	defer func() {
		err = app.Stop(true, false)
		assert.NoError(err)
		app.NoProjectMount = false
		err = app.WriteConfig()
		assert.NoError(err)
	}()

	err = app.Start()
	assert.NoError(err)

	stdout, _, err := app.Exec(&ddevapp.ExecOpts{
		Service: "web",
		Dir:     "/var/www/html",
		Cmd:     `findmnt -T /var/www/html | awk '$1 != "TARGET" {printf $1}'`,
	})
	assert.NoError(err)
	assert.NotEqual("/var/www/html", stdout)
}

// TestDdevXdebugEnabled tests running with xdebug_enabled = true, etc.
func TestDdevXdebugEnabled(t *testing.T) {
	// 2021-02: I've been unable to make this test work on WSL2, even though it's easy to demonstrate
	// that it works using PhpStorm, etc. The go listener here doesn't seem to listen on all interfaces.
	// If you get golang listening, then enter the web container and try to connect to the port golang
	// is listening on, it can't connect. However, if you use netcat to listen on the wsl2 side and then
	// connect to it from inside the container, it connects fine.
	if nodeps.IsWSL2() {
		t.Skip("Skipping on WSL2 because this test doesn't work although manual testing works")
	}
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}
	testcommon.ClearDockerEnv()

	// On macOS we want to just listen on localhost port, so as to not trigger
	// firewall block. On other systems, just listen on all interfaces
	listenPort := ":9000"
	if runtime.GOOS == "darwin" {
		listenPort = "127.0.0.1:9000"
	}

	site := TestSites[0]
	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s %s", site.Name, t.Name()))

	phpVersions := nodeps.ValidPHPVersions
	phpKeys := make([]string, 0, len(phpVersions))
	for k := range phpVersions {
		phpKeys = append(phpKeys, k)
	}
	sort.Strings(phpKeys)

	err := app.Init(site.Dir)
	assert.NoError(err)

	t.Cleanup(func() {
		app.XdebugEnabled = false
		app.PHPVersion = nodeps.PHPDefault
		err = app.WriteConfig()
		assert.NoError(err)
		err = app.Stop(true, false)
		assert.NoError(err)
	})

	for _, v := range phpKeys {
		app.PHPVersion = v
		t.Logf("Beginning XDebug checks with XDebug php%s\n", v)
		fmt.Printf("Attempting XDebug checks with XDebug %s\n", v)
		err = app.Start()
		require.NoError(t, err)

		opts := &ddevapp.ExecOpts{
			Service: "web",
			Cmd:     "php --ri xdebug",
		}
		stdout, _, err := app.Exec(opts)
		assert.Error(err)
		assert.Contains(stdout, "Extension 'xdebug' not present")

		// Run with xdebug enabled
		_, _, err = app.Exec(&ddevapp.ExecOpts{
			Cmd: "enable_xdebug",
		})
		assert.NoError(err)

		stdout, _, err = app.Exec(opts)
		assert.NoError(err)

		if err != nil {
			t.Errorf("Aborting xdebug check for php%s: %v", v, err)
			continue
		}
		// PHP 7.2 through 8.0 gets xdebug 3.0+
		if app.PHPVersion == nodeps.PHP72 || app.PHPVersion == nodeps.PHP73 || app.PHPVersion == nodeps.PHP74 || app.PHPVersion == nodeps.PHP80 {
			assert.Contains(stdout, "xdebug.mode => debug,develop => debug,develop", "xdebug is not enabled for %s", v)
		} else {
			assert.Contains(stdout, "xdebug support => enabled", "xdebug is not enabled for %s", v)
		}

		if app.PHPVersion == nodeps.PHP72 || app.PHPVersion == nodeps.PHP73 || app.PHPVersion == nodeps.PHP74 || app.PHPVersion == nodeps.PHP80 {
			assert.Contains(stdout, "xdebug.client_host => host.docker.internal => host.docker.internal")
		} else {
			assert.Contains(stdout, "xdebug.remote_host => host.docker.internal => host.docker.internal")
		}

		// Start a listener on port 9000 of localhost (where PHPStorm or whatever would listen)
		listener, err := net.Listen("tcp", listenPort)
		require.NoError(t, err)

		// Curl to the project's index.php or anything else
		_, _, _ = testcommon.GetLocalHTTPResponse(t, app.GetHTTPURL())

		fmt.Printf("Attempting accept of port 9000 with xdebug enabled, PHP version=%s\n", v)

		// Accept is blocking, no way to timeout, so use
		// goroutine instead.
		acceptListenDone := make(chan bool, 1)
		defer close(acceptListenDone)

		go func() {
			conn, err := listener.Accept()
			assert.NoError(err)
			if err == nil {
				t.Logf("Completed accept of port 9000 with xdebug enabled, PHP version=%s, time=%v\n", v, time.Now())
			}
			// Grab the Xdebug connection start and look in it for "Xdebug"
			b := make([]byte, 650)
			_, err = bufio.NewReader(conn).Read(b)
			assert.NoError(err)
			lineString := string(b)
			assert.Contains(lineString, "Xdebug")
			assert.Contains(lineString, `xdebug:language_version="`+v)
			acceptListenDone <- true
		}()

		select {
		case <-acceptListenDone:
			fmt.Printf("Read from acceptListenDone at %v\n", time.Now())
		case <-time.After(3 * time.Second):
			t.Fatalf("Timed out waiting for accept/listen at %v, PHP version %v\n", time.Now(), v)
		}
	}
	runTime()
}

// TestDdevXhprofEnabled tests running with xhprof_enabled = true, etc.
func TestDdevXhprofEnabled(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}
	testcommon.ClearDockerEnv()

	site := TestSites[0]
	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s %s", site.Name, t.Name()))

	phpVersions := nodeps.ValidPHPVersions
	// Does not work with php5.6 anyway (SEGV), for resource conservation
	// skip older unsupported versions
	for _, k := range []string{"5.6", "7.0", "7.1"} {
		delete(phpVersions, k)
	}
	phpKeys := make([]string, 0, len(phpVersions))
	for k := range phpVersions {
		phpKeys = append(phpKeys, k)
	}
	sort.Strings(phpKeys)

	err := app.Init(site.Dir)
	assert.NoError(err)

	phpInfoFile := path.Join(app.AppRoot, app.Docroot, "phpinfo.php")
	err = os.WriteFile(phpInfoFile, []byte("<?php phpinfo();"), 0755)
	require.NoError(t, err)
	t.Cleanup(func() {
		app.PHPVersion = nodeps.PHPDefault
		app.WebserverType = nodeps.WebserverDefault
		err = os.Remove(phpInfoFile)
		assert.NoError(err)
		err = app.WriteConfig()
		assert.NoError(err)
		err = app.Stop(true, false)
		assert.NoError(err)
	})

	webserverKeys := make([]string, 0, len(nodeps.ValidWebserverTypes))
	for k := range nodeps.ValidWebserverTypes {
		webserverKeys = append(webserverKeys, k)
	}

	for _, webserverKey := range webserverKeys {
		app.WebserverType = webserverKey

		for _, v := range phpKeys {
			t.Logf("Beginning XHProf checks with XHProf webserver_type=%s php%s\n", webserverKey, v)
			fmt.Printf("Attempting XHProf checks with XHProf PHP%s\n", v)
			app.PHPVersion = v

			err = app.Start()
			require.NoError(t, err)

			stdout, _, err := app.Exec(&ddevapp.ExecOpts{
				Service: "web",
				Cmd:     fmt.Sprintf("php --ri xhprof"),
			})
			assert.Error(err)
			assert.Contains(stdout, "Extension 'xhprof' not present")

			// Run with xhprof enabled
			_, _, err = app.Exec(&ddevapp.ExecOpts{
				Cmd: fmt.Sprintf("enable_xhprof"),
			})
			assert.NoError(err)

			stdout, _, err = app.Exec(&ddevapp.ExecOpts{
				Service: "web",
				Cmd:     fmt.Sprintf("php --ri xhprof"),
			})
			assert.NoError(err)
			if err != nil {
				t.Errorf("Aborting xhprof check for php%s: %v", v, err)
				continue
			}
			assert.Contains(stdout, "xhprof.output_dir", "xhprof is not enabled for %s", v)

			out, _, err := testcommon.GetLocalHTTPResponse(t, app.GetHTTPSURL()+"/phpinfo.php")
			assert.NoError(err, "Failed to get base URL webserver_type=%s, php_version=%s", webserverKey, v)
			assert.Contains(out, "module_xhprof")

			out, _, err = testcommon.GetLocalHTTPResponse(t, app.GetHTTPSURL()+"/xhprof/")
			assert.NoError(err)
			// Output should contain at least one run
			assert.Contains(out, ".ddev.xhprof</a><small>")

			// Disable all to avoid confusion
			_, _, err = app.Exec(&ddevapp.ExecOpts{
				Cmd: fmt.Sprintf("disable_xhprof && rm -rf /tmp/xhprof"),
			})
			assert.NoError(err)
		}
	}
	runTime()
}

// TestDdevMysqlWorks tests that mysql client can be run in both containers.
func TestDdevMysqlWorks(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}

	site := TestSites[0]
	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s DdevMysqlWorks", site.Name))

	err := app.Init(site.Dir)
	assert.NoError(err)

	testcommon.ClearDockerEnv()
	err = app.StartAndWait(0)
	//nolint: errcheck
	defer app.Stop(true, false)
	require.NoError(t, err)

	// Test that mysql + .my.cnf works on web container
	_, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "web",
		Cmd:     "mysql -e 'SELECT USER();' | grep 'db@'",
	})
	assert.NoError(err)
	_, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "web",
		Cmd:     "mysql -e 'SELECT DATABASE();' | grep 'db'",
	})
	assert.NoError(err)

	// Test that mysql + .my.cnf works on db container
	_, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "db",
		Cmd:     "mysql -e 'SELECT USER();' | grep 'root@localhost'",
	})
	assert.NoError(err)
	_, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "db",
		Cmd:     "mysql -e 'SELECT DATABASE();' | grep 'db'",
	})
	assert.NoError(err)

	err = app.Stop(true, false)
	assert.NoError(err)

	runTime()

}

// TestStartWithoutDdev makes sure we don't have a regression where lack of .ddev
// causes a panic.
func TestStartWithoutDdevConfig(t *testing.T) {
	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)
	testDir := testcommon.CreateTmpDir(t.Name())

	// testcommon.Chdir()() and CleanupDir() check their own errors (and exit)
	defer testcommon.CleanupDir(testDir)
	defer testcommon.Chdir(testDir)()

	err := os.MkdirAll(testDir+"/sites/default", 0777)
	assert.NoError(err)
	err = os.Chdir(testDir)
	assert.NoError(err)

	_, err = ddevapp.GetActiveApp("")
	assert.Error(err)
	if err != nil {
		assert.Contains(err.Error(), "Could not find a project")
	}
}

// TestGetApps tests the GetActiveProjects function to ensure it accurately returns a list of running applications.
func TestGetApps(t *testing.T) {
	assert := asrt.New(t)

	// Start the apps.
	for _, site := range TestSites {
		testcommon.ClearDockerEnv()
		app := &ddevapp.DdevApp{}

		err := app.Init(site.Dir)
		assert.NoError(err)

		err = app.Start()
		assert.NoError(err)
	}

	apps := ddevapp.GetActiveProjects()

	for _, testSite := range TestSites {
		var found bool
		for _, app := range apps {
			if testSite.Name == app.GetName() {
				found = true
				break
			}
		}
		assert.True(found, "Found testSite %s in list", testSite.Name)
	}

	// Now shut down all sites as we expect them to be shut down.
	for _, site := range TestSites {
		testcommon.ClearDockerEnv()
		app := &ddevapp.DdevApp{}

		err := app.Init(site.Dir)
		assert.NoError(err)

		err = app.Stop(true, false)
		assert.NoError(err)
	}
}

// TestDdevImportDB tests the functionality that is called when "ddev import-db" is executed
func TestDdevImportDB(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}
	testDir, _ := os.Getwd()

	site := TestSites[0]

	switchDir := site.Chdir()
	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s %s", site.Name, t.Name()))

	testcommon.ClearDockerEnv()
	err := app.Init(site.Dir)
	assert.NoError(err)
	err = app.Start()
	assert.NoError(err)
	defer func() {
		app.Hooks = nil
		_ = app.WriteConfig()
		_ = app.Stop(true, false)
	}()

	_, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "db",
		Cmd:     "mysql -N -e 'DROP DATABASE IF EXISTS test;'",
	})
	assert.NoError(err)

	app.Hooks = map[string][]ddevapp.YAMLTask{"post-import-db": {{"exec-host": "touch hello-post-import-db-" + app.Name}}, "pre-import-db": {{"exec-host": "touch hello-pre-import-db-" + app.Name}}}

	// Test simple db loads.
	for _, file := range []string{"users.sql", "users.mysql", "users.sql.gz", "users.mysql.gz", "users.sql.tar", "users.mysql.tar", "users.sql.tar.gz", "users.mysql.tar.gz", "users.sql.tgz", "users.mysql.tgz", "users.sql.zip", "users.mysql.zip", "users_with_USE_statement.sql"} {
		path := filepath.Join(testDir, "testdata", t.Name(), file)
		err = app.ImportDB(path, "", false, false, "db")
		assert.NoError(err, "Failed to app.ImportDB path: %s err: %v", path, err)
		if err != nil {
			continue
		}

		// There should be exactly the one users table for each of these files
		out, _, err := app.Exec(&ddevapp.ExecOpts{
			Service: "db",
			Cmd:     "mysql -N -e 'SHOW TABLES;' | cat",
		})
		assert.NoError(err)
		assert.Equal("users\n", out)

		// Verify that no extra database was created
		out, _, err = app.Exec(&ddevapp.ExecOpts{
			Service: "db",
			Cmd:     `mysql -N -e 'SHOW DATABASES;' | egrep -v "^(information_schema|performance_schema|mysql)$"`,
		})
		assert.NoError(err)
		assert.Equal("db\n", out)

		// Test that a settings file has correct hash_salt format
		switch app.Type {
		case nodeps.AppTypeDrupal7:
			drupalHashSalt, err := fileutil.FgrepStringInFile(app.SiteDdevSettingsFile, "$drupal_hash_salt")
			assert.NoError(err)
			assert.True(drupalHashSalt)
		case nodeps.AppTypeDrupal8:
			settingsHashSalt, err := fileutil.FgrepStringInFile(app.SiteDdevSettingsFile, "settings['hash_salt']")
			assert.NoError(err)
			assert.True(settingsHashSalt)
		case nodeps.AppTypeWordPress:
			hasAuthSalt, err := fileutil.FgrepStringInFile(app.SiteSettingsPath, "SECURE_AUTH_SALT")
			assert.NoError(err)
			assert.True(hasAuthSalt)
		}
	}

	// Test database that has SQL DDL in the content to make sure nothing gets corrupted.
	_, _, err = app.Exec(&ddevapp.ExecOpts{Service: "db", Cmd: "mysql -N -e 'DROP TABLE IF EXISTS wp_posts;'"})
	require.NoError(t, err)
	file := "posts_with_ddl_content.sql"
	path := filepath.Join(testDir, "testdata", t.Name(), file)
	err = app.ImportDB(path, "", false, false, "db")
	assert.NoError(err, "Failed to app.ImportDB path: %s err: %v", path, err)
	checkImportDbImports(t, app)

	// Now test the same when importing from stdin, same file
	_, _, err = app.Exec(&ddevapp.ExecOpts{Service: "db", Cmd: "mysql -N -e 'DROP TABLE wp_posts;'"})
	require.NoError(t, err)
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	oldStdin := os.Stdin
	t.Cleanup(func() {
		os.Stdin = oldStdin
	})
	os.Stdin = f
	err = app.ImportDB("", "", false, false, "db")
	assert.NoError(err, "Failed to app.ImportDB path: %s err: %v", path, err)
	os.Stdin = oldStdin
	checkImportDbImports(t, app)

	// Verify that the count of tables is exactly what it should be, that nothing was lost in the
	// import due to embedded DDL statements.
	out, _, err := app.Exec(&ddevapp.ExecOpts{
		Service: "db",
		Cmd:     `mysql -N -e 'SELECT COUNT(*) FROM wp_posts;'`,
	})
	assert.NoError(err)
	assert.Equal("180\n", out)

	// Now check standard archive imports
	if site.DBTarURL != "" {
		_, cachedArchive, err := testcommon.GetCachedArchive(site.Name, site.Name+"_siteTarArchive", "", site.DBTarURL)
		require.NoError(t, err)
		err = app.ImportDB(cachedArchive, "", false, false, "db")
		assert.NoError(err)
		assert.FileExists("hello-pre-import-db-" + app.Name)
		assert.FileExists("hello-post-import-db-" + app.Name)
		err = os.Remove("hello-pre-import-db-" + app.Name)
		assert.NoError(err)
		err = os.Remove("hello-post-import-db-" + app.Name)
		assert.NoError(err)
	}

	if site.DBZipURL != "" {
		_, cachedArchive, err := testcommon.GetCachedArchive(site.Name, site.Name+"_siteZipArchive", "", site.DBZipURL)

		require.NoError(t, err)
		err = app.ImportDB(cachedArchive, "", false, false, "db")
		assert.NoError(err)

		assert.FileExists("hello-pre-import-db-" + app.Name)
		assert.FileExists("hello-post-import-db-" + app.Name)
		_ = os.RemoveAll("hello-pre-import-db-" + app.Name)
		_ = os.RemoveAll("hello-post-import-db-" + app.Name)
	}

	if site.FullSiteTarballURL != "" {
		_, cachedArchive, err := testcommon.GetCachedArchive(site.Name, site.Name+"_FullSiteTarballURL", "", site.FullSiteTarballURL)
		require.NoError(t, err)

		err = app.ImportDB(cachedArchive, "data.sql", false, false, "db")
		assert.NoError(err, "Failed to find data.sql at root of tarball %s", cachedArchive)
		assert.FileExists("hello-pre-import-db-" + app.Name)
		assert.FileExists("hello-post-import-db-" + app.Name)
		_ = os.RemoveAll("hello-pre-import-db-" + app.Name)
		_ = os.RemoveAll("hello-post-import-db-" + app.Name)
	}

	app.Hooks = nil

	for _, db := range []string{"db", "extradb"} {

		// Import from stdin, make sure that works
		inputFile := filepath.Join(testDir, "testdata", t.Name(), "stdintable.sql")
		f, err := os.Open(inputFile)
		require.NoError(t, err)
		// nolint: errcheck
		defer f.Close()
		savedStdin := os.Stdin
		os.Stdin = f
		err = app.ImportDB("", "", false, false, db)
		os.Stdin = savedStdin
		assert.NoError(err)
		out, _, err := app.Exec(&ddevapp.ExecOpts{
			Service: "db",
			Cmd:     fmt.Sprintf(`echo "SHOW DATABASES LIKE '%s'; SELECT COUNT(*) FROM stdintable;" | mysql -N %s`, db, db),
		})
		assert.NoError(err)
		assert.Equal(out, fmt.Sprintf("%s\n2\n", db))

		// Import 2-user users.sql into users table
		path := filepath.Join(testDir, "testdata", t.Name(), "users.sql")
		err = app.ImportDB(path, "", false, false, db)
		assert.NoError(err)
		out, stderr, err := app.Exec(&ddevapp.ExecOpts{
			Service: "db",
			Cmd:     fmt.Sprintf(`echo "SELECT COUNT(*) AS TOTALNUMBEROFTABLES FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = '%s';" | mysql -N %s`, db, db),
		})
		assert.NoError(err, "exec failed: %v", stderr)
		assert.Equal("1\n", out)

		// Import 1-user sql and make sure only one row is left there
		path = filepath.Join(testDir, "testdata", t.Name(), "oneuser.sql")
		err = app.ImportDB(path, "", false, false, db)
		assert.NoError(err)

		out, _, err = app.Exec(&ddevapp.ExecOpts{
			Service: "db",
			Cmd:     fmt.Sprintf(`echo "SELECT COUNT(*) AS TOTALNUMBEROFTABLES FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = '%s';" | mysql -N %s`, db, db),
		})
		assert.NoError(err)
		assert.Equal("1\n", out)

		// Import 2-user users.sql again, but with nodrop=true
		// We should end up with 2 tables now
		path = filepath.Join(testDir, "testdata", t.Name(), "users.sql")
		err = app.ImportDB(path, "", false, true, db)
		assert.NoError(err)
		out, _, err = app.Exec(&ddevapp.ExecOpts{
			Service: "db",
			Cmd:     fmt.Sprintf(`echo "SELECT COUNT(*) AS TOTALNUMBEROFTABLES FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = '%s';" | mysql -N %s`, db, db),
		})
		assert.NoError(err)
		assert.Equal("2\n", out)

	}
	runTime()
	switchDir()
}

func checkImportDbImports(t *testing.T, app *ddevapp.DdevApp) {
	assert := asrt.New(t)

	// There should be exactly the one wp_posts table for this file
	out, _, err := app.Exec(&ddevapp.ExecOpts{
		Service: "db",
		Cmd:     "mysql -N -e 'SHOW TABLES;' | cat",
	})
	assert.NoError(err)
	assert.Equal("wp_posts\n", out)

	// Verify that no extra database was created (this one has a CREATE DATABASE statement)
	out, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "db",
		Cmd:     `mysql -N -e 'SHOW DATABASES;' | egrep -v "^(information_schema|performance_schema|mysql)$"`,
	})
	assert.NoError(err)
	assert.Equal("db\n", out)

}

// TestDdevAllDatabases tests db import/export/start with all MariaDB versions
func TestDdevAllDatabases(t *testing.T) {
	assert := asrt.New(t)

	dbVersions := map[string]map[string]bool{
		"mariadb": nodeps.ValidMariaDBVersions,
		"mysql":   nodeps.ValidMySQLVersions,
	}
	//Use a smaller list if GOTEST_SHORT
	if os.Getenv("GOTEST_SHORT") != "" {
		t.Log("Using limited set of database servers because GOTEST_SHORT is set")
		dbVersions = map[string]map[string]bool{
			"mariadb": {nodeps.MariaDB102: true, nodeps.MariaDB103: true},
		}
		// If we have any mysql, limit what we test (but there may not be any)
		if len(dbVersions["mysql"]) != 0 {
			dbVersions["mysql"] = map[string]bool{nodeps.MySQL80: true, nodeps.MySQL56: true}
		}
	}

	app := &ddevapp.DdevApp{}
	testDir, _ := os.Getwd()

	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()
	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s %s", site.Name, t.Name()))

	testcommon.ClearDockerEnv()
	err := app.Init(site.Dir)
	assert.NoError(err)

	// Make sure there isn't an old db laying around
	_ = dockerutil.RemoveVolume(app.Name + "-mariadb")
	//nolint: errcheck
	defer func() {
		_ = app.Stop(true, false)
		// Make sure we leave the config.yaml in expected state
		app.MariaDBVersion = ""
		app.MySQLVersion = ""
		app.DBImage = ""
		_ = app.WriteConfig()
	}()

	for dbType, versions := range dbVersions {
		for v := range versions {

			t.Logf("testing db server functionality of %v:%v", dbType, v)
			_ = app.Stop(true, false)
			if dbType == "mariadb" {
				app.MySQLVersion = ""
				app.MariaDBVersion = v
			} else if dbType == "mysql" {
				app.MariaDBVersion = ""
				app.MySQLVersion = v
			}
			app.DBImage = ""
			_ = app.WriteConfig()
			startErr := app.Start()
			if startErr != nil {
				appLogs, err := ddevapp.GetErrLogsFromApp(app, startErr)
				assert.NoError(err)
				t.Fatalf("app.Start() failure %v; logs:\n=====\n%s\n=====\n", startErr, appLogs)
			}

			// Make sure the version of db running matches expected
			containerDBVersion, _, _ := app.Exec(&ddevapp.ExecOpts{
				Service: "db",
				Cmd:     "cat /var/lib/mysql/db_mariadb_version.txt",
			})
			assert.Equal(v, strings.Trim(containerDBVersion, "\n\r "))

			importPath := filepath.Join(testDir, "testdata", t.Name(), "users.sql")
			err = app.ImportDB(importPath, "", false, false, "db")
			assert.NoError(err, "failed to import %v", importPath)

			_ = os.Mkdir("tmp", 0777)
			err = fileutil.PurgeDirectory("tmp")
			assert.NoError(err)

			// Test that we can export-db to a gzipped file
			err = app.ExportDB("tmp/users1.sql.gz", true, "db")
			assert.NoError(err)

			// Validate contents
			err = archive.Ungzip("tmp/users1.sql.gz", "tmp")
			assert.NoError(err)
			stringFound, err := fileutil.FgrepStringInFile("tmp/users1.sql", "Table structure for table `users`")
			assert.NoError(err)
			assert.True(stringFound)

			err = fileutil.PurgeDirectory("tmp")
			assert.NoError(err)

			// Export to an ungzipped file and validate
			err = app.ExportDB("tmp/users2.sql", false, "db")
			assert.NoError(err)

			// Validate contents
			stringFound, err = fileutil.FgrepStringInFile("tmp/users2.sql", "Table structure for table `users`")
			assert.NoError(err)
			assert.True(stringFound)

			err = fileutil.PurgeDirectory("tmp")
			assert.NoError(err)

			// Capture to stdout without gzip compression
			stdout := util.CaptureStdOut()
			err = app.ExportDB("", false, "db")
			assert.NoError(err)
			out := stdout()
			assert.Contains(out, "Table structure for table `users`")

			snapshotName := v + "_" + fileutil.RandomFilenameBase()
			output, err := app.Snapshot(snapshotName)
			assert.NoError(err, "could not create snapshot %s for version %s: %v output=%v", snapshotName, v, err, output)
			err = app.RestoreSnapshot(snapshotName)
			assert.NoError(err, "could not restore snapshot %s for version %s: %v", snapshotName, v, err)

			// Make sure the version of db running matches expected
			containerDBVersion, _, _ = app.Exec(&ddevapp.ExecOpts{
				Service: "db",
				Cmd:     "cat /var/lib/mysql/db_mariadb_version.txt",
			})
			assert.Equal(v, strings.Trim(containerDBVersion, "\n\r "))

			// TODO: Restore a snapshot from a different version note warning.

			_ = app.Stop(true, false)
		}
	}
	runTime()
}

// TestDdevExportDB tests the functionality that is called when "ddev export-db" is executed
func TestDdevExportDB(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}
	testDir, _ := os.Getwd()

	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()

	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s DdevExportDB", site.Name))

	testcommon.ClearDockerEnv()
	err := app.Init(site.Dir)
	assert.NoError(err)
	err = app.Start()
	assert.NoError(err)
	//nolint: errcheck
	defer app.Stop(true, false)
	importPath := filepath.Join(testDir, "testdata", t.Name(), "users.sql")
	err = app.ImportDB(importPath, "", false, false, "db")
	require.NoError(t, err)

	_ = os.Mkdir("tmp", 0777)
	// Most likely reason for failure is it exists, so let that go
	err = fileutil.PurgeDirectory("tmp")
	assert.NoError(err)

	// Test that we can export-db to a gzipped file
	err = app.ExportDB("tmp/users1.sql.gz", true, "db")
	assert.NoError(err)

	// Validate contents
	err = archive.Ungzip("tmp/users1.sql.gz", "tmp")
	assert.NoError(err)
	stringFound, err := fileutil.FgrepStringInFile("tmp/users1.sql", "Table structure for table `users`")
	assert.NoError(err)
	assert.True(stringFound)

	err = fileutil.PurgeDirectory("tmp")
	assert.NoError(err)

	// Export to an ungzipped file and validate
	err = app.ExportDB("tmp/users2.sql", false, "db")
	assert.NoError(err)

	// Validate contents
	stringFound, err = fileutil.FgrepStringInFile("tmp/users2.sql", "Table structure for table `users`")
	assert.NoError(err)
	assert.True(stringFound)

	err = fileutil.PurgeDirectory("tmp")
	assert.NoError(err)

	// Capture to stdout without gzip compression
	stdout := util.CaptureStdOut()
	err = app.ExportDB("", false, "db")
	assert.NoError(err)
	output := stdout()
	assert.Contains(output, "Table structure for table `users`")

	// Export an alternate database
	importPath = filepath.Join(testDir, "testdata", t.Name(), "users.sql")
	err = app.ImportDB(importPath, "", false, false, "anotherdb")
	require.NoError(t, err)
	err = app.ExportDB("tmp/anotherdb.sql.gz", true, "anotherdb")
	assert.NoError(err)
	importPath = "tmp/anotherdb.sql.gz"
	err = app.ImportDB(importPath, "", false, false, "thirddb")
	assert.NoError(err)
	out, _, err := app.Exec(&ddevapp.ExecOpts{
		Service: "db",
		Cmd:     fmt.Sprintf(`echo "SELECT COUNT(*) FROM users;" | mysql -N thirddb`),
	})
	assert.NoError(err)
	assert.Equal("2\n", out)

	runTime()
}

// TestDdevFullSiteSetup tests a full import-db and import-files and then looks to see if
// we have a spot-test success hit on a URL
func TestDdevFullSiteSetup(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}

	for _, site := range TestSites {
		switchDir := site.Chdir()
		defer switchDir()
		runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s DdevFullSiteSetup", site.Name))
		t.Logf("== BEGIN TestDdevFullSiteSetup for %s\n", site.Name)
		testcommon.ClearDockerEnv()
		err := app.Init(site.Dir)
		assert.NoError(err)

		// Get files before start, as syncing can start immediately.
		if site.FilesTarballURL != "" {
			_, tarballPath, err := testcommon.GetCachedArchive(site.Name, "local-tarballs-files", "", site.FilesTarballURL)
			require.NoError(t, err)
			err = app.ImportFiles(tarballPath, "")
			assert.NoError(err)
		}

		// Running WriteConfig assures that settings.ddev.php gets written
		// so Drupal 8 won't try to set things unwriteable
		err = app.WriteConfig()
		assert.NoError(err)

		restoreOutput := util.CaptureUserOut()
		err = app.Start()
		assert.NoError(err)
		out := restoreOutput()
		assert.NotContains(out, "Unable to create settings file")

		// Validate PHPMyAdmin is working and database named db is present
		_, _ = testcommon.EnsureLocalHTTPContent(t, app.GetHTTPURL()+":8036/index.php?route=/database/structure&server=1&db=db", "No tables found in database")
		// Validate MailHog is working and "connected"
		_, _ = testcommon.EnsureLocalHTTPContent(t, app.GetHTTPURL()+":8025/#", "Connected")

		settingsLocation, err := app.DetermineSettingsPathLocation()
		assert.NoError(err)

		if app.Type != nodeps.AppTypeShopware6 {
			assert.Equal(filepath.Dir(settingsLocation), filepath.Dir(app.SiteSettingsPath))
		}
		if nodeps.ArrayContainsString([]string{"drupal6", "drupal7"}, app.Type) {
			assert.FileExists(filepath.Join(filepath.Dir(app.SiteSettingsPath), "drushrc.php"))
		}

		if site.DBTarURL != "" {
			_, cachedArchive, err := testcommon.GetCachedArchive(site.Name, site.Name+"_siteTarArchive", "", site.DBTarURL)
			require.NoError(t, err)
			err = app.ImportDB(cachedArchive, "", false, false, "db")
			assert.NoError(err, "failed to import-db with dbtarball %s, app.Type=%s, mariadb_version=%s, mysql_version=%s", site.DBTarURL, app.Type, app.MariaDBVersion, app.MySQLVersion)
		}

		startErr := app.StartAndWait(2)
		if startErr != nil {
			appLogs, getLogsErr := ddevapp.GetErrLogsFromApp(app, startErr)
			assert.NoError(getLogsErr)
			t.Fatalf("app.StartAndWait() failure err=%v; logs:\n=====\n%s\n=====\n", startErr, appLogs)
		}

		// Test static content.
		_, _ = testcommon.EnsureLocalHTTPContent(t, app.GetHTTPSURL()+site.Safe200URIWithExpectation.URI, site.Safe200URIWithExpectation.Expect)
		// Test dynamic php + database content.
		rawurl := app.GetHTTPSURL() + site.DynamicURI.URI
		body, resp, err := testcommon.GetLocalHTTPResponse(t, rawurl, 120)
		assert.NoError(err, "GetLocalHTTPResponse returned err on project=%s rawurl %s, resp=%v: %v", site.Name, rawurl, resp, err)
		if err != nil && strings.Contains(err.Error(), "container ") {
			logs, err := ddevapp.GetErrLogsFromApp(app, err)
			assert.NoError(err)
			t.Fatalf("Logs after GetLocalHTTPResponse: %s", logs)
		}
		assert.Contains(body, site.DynamicURI.Expect, "expected %s on project %s", site.DynamicURI.Expect, site.Name)

		// Load an image from the files section
		if site.FilesImageURI != "" {
			_, resp, err := testcommon.GetLocalHTTPResponse(t, app.GetHTTPSURL()+site.FilesImageURI)
			assert.NoError(err, "failed ImageURI response on project %s", site.Name)
			if err != nil && resp != nil {
				assert.Equal("image/jpeg", resp.Header["Content-Type"][0])
			}
		}

		// Make sure we can do a simple hit against the host-mount of web container.
		_, _ = testcommon.EnsureLocalHTTPContent(t, app.GetWebContainerDirectHTTPURL()+site.Safe200URIWithExpectation.URI, site.Safe200URIWithExpectation.Expect)

		// We don't want all the projects running at once.
		err = app.Stop(true, false)
		assert.NoError(err)

		runTime()
		switchDir()
	}
	fmt.Print()
}

// TestDdevSnapshotCleanup tests creating a snapshot and deleting it.
func TestDdevSnapshotCleanup(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}
	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()

	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("TestDdevSnapshotCleanup"))

	testcommon.ClearDockerEnv()
	err := app.Init(site.Dir)
	assert.NoError(err)

	err = app.StartAndWait(0)
	assert.NoError(err)

	// Make a snapshot of d7 tester test 1
	backupsDir := filepath.Join(app.GetConfigPath(""), "db_snapshots")
	snapshotName, err := app.Snapshot("d7testerTest1")
	assert.NoError(err)

	assert.True(fileutil.FileExists(filepath.Join(backupsDir, snapshotName, "xtrabackup_info")), "Expected that file xtrabackup_info in snapshot exists")

	err = app.Init(site.Dir)
	require.NoError(t, err)

	err = app.Start()
	require.NoError(t, err)
	//nolint: errcheck
	defer app.Stop(true, false)

	err = app.DeleteSnapshot("d7testerTest1")
	assert.NoError(err)

	// Snapshot data should be deleted
	assert.False(fileutil.FileExists(filepath.Join(backupsDir, snapshotName, "xtrabackup_info")), "Expected that file of snapshot is deleted during cleanup")

	runTime()
}

// TestGetLatestSnapshot tests if the latest snapshot of a project is returned correctly.
func TestGetLatestSnapshot(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}
	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()

	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("TestGetLatestSnapshot"))

	testcommon.ClearDockerEnv()
	err := app.Init(site.Dir)
	assert.NoError(err)

	err = app.StartAndWait(0)
	assert.NoError(err)
	//nolint: errcheck
	defer app.Stop(true, false)

	// Make three snapshots and compare the last
	_, err = app.Snapshot("d7testerTest1")
	assert.NoError(err)
	_, err = app.Snapshot("d7testerTest2")
	assert.NoError(err)
	_, err = app.Snapshot("d7testerTest3") // last = latest
	assert.NoError(err)

	latestSnapshot, err := app.GetLatestSnapshot()
	assert.NoError(err)
	assert.Equal("d7testerTest3", latestSnapshot)

	// delete last latest
	err = app.DeleteSnapshot("d7testerTest3")
	assert.NoError(err)
	latestSnapshot, err = app.GetLatestSnapshot()
	assert.NoError(err)
	assert.Equal("d7testerTest2", latestSnapshot, "d7testerTest2 should be latest snapshot")

	// cleanup snapshots
	err = app.DeleteSnapshot("d7testerTest2")
	assert.NoError(err)
	latestSnapshot, err = app.GetLatestSnapshot()
	assert.NoError(err)
	assert.Equal("d7testerTest1", latestSnapshot, "d7testerTest1 should be latest snapshot")

	err = app.DeleteSnapshot("d7testerTest1")
	assert.NoError(err)
	latestSnapshot, _ = app.GetLatestSnapshot()
	assert.NotEqual("d7testerTest1", latestSnapshot)

	runTime()
}

// TestDdevRestoreSnapshot tests creating a snapshot and reverting to it.
func TestDdevRestoreSnapshot(t *testing.T) {
	assert := asrt.New(t)
	testDir, _ := os.Getwd()
	app := &ddevapp.DdevApp{}

	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("TestDdevRestoreSnapshot"))

	d7testerTest1Dump, err := filepath.Abs(filepath.Join("testdata", t.Name(), "restore_snapshot", "d7tester_test_1.sql.gz"))
	assert.NoError(err)
	d7testerTest2Dump, err := filepath.Abs(filepath.Join("testdata", t.Name(), "restore_snapshot", "d7tester_test_2.sql.gz"))
	assert.NoError(err)

	// Use d7 only for this test, the key thing is the database interaction
	site := FullTestSites[2]
	// If running this with GOTEST_SHORT we have to create the directory, tarball etc.
	if site.Dir == "" || !fileutil.FileExists(site.Dir) {
		err = site.Prepare()
		require.NoError(t, err)
	}

	switchDir := site.Chdir()
	defer switchDir()

	testcommon.ClearDockerEnv()

	err = app.Init(site.Dir)
	require.NoError(t, err)

	t.Cleanup(func() {
		app.Hooks = nil
		_ = app.WriteConfig()
		err = app.Stop(true, false)
		assert.NoError(err)
	})

	app.Hooks = map[string][]ddevapp.YAMLTask{"post-snapshot": {{"exec-host": "touch hello-post-snapshot-" + app.Name}}, "pre-snapshot": {{"exec-host": "touch hello-pre-snapshot-" + app.Name}}}

	// First do regular start, which is good enough to get us to an ImportDB()
	err = app.Start()
	require.NoError(t, err)

	err = app.ImportDB(d7testerTest1Dump, "", false, false, "db")
	require.NoError(t, err, "Failed to app.ImportDB path: %s err: %v", d7testerTest1Dump, err)

	err = app.StartAndWait(2)
	require.NoError(t, err, "app.Start() failed on site %s, err=%v", site.Name, err)

	resp, ensureErr := testcommon.EnsureLocalHTTPContent(t, app.GetHTTPSURL(), "d7 tester test 1 has 1 node", 45)
	assert.NoError(ensureErr)
	if ensureErr != nil && strings.Contains(ensureErr.Error(), "container failed") {
		logs, err := ddevapp.GetErrLogsFromApp(app, ensureErr)
		assert.NoError(err)
		t.Fatalf("container failed: logs:\n=======\n%s\n========\n", logs)
	}
	require.NotNil(t, resp)
	if ensureErr != nil && resp.StatusCode != 200 {
		logs, err := app.CaptureLogs("web", false, "")
		assert.NoError(err)
		t.Fatalf("EnsureLocalHTTPContent received %d. Resp=%v, web logs=\n========\n%s\n=========\n", resp.StatusCode, resp, logs)
	}

	// Make a snapshot of d7 tester test 1
	backupsDir := filepath.Join(app.GetConfigPath(""), "db_snapshots")
	snapshotName, err := app.Snapshot("d7testerTest1")
	assert.NoError(err)

	assert.EqualValues(snapshotName, "d7testerTest1")
	assert.True(fileutil.FileExists(filepath.Join(backupsDir, snapshotName, "xtrabackup_info")))

	assert.FileExists("hello-pre-snapshot-" + app.Name)
	assert.FileExists("hello-post-snapshot-" + app.Name)
	err = os.Remove("hello-pre-snapshot-" + app.Name)
	assert.NoError(err)
	err = os.Remove("hello-post-snapshot-" + app.Name)
	assert.NoError(err)

	// Make sure duplicate snapshot name gives an error
	_, err = app.Snapshot(snapshotName)
	assert.Error(err)

	err = app.ImportDB(d7testerTest2Dump, "", false, false, "db")
	assert.NoError(err, "Failed to app.ImportDB path: %s err: %v", d7testerTest2Dump, err)
	_, _ = testcommon.EnsureLocalHTTPContent(t, app.GetHTTPSURL(), "d7 tester test 2 has 2 nodes", 45)

	snapshotName, err = app.Snapshot("d7testerTest2")
	assert.NoError(err)
	assert.EqualValues(snapshotName, "d7testerTest2")
	assert.True(fileutil.FileExists(filepath.Join(backupsDir, snapshotName, "xtrabackup_info")))

	app.Hooks = map[string][]ddevapp.YAMLTask{"post-restore-snapshot": {{"exec-host": "touch hello-post-restore-snapshot-" + app.Name}}, "pre-restore-snapshot": {{"exec-host": "touch hello-pre-restore-snapshot-" + app.Name}}}

	err = app.RestoreSnapshot("d7testerTest1")
	assert.NoError(err)

	assert.FileExists("hello-pre-restore-snapshot-" + app.Name)
	assert.FileExists("hello-post-restore-snapshot-" + app.Name)
	err = os.Remove("hello-pre-restore-snapshot-" + app.Name)
	assert.NoError(err)
	err = os.Remove("hello-post-restore-snapshot-" + app.Name)
	assert.NoError(err)

	_, _ = testcommon.EnsureLocalHTTPContent(t, app.GetHTTPSURL(), "d7 tester test 1 has 1 node", 45)
	err = app.RestoreSnapshot("d7testerTest2")
	assert.NoError(err)

	body, resp, err := testcommon.GetLocalHTTPResponse(t, app.GetHTTPSURL(), 45)
	assert.NoError(err, "GetLocalHTTPResponse returned err on rawurl %s: %v", app.GetHTTPSURL(), err)
	assert.Contains(body, "d7 tester test 2 has 2 nodes")
	if err != nil {
		t.Logf("resp after timeout: %v", resp)
		out, err := app.CaptureLogs("web", false, "")
		assert.NoError(err)
		t.Logf("web container logs after timeout: %s", out)
	}

	// Attempt a restore with a pre-mariadb_10.2 snapshot. It should fail and give a link.
	oldSnapshotTarball, err := filepath.Abs(filepath.Join(testDir, "testdata", t.Name(), "restore_snapshot", "d7tester_test_1.snapshot_mariadb_10_1.tgz"))
	assert.NoError(err)

	err = archive.Untar(oldSnapshotTarball, filepath.Join(site.Dir, ".ddev", "db_snapshots"), "")
	assert.NoError(err)
	err = app.RestoreSnapshot("d7tester_test_1.snapshot_mariadb_10.1")
	assert.Error(err)
	assert.Contains(err.Error(), "is not compatible")

	runTime()
}

// TestWriteableFilesDirectory tests to make sure that files created on host are writable on container
// and files ceated in container are correct user on host.
func TestWriteableFilesDirectory(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}
	site := TestSites[0]
	switchDir := site.Chdir()
	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s TestWritableFilesDirectory", site.Name))

	testcommon.ClearDockerEnv()
	err := app.Init(site.Dir)
	assert.NoError(err)

	err = app.StartAndWait(0)
	assert.NoError(err)

	uploadDir := app.GetUploadDir()
	assert.NotEmpty(uploadDir)

	// Use exec to touch a file in the container and see what the result is. Make sure it comes out with ownership
	// making it writeable on the host.
	filename := fileutil.RandomFilenameBase()
	dirname := fileutil.RandomFilenameBase()
	// Use path.Join for items on th container (linux) and filepath.Join for items on the host.
	inContainerDir := path.Join(uploadDir, dirname)
	onHostDir := filepath.Join(app.Docroot, inContainerDir)

	// The container execution directory is dependent on the app type
	switch app.Type {
	case nodeps.AppTypeWordPress, nodeps.AppTypeTYPO3, nodeps.AppTypePHP:
		inContainerDir = path.Join(app.Docroot, inContainerDir)
	}

	inContainerRelativePath := path.Join(inContainerDir, filename)
	onHostRelativePath := path.Join(onHostDir, filename)

	err = os.MkdirAll(onHostDir, 0775)
	assert.NoError(err)
	// Create a file in the directory to make sure it syncs
	f, err := os.OpenFile(filepath.Join(onHostDir, "junk.txt"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	assert.NoError(err)
	_ = f.Close()

	_, _, createFileErr := app.Exec(&ddevapp.ExecOpts{
		Service: "web",
		Cmd:     "echo 'content created inside container\n' >" + inContainerRelativePath,
	})
	assert.NoError(createFileErr)

	// Now try to append to the file on the host.
	// os.OpenFile() for append here fails if the file does not already exist.
	f, err = os.OpenFile(onHostRelativePath, os.O_APPEND|os.O_WRONLY, 0660)
	assert.NoError(err)
	_, err = f.WriteString("this addition to the file was added on the host side")
	assert.NoError(err)
	_ = f.Close()

	// Create a file on the host and see what the result is. Make sure we can not append/write to it in the container.
	filename = fileutil.RandomFilenameBase()
	dirname = fileutil.RandomFilenameBase()
	inContainerDir = path.Join(uploadDir, dirname)
	onHostDir = filepath.Join(app.Docroot, inContainerDir)
	// The container execution directory is dependent on the app type
	switch app.Type {
	case nodeps.AppTypeWordPress, nodeps.AppTypeTYPO3, nodeps.AppTypePHP:
		inContainerDir = path.Join(app.Docroot, inContainerDir)
	}

	inContainerRelativePath = path.Join(inContainerDir, filename)
	onHostRelativePath = filepath.Join(onHostDir, filename)

	err = os.MkdirAll(onHostDir, 0775)
	assert.NoError(err)

	f, err = os.OpenFile(onHostRelativePath, os.O_CREATE|os.O_RDWR, 0660)
	assert.NoError(err)
	_, err = f.WriteString("this base content was inserted on the host side\n")
	assert.NoError(err)
	_ = f.Close()

	// if the file exists, add to it. We don't want to add if it's not already there.
	_, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "web",
		Cmd:     "if [ -f " + inContainerRelativePath + " ]; then echo 'content added inside container\n' >>" + inContainerRelativePath + "; fi",
	})
	assert.NoError(err)
	// grep the file for both the content added on host and that added in container.
	_, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "web",
		Cmd:     "grep 'base content was inserted on the host' " + inContainerRelativePath + "&& grep 'content added inside container' " + inContainerRelativePath,
	})
	assert.NoError(err)

	err = app.Stop(true, false)
	assert.NoError(err)

	runTime()
	switchDir()
}

// TestDdevImportFilesDir tests that "ddev import-files" can successfully import non-archive directories
func TestDdevImportFilesDir(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}

	// Create a dummy directory to test non-archive imports
	importDir, err := os.MkdirTemp("", t.Name())
	assert.NoError(err)
	fileNames := make([]string, 0)
	for i := 0; i < 5; i++ {
		fileName := uuid.New().String()
		fileNames = append(fileNames, fileName)

		fullPath := filepath.Join(importDir, fileName)
		err = os.WriteFile(fullPath, []byte(fileName), 0644)
		assert.NoError(err)
	}

	for _, site := range TestSites {
		if site.FilesTarballURL == "" && site.FilesZipballURL == "" {
			t.Logf("== SKIP TestDdevImportFilesDir for %s (FilesTarballURL and FilesZipballURL are not provided)\n", site.Name)
			continue
		}

		switchDir := site.Chdir()
		runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s %s", site.Name, t.Name()))
		t.Logf("== BEGIN TestDdevImportFilesDir for %s\n", site.Name)

		testcommon.ClearDockerEnv()
		err = app.Init(site.Dir)
		assert.NoError(err)

		// Function under test
		err = app.ImportFiles(importDir, "")
		assert.NoError(err, "Importing a directory returned an error:", err)

		// Confirm contents of destination dir after import
		absUploadDir := filepath.Join(app.AppRoot, app.Docroot, app.GetUploadDir())
		uploadedFilesDirEntrySlice, err := os.ReadDir(absUploadDir)
		assert.NoError(err)

		uploadedFilesMap := map[string]bool{}
		for _, de := range uploadedFilesDirEntrySlice {
			uploadedFilesMap[filepath.Base(de.Name())] = true
		}

		for _, expectedFile := range fileNames {
			assert.True(uploadedFilesMap[expectedFile], "Expected file %s not found for site: %s", expectedFile, site.Name)
		}

		runTime()
		switchDir()
	}
}

// TestDdevImportFiles tests the functionality that is called when "ddev import-files" is executed
func TestDdevImportFiles(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}

	for _, site := range TestSites {
		if site.FilesTarballURL == "" && site.FilesZipballURL == "" && site.FullSiteTarballURL == "" {
			t.Logf("== SKIP TestDdevImportFiles for %s (FilesTarballURL and FilesZipballURL are not provided)\n", site.Name)
			continue
		}

		switchDir := site.Chdir()
		runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s %s", site.Name, t.Name()))

		testcommon.ClearDockerEnv()
		err := app.Init(site.Dir)
		assert.NoError(err)
		app.Hooks = map[string][]ddevapp.YAMLTask{"post-import-files": {{"exec-host": "touch hello-post-import-files-" + app.Name}}, "pre-import-files": {{"exec-host": "touch hello-pre-import-files-" + app.Name}}}

		if site.FilesTarballURL != "" {
			_, tarballPath, err := testcommon.GetCachedArchive(site.Name, "local-tarballs-files", "", site.FilesTarballURL)
			require.NoError(t, err)
			err = app.ImportFiles(tarballPath, "")
			assert.NoError(err)
		}

		if site.FilesZipballURL != "" {
			_, zipballPath, err := testcommon.GetCachedArchive(site.Name, "local-zipballs-files", "", site.FilesZipballURL)
			require.NoError(t, err)
			err = app.ImportFiles(zipballPath, "")
			assert.NoError(err)
		}

		if site.FullSiteTarballURL != "" && site.FullSiteArchiveExtPath != "" {
			_, siteTarPath, err := testcommon.GetCachedArchive(site.Name, "local-site-tar", "", site.FullSiteTarballURL)
			require.NoError(t, err)
			err = app.ImportFiles(siteTarPath, site.FullSiteArchiveExtPath)
			assert.NoError(err)
		}
		assert.FileExists("hello-pre-import-files-" + app.Name)
		assert.FileExists("hello-post-import-files-" + app.Name)
		err = os.Remove("hello-pre-import-files-" + app.Name)
		assert.NoError(err)
		err = os.Remove("hello-post-import-files-" + app.Name)
		assert.NoError(err)

		runTime()
		switchDir()
	}
}

// TestDdevImportFilesCustomUploadDir ensures that files are imported to a custom upload directory when requested
func TestDdevImportFilesCustomUploadDir(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}

	for _, site := range TestSites {
		switchDir := site.Chdir()
		runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s %s", site.Name, t.Name()))
		t.Logf("== BEGIN TestDdevImportFilesCustomUploadDir for %s\n", site.Name)

		testcommon.ClearDockerEnv()
		err := app.Init(site.Dir)
		assert.NoError(err)

		// Set custom upload dir
		app.UploadDir = "my/upload/dir"
		absUploadDir := filepath.Join(app.AppRoot, app.Docroot, app.UploadDir)
		err = os.MkdirAll(absUploadDir, 0755)
		assert.NoError(err)

		if site.FilesTarballURL != "" {
			_, tarballPath, err := testcommon.GetCachedArchive(site.Name, "local-tarballs-files", "", site.FilesTarballURL)
			require.NoError(t, err)
			err = app.ImportFiles(tarballPath, "")
			assert.NoError(err)

			// Ensure upload dir isn't empty
			dirEntrySlice, err := os.ReadDir(absUploadDir)
			assert.NoError(err)
			assert.NotEmpty(dirEntrySlice)
		}

		if site.FilesZipballURL != "" {
			_, zipballPath, err := testcommon.GetCachedArchive(site.Name, "local-zipballs-files", "", site.FilesZipballURL)
			require.NoError(t, err)
			err = app.ImportFiles(zipballPath, "")
			assert.NoError(err)

			// Ensure upload dir isn't empty
			dirEntrySlice, err := os.ReadDir(absUploadDir)
			assert.NoError(err)
			assert.NotEmpty(dirEntrySlice)
		}

		if site.FullSiteTarballURL != "" && site.FullSiteArchiveExtPath != "" {
			_, siteTarPath, err := testcommon.GetCachedArchive(site.Name, "local-site-tar", "", site.FullSiteTarballURL)
			require.NoError(t, err)
			err = app.ImportFiles(siteTarPath, site.FullSiteArchiveExtPath)
			assert.NoError(err)

			// Ensure upload dir isn't empty
			dirEntrySlice, err := os.ReadDir(absUploadDir)
			assert.NoError(err)
			assert.NotEmpty(dirEntrySlice)
		}

		runTime()
		switchDir()
	}
}

// TestDdevExec tests the execution of commands inside a docker container of a site.
func TestDdevExec(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}
	testDir, _ := os.Getwd()

	site := TestSites[0]
	switchDir := site.Chdir()
	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s DdevExec", site.Name))

	// Make a dummy service out of busybox
	err := fileutil.CopyFile(filepath.Join(testDir, "testdata", t.Name(), "docker-compose.busybox.yaml"), filepath.Join(site.Dir, ".ddev", "docker-compose.busybox.yaml"))
	assert.NoError(err)

	err = app.Init(site.Dir)
	assert.NoError(err)

	t.Cleanup(func() {
		app.Hooks = nil
		err = app.Stop(true, false)
		assert.NoError(err)
		err = app.WriteConfig()
		assert.NoError(err)
		err = os.RemoveAll(filepath.Join(site.Dir, ".ddev", "docker-compose.busybox.yaml"))
		assert.NoError(err)
	})

	app.Hooks = map[string][]ddevapp.YAMLTask{"post-exec": {{"exec-host": "touch hello-post-exec-" + app.Name}}, "pre-exec": {{"exec-host": "touch hello-pre-exec-" + app.Name}}}

	startErr := app.Start()
	if startErr != nil {
		logs, err := ddevapp.GetErrLogsFromApp(app, startErr)
		assert.NoError(err)
		t.Fatalf("app.Start() failed err=%v, logs from broken container:\n=======\n%s\n========\n", startErr, logs)
	}

	out, _, err := app.Exec(&ddevapp.ExecOpts{
		Service: "web",
		Cmd:     "pwd",
	})
	assert.NoError(err)
	assert.Contains(out, "/var/www/html")

	assert.FileExists("hello-pre-exec-" + app.Name)
	assert.FileExists("hello-post-exec-" + app.Name)
	err = os.Remove("hello-pre-exec-" + app.Name)
	assert.NoError(err)
	err = os.Remove("hello-post-exec-" + app.Name)
	assert.NoError(err)

	out, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "web",
		Dir:     "/usr/local",
		Cmd:     "pwd",
	})
	assert.NoError(err)
	assert.Contains(out, "/usr/local")

	_, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "db",
		Cmd:     "mysql -e 'DROP DATABASE db;'",
	})
	assert.NoError(err)
	_, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "db",
		Cmd:     "mysql information_schema -e 'CREATE DATABASE db;'",
	})
	assert.NoError(err)

	// Make sure that exec works on non-ddev container like busybox as well
	grepOpts := &ddevapp.ExecOpts{
		Service: "busybox",
		Cmd:     "ls | grep bin",
	}
	_, _, err = app.Exec(grepOpts)
	assert.NoError(err)

	_, stderr, err := app.Exec(&ddevapp.ExecOpts{
		Service: "busybox",
		Cmd:     "echo $ENVDOESNOTEXIST",
	})
	assert.Error(err)
	assert.Contains(stderr, "parameter not set")

	errorOpts := &ddevapp.ExecOpts{
		Service: "busybox",
		Cmd:     "this is an error;",
	}
	_, stderr, err = app.Exec(errorOpts)
	assert.Error(err)
	assert.Contains(stderr, "this: not found")
	err = app.ExecWithTty(errorOpts)
	assert.Error(err)
	assert.Contains(stderr, "this: not found")

	// Now kill the busybox service and make sure that responses to app.Exec are correct
	client := dockerutil.GetDockerClient()
	bbc, err := dockerutil.FindContainerByName(fmt.Sprintf("ddev-%s-%s", app.Name, "busybox"))
	require.NoError(t, err)
	err = client.StopContainer(bbc.ID, 2)
	assert.NoError(err)

	simpleOpts := ddevapp.ExecOpts{
		Service: "busybox",
		Cmd:     "ls",
	}
	_, _, err = app.Exec(&simpleOpts)
	assert.Error(err)
	assert.Contains(err.Error(), "not currently running")
	assert.Contains(err.Error(), "state=exited")

	err = client.RemoveContainer(docker.RemoveContainerOptions{ID: bbc.ID, Force: true})
	assert.NoError(err)
	_, _, err = app.Exec(&simpleOpts)
	assert.Error(err)
	assert.Contains(err.Error(), "does not exist")
	assert.Contains(err.Error(), "state=doesnotexist")

	runTime()
	switchDir()
}

// TestDdevLogs tests the container log output functionality.
func TestDdevLogs(t *testing.T) {
	assert := asrt.New(t)

	// Skip test because on Windows because the CaptureUserOut() hangs, at least
	// sometimes.
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test TestDdevLogs on Windows")
	}

	app := &ddevapp.DdevApp{}

	site := TestSites[0]
	switchDir := site.Chdir()
	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s DdevLogs", site.Name))

	err := app.Init(site.Dir)
	assert.NoError(err)

	//nolint: errcheck
	defer app.Stop(true, false)

	startErr := app.StartAndWait(0)
	if startErr != nil {
		logs, err := ddevapp.GetErrLogsFromApp(app, startErr)
		assert.NoError(err)
		t.Fatalf("app.Start failed, err=%v, logs=\n========\n%s\n===========\n", startErr, logs)
	}

	out, err := app.CaptureLogs("web", false, "")
	assert.NoError(err)
	assert.Contains(out, "Server started")

	out, err = app.CaptureLogs("db", false, "")
	assert.NoError(err)
	assert.Contains(out, "MySQL init process done. Ready for start up.")

	// Test that we can get logs when project is stopped also
	err = app.Pause()
	assert.NoError(err)

	out, err = app.CaptureLogs("web", false, "")
	assert.NoError(err)
	assert.Contains(out, "Server started")

	out, err = app.CaptureLogs("db", false, "")
	assert.NoError(err)
	assert.Contains(out, "MySQL init process done. Ready for start up.")

	runTime()
	switchDir()
}

// TestProcessHooks tests execution of commands defined in config.yaml
func TestProcessHooks(t *testing.T) {
	assert := asrt.New(t)

	site := TestSites[0]
	switchDir := site.Chdir()

	runTime := util.TimeTrack(time.Now(), t.Name())

	testcommon.ClearDockerEnv()
	app, err := ddevapp.NewApp(site.Dir, true)
	assert.NoError(err)
	t.Cleanup(func() {
		err = app.Stop(true, false)
		assert.NoError(err)
		app.Hooks = nil
		err = app.WriteConfig()
		assert.NoError(err)
		switchDir()
	})
	err = app.Start()
	assert.NoError(err)

	// Note that any ExecHost commands must be able to run on Windows.
	// echo and pwd are things that work pretty much the same in both places.
	app.Hooks = map[string][]ddevapp.YAMLTask{
		"hook-test": {
			{"exec": "ls /usr/local/bin/composer"},
			{"exec-host": "echo something"},
			{"exec": "echo MYSQL_USER=${MYSQL_USER}", "service": "db"},
			{"exec": "echo TestProcessHooks > /var/www/html/TestProcessHooks${DDEV_ROUTER_HTTPS_PORT}.txt"},
			{"exec": "touch /var/tmp/TestProcessHooks && touch /var/www/html/touch_works_after_and.txt"},
		},
	}

	captureOutputFunc, err := util.CaptureOutputToFile()
	assert.NoError(err)
	userOutFunc := util.CaptureUserOut()

	err = app.ProcessHooks("hook-test")
	assert.NoError(err)
	out := captureOutputFunc()
	userOut := userOutFunc()
	// Ignore color in output, can be different in different OS's
	out = vtclean.Clean(out, false)

	assert.Contains(userOut, "Executing hook-test hook")
	assert.Contains(userOut, "Exec command 'ls /usr/local/bin/composer' in container/service 'web'")
	assert.Contains(userOut, "Exec command 'echo something' on the host")
	assert.Contains(userOut, "Exec command 'echo MYSQL_USER=${MYSQL_USER}' in container/service 'db'")
	assert.Contains(out, "MYSQL_USER=db")
	assert.Contains(userOut, "Exec command 'echo TestProcessHooks > /var/www/html/TestProcessHooks${DDEV_ROUTER_HTTPS_PORT}.txt' in container/service 'web'")
	assert.Contains(userOut, "Exec command 'touch /var/tmp/TestProcessHooks && touch /var/www/html/touch_works_after_and.txt' in container/service 'web',")
	assert.FileExists(filepath.Join(app.AppRoot, fmt.Sprintf("TestProcessHooks%s.txt", app.RouterHTTPSPort)))
	assert.FileExists(filepath.Join(app.AppRoot, "touch_works_after_and.txt"))

	// Attempt processing hooks with a guaranteed failure
	app.Hooks = map[string][]ddevapp.YAMLTask{
		"hook-test": {
			{"exec": "ls /does-not-exist"},
		},
	}
	// With default setting, ProcessHooks should succeeed
	err = app.ProcessHooks("hook-test")
	assert.NoError(err)
	// With FailOnHookFail or FailOnHookFailGlobal or both, it should fail.
	app.FailOnHookFail = true
	err = app.ProcessHooks("hook-test")
	assert.Error(err)
	app.FailOnHookFail = false
	app.FailOnHookFailGlobal = true
	err = app.ProcessHooks("hook-test")
	assert.Error(err)
	app.FailOnHookFail = true
	err = app.ProcessHooks("hook-test")
	assert.Error(err)

	runTime()
}

// TestDdevPause tests the functionality that is called when "ddev pause" is executed
func TestDdevPause(t *testing.T) {
	assert := asrt.New(t)

	app := &ddevapp.DdevApp{}

	site := TestSites[0]
	switchDir := site.Chdir()
	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s DdevStop", site.Name))

	testcommon.ClearDockerEnv()
	err := app.Init(site.Dir)
	assert.NoError(err)
	err = app.StartAndWait(0)
	app.Hooks = map[string][]ddevapp.YAMLTask{"post-pause": {{"exec-host": "touch hello-post-pause-" + app.Name}}, "pre-pause": {{"exec-host": "touch hello-pre-pause-" + app.Name}}}

	defer func() {
		app.Hooks = nil
		_ = app.WriteConfig()
		_ = app.Stop(true, false)
	}()
	require.NoError(t, err)
	err = app.Pause()
	assert.NoError(err)

	for _, containerType := range [3]string{"web", "db", "dba"} {
		containerName, err := constructContainerName(containerType, app)
		assert.NoError(err)
		check, err := testcommon.ContainerCheck(containerName, "exited")
		assert.NoError(err)
		assert.True(check, containerType, "container has exited")
	}
	assert.FileExists("hello-pre-pause-" + app.Name)
	assert.FileExists("hello-post-pause-" + app.Name)
	err = os.Remove("hello-pre-pause-" + app.Name)
	assert.NoError(err)
	err = os.Remove("hello-post-pause-" + app.Name)
	assert.NoError(err)

	runTime()
	switchDir()
}

// TestDdevStopMissingDirectory tests that the 'ddev stop' command works properly on sites with missing directories or ddev configs.
func TestDdevStopMissingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping because unreliable on Windows")
	}

	assert := asrt.New(t)

	site := TestSites[0]
	testcommon.ClearDockerEnv()
	app := &ddevapp.DdevApp{}
	err := app.Init(site.Dir)
	assert.NoError(err)

	startErr := app.StartAndWait(0)
	//nolint: errcheck
	defer app.Stop(true, false)
	if startErr != nil {
		logs, err := ddevapp.GetErrLogsFromApp(app, startErr)
		assert.NoError(err)
		t.Fatalf("app.StartAndWait failed err=%v logs from broken container: \n=======\n%s\n========\n", startErr, logs)
	}

	tempPath := testcommon.CreateTmpDir("site-copy")
	siteCopyDest := filepath.Join(tempPath, "site")
	defer removeAllErrCheck(tempPath, assert)

	_ = app.Stop(false, false)

	// Move the site directory to a temp location to mimic a missing directory.
	err = os.Rename(site.Dir, siteCopyDest)
	assert.NoError(err)

	//nolint: errcheck
	defer os.Rename(siteCopyDest, site.Dir)

	// ddev stop (in cmd) actually does the check for missing project files,
	// so we imitate that here.
	err = ddevapp.CheckForMissingProjectFiles(app)
	assert.Error(err)
	if err != nil {
		assert.Contains(err.Error(), "If you would like to continue using ddev to manage this project please restore your files to that directory.")
	}
}

// TestDdevDescribe tests that the describe command works properly on a running
// and also a stopped project.
func TestDdevDescribe(t *testing.T) {
	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}

	site := TestSites[0]
	switchDir := site.Chdir()

	testcommon.ClearDockerEnv()
	err := app.Init(site.Dir)
	assert.NoError(err)

	app.Hooks = map[string][]ddevapp.YAMLTask{"post-describe": {{"exec-host": "touch hello-post-describe-" + app.Name}}, "pre-describe": {{"exec-host": "touch hello-pre-describe-" + app.Name}}}

	startErr := app.StartAndWait(0)
	defer func() {
		_ = app.Stop(true, false)
		app.Hooks = nil
		_ = app.WriteConfig()
	}()
	// If we have a problem starting, get the container logs and output.
	if startErr != nil {
		out, logsErr := app.CaptureLogs("web", false, "")
		assert.NoError(logsErr)

		healthcheck, inspectErr := exec.RunCommandPipe("sh", []string{"-c", fmt.Sprintf("docker inspect ddev-%s-web|jq -r '.[0].State.Health.Log[-1]'", app.Name)})
		assert.NoError(inspectErr)

		t.Fatalf("app.StartAndWait(%s) failed: %v, \nweb container healthcheck='%s', \n== web container logs=\n%s\n== END web container logs ==", site.Name, err, healthcheck, out)
	}

	desc, err := app.Describe(false)
	assert.NoError(err)
	assert.EqualValues(ddevapp.SiteRunning, desc["status"], "")
	assert.EqualValues(app.GetName(), desc["name"])
	assert.EqualValues(ddevapp.RenderHomeRootedDir(app.GetAppRoot()), desc["shortroot"])
	assert.EqualValues(app.GetAppRoot(), desc["approot"])
	assert.EqualValues(app.GetPhpVersion(), desc["php_version"])

	assert.FileExists("hello-pre-describe-" + app.Name)
	assert.FileExists("hello-post-describe-" + app.Name)
	err = os.Remove("hello-pre-describe-" + app.Name)
	assert.NoError(err)
	err = os.Remove("hello-post-describe-" + app.Name)
	assert.NoError(err)

	// Now stop it and test behavior.
	err = app.Pause()
	assert.NoError(err)

	desc, err = app.Describe(false)
	assert.NoError(err)
	assert.EqualValues(ddevapp.SitePaused, desc["status"])

	switchDir()
}

// TestDdevDescribeMissingDirectory tests that the describe command works properly on sites with missing directories or ddev configs.
func TestDdevDescribeMissingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping because unreliable on Windows")
	}

	assert := asrt.New(t)
	site := TestSites[0]
	tempPath := testcommon.CreateTmpDir("site-copy")
	siteCopyDest := filepath.Join(tempPath, "site")
	defer removeAllErrCheck(tempPath, assert)

	app := &ddevapp.DdevApp{}
	err := app.Init(site.Dir)
	assert.NoError(err)
	startErr := app.StartAndWait(0)
	//nolint: errcheck
	defer app.Stop(true, false)
	if startErr != nil {
		logs, err := ddevapp.GetErrLogsFromApp(app, startErr)
		assert.NoError(err)
		t.Fatalf("app.StartAndWait failed err=%v logs from broken container: \n=======\n%s\n========\n", startErr, logs)
	}
	// Move the site directory to a temp location to mimic a missing directory.
	err = app.Stop(false, false)
	assert.NoError(err)
	err = os.Rename(site.Dir, siteCopyDest)
	assert.NoError(err)

	desc, err := app.Describe(false)
	assert.NoError(err)
	assert.Contains(desc["status"], ddevapp.SiteDirMissing, "Status did not include the phrase '%s' when describing a site with missing directories.", ddevapp.SiteDirMissing)
	// Move the site directory back to its original location.
	err = os.Rename(siteCopyDest, site.Dir)
	assert.NoError(err)
}

// TestRouterPortsCheck makes sure that we can detect if the ports are available before starting the router.
func TestRouterPortsCheck(t *testing.T) {
	assert := asrt.New(t)

	// First, stop any sites that might be running
	app := &ddevapp.DdevApp{}

	// Stop/Remove all sites, which should get the router out of there.
	for _, site := range TestSites {
		switchDir := site.Chdir()

		testcommon.ClearDockerEnv()
		err := app.Init(site.Dir)
		assert.NoError(err)

		if app.SiteStatus() == ddevapp.SiteRunning || app.SiteStatus() == ddevapp.SitePaused {
			err = app.Stop(true, false)
			assert.NoError(err)
		}

		switchDir()
	}

	// Now start one site, it's hard to get router to behave without one site.
	site := TestSites[0]
	testcommon.ClearDockerEnv()

	err := app.Init(site.Dir)
	assert.NoError(err)
	startErr := app.StartAndWait(5)
	//nolint: errcheck
	defer app.Stop(true, false)
	if startErr != nil {
		appLogs, getLogsErr := ddevapp.GetErrLogsFromApp(app, startErr)
		assert.NoError(getLogsErr)
		t.Fatalf("app.StartAndWait() failure; err=%v logs:\n=====\n%s\n=====\n", startErr, appLogs)
	}

	app, err = ddevapp.GetActiveApp(site.Name)
	if err != nil {
		t.Fatalf("Failed to GetActiveApp(%s), err:%v", site.Name, err)
	}
	startErr = app.StartAndWait(5)
	//nolint: errcheck
	defer app.Stop(true, false)
	if startErr != nil {
		appLogs, getLogsErr := ddevapp.GetErrLogsFromApp(app, startErr)
		assert.NoError(getLogsErr)
		t.Fatalf("app.StartAndWait() failure err=%v logs:\n=====\n%s\n=====\n", startErr, appLogs)
	}

	// Stop the router using code from StopRouterIfNoContainers().
	// StopRouterIfNoContainers can't be used here because it checks to see if containers are running
	// and doesn't do its job as a result.
	dest := ddevapp.RouterComposeYAMLPath()
	_, _, err = dockerutil.ComposeCmd([]string{dest}, "-p", ddevapp.RouterProjectName, "down")
	assert.NoError(err, "Failed to stop router using docker-compose, err=%v", err)

	// Occupy port 80 using docker busybox trick, then see if we can start router.
	// This is done with docker so that we don't have to use explicit sudo
	containerID, err := exec.RunCommand("sh", []string{"-c", "docker run -d -p80:80 --rm busybox:latest sleep 100 2>/dev/null"})
	if err != nil {
		t.Fatalf("Failed to run docker command to occupy port 80, err=%v output=%v", err, containerID)
	}
	containerID = strings.TrimSpace(containerID)

	// Now try to start the router. It should fail because the port is occupied.
	err = ddevapp.StartDdevRouter()
	assert.Error(err, "Failure: router started even though port 80 was occupied")

	// Remove our dummy busybox docker container.
	out, err := exec.RunCommand("docker", []string{"rm", "-f", containerID})
	assert.NoError(err, "Failed to docker rm the port-occupier container, err=%v output=%v", err, out)
}

// TestCleanupWithoutCompose ensures app containers can be properly cleaned up without a docker-compose config file present.
func TestCleanupWithoutCompose(t *testing.T) {
	assert := asrt.New(t)

	// Skip test because we can't rename folders while they're in use if running on Windows.
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test TestCleanupWithoutCompose on Windows")
	}

	site := TestSites[0]

	revertDir := site.Chdir()
	app := &ddevapp.DdevApp{}

	testcommon.ClearDockerEnv()
	err := app.Init(site.Dir)
	assert.NoError(err)

	// Ensure we have a site started so we have something to cleanup

	startErr := app.StartAndWait(5)
	//nolint: errcheck
	defer app.Stop(true, false)
	if startErr != nil {
		appLogs, getLogsErr := ddevapp.GetErrLogsFromApp(app, startErr)
		assert.NoError(getLogsErr)
		t.Fatalf("app.StartAndWait failure; err=%v, logs:\n=====\n%s\n=====\n", startErr, appLogs)
	}
	// Setup by creating temp directory and nesting a folder for our site.
	tempPath := testcommon.CreateTmpDir("site-copy")
	siteCopyDest := filepath.Join(tempPath, "site")

	//nolint: errcheck
	defer os.RemoveAll(tempPath)
	//nolint: errcheck
	defer revertDir()
	// Move the site directory back to its original location.
	//nolint: errcheck
	defer os.Rename(siteCopyDest, site.Dir)

	// Move site directory to a temp directory to mimick a missing directory.
	err = os.Rename(site.Dir, siteCopyDest)
	assert.NoError(err)

	// Call the Stop command()
	// Notice that we set the removeData parameter to true.
	// This gives us added test coverage over sites with missing directories
	// by ensuring any associated database files get cleaned up as well.
	err = app.Stop(true, false)
	assert.NoError(err)
	assert.Empty(globalconfig.DdevGlobalConfig.ProjectList[app.Name])

	for _, containerType := range [3]string{"web", "db", "dba"} {
		_, err := constructContainerName(containerType, app)
		assert.Error(err)
	}

	// Ensure there are no volumes associated with this project
	client := dockerutil.GetDockerClient()
	volumes, err := client.ListVolumes(docker.ListVolumesOptions{})
	assert.NoError(err)
	for _, volume := range volumes {
		assert.False(volume.Labels["com.docker.compose.project"] == "ddev"+strings.ToLower(app.GetName()))
	}

}

// TestGetappsEmpty ensures that GetActiveProjects returns an empty list when no applications are running.
func TestGetAppsEmpty(t *testing.T) {
	assert := asrt.New(t)

	// Ensure test sites are removed
	for _, site := range TestSites {
		app := &ddevapp.DdevApp{}

		switchDir := site.Chdir()

		testcommon.ClearDockerEnv()
		err := app.Init(site.Dir)
		assert.NoError(err)

		if app.SiteStatus() != ddevapp.SiteStopped {
			err = app.Stop(true, false)
			assert.NoError(err)
		}
		switchDir()
	}

	apps := ddevapp.GetActiveProjects()
	assert.Equal(0, len(apps), "Expected to find no apps but found %d apps=%v", len(apps), apps)
}

// TestRouterNotRunning ensures the router is shut down after all sites are stopped.
// This depends on TestGetAppsEmpty() having shut everything down.
func TestRouterNotRunning(t *testing.T) {
	assert := asrt.New(t)
	containers, err := dockerutil.GetDockerContainers(false)
	assert.NoError(err)

	for _, container := range containers {
		assert.NotEqual("ddev-router", dockerutil.ContainerName(container), "ddev-router was not supposed to be running but it was")
	}
}

// TestListWithoutDir prevents regression where ddev list panics if one of the
// sites found is missing a directory
func TestListWithoutDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping because unreliable on Windows")
	}
	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)
	testcommon.ClearDockerEnv()
	packageDir, _ := os.Getwd()

	// startCount is the count of apps at the start of this adventure
	apps := ddevapp.GetActiveProjects()
	startCount := len(apps)

	testDir := testcommon.CreateTmpDir("TestStartWithoutDdevConfig")
	defer testcommon.CleanupDir(testDir)

	err := os.MkdirAll(testDir+"/sites/default", 0777)
	assert.NoError(err)
	err = os.Chdir(testDir)
	assert.NoError(err)

	app, err := ddevapp.NewApp(testDir, true)
	assert.NoError(err)
	app.Name = "junk"
	app.Type = nodeps.AppTypeDrupal7
	err = app.WriteConfig()
	assert.NoError(err)

	// Do a start on the configured site.
	app, err = ddevapp.GetActiveApp("")
	assert.NoError(err)
	err = app.Start()
	assert.NoError(err)

	// Make sure we move out of the directory for Windows' sake
	garbageDir := testcommon.CreateTmpDir("RestingHere")
	defer testcommon.CleanupDir(garbageDir)

	err = os.Chdir(garbageDir)
	assert.NoError(err)

	testcommon.CleanupDir(testDir)

	apps = ddevapp.GetActiveProjects()

	assert.EqualValues(len(apps), startCount+1)

	// Make a whole table and make sure our app directory missing shows up.
	// This could be done otherwise, but we'd have to go find the site in the
	// array first.
	table := ddevapp.CreateAppTable()
	for _, site := range apps {
		desc, err := site.Describe(false)
		if err != nil {
			t.Fatalf("Failed to describe site %s: %v", site.GetName(), err)
		}

		ddevapp.RenderAppRow(table, desc)
	}

	// testDir on Windows has backslashes in it, resulting in invalid regexp
	// Remove them and use ., which is good enough.
	testDirSafe := strings.Replace(testDir, "\\", ".", -1)
	assert.Regexp(regexp.MustCompile("(?s)"+ddevapp.SiteDirMissing+".*"+testDirSafe), table.String())

	err = app.Stop(true, false)
	assert.NoError(err)

	// Change back to package dir. Lots of things will have to be cleaned up
	// in defers, and for windows we have to not be sitting in them.
	err = os.Chdir(packageDir)
	assert.NoError(err)
}

type URLRedirectExpectations struct {
	scheme              string
	uri                 string
	expectedRedirectURI string
}

// TestAppdirAlreadyInUse tests trying to start a project in an already-used
// directory will fail
func TestAppdirAlreadyInUse(t *testing.T) {
	assert := asrt.New(t)
	originalProjectName := "originalproject"
	secondProjectName := "secondproject"
	// Create a temporary directory and switch to it.
	tmpdir := testcommon.CreateTmpDir(t.Name())

	app, err := ddevapp.NewApp(tmpdir, false)
	require.NoError(t, err)
	defer func() {
		app.Name = originalProjectName
		_ = app.Stop(true, false)
		app.Name = secondProjectName
		_ = app.Stop(true, false)

		testcommon.Chdir(tmpdir)()
		testcommon.CleanupDir(tmpdir)
	}()

	// Write/create global with the name "originalproject"
	app.Name = originalProjectName
	require.NoError(t, err)
	err = app.Start()
	require.NoError(t, err)

	// Now change the project namename and look for the complaint
	app.Name = secondProjectName
	err = app.Start()
	assert.Error(err)
	assert.Contains(err.Error(), "already contains a project named "+originalProjectName)
	err = app.Stop(true, false)
	assert.NoError(err)

	// Change back to original name
	app.Name = originalProjectName
	assert.NoError(err)
	err = app.Start()
	assert.NoError(err)

	// Now stop and make sure the behavior is same with everything stopped
	err = app.Stop(false, false)
	assert.NoError(err)

	// Now change the project name again and look for the error
	app.Name = secondProjectName
	err = app.Start()
	assert.Error(err)
	assert.Contains(err.Error(), "already contains a project named "+originalProjectName)
}

// TestHttpsRedirection tests to make sure that webserver and php redirect to correct
// scheme (http or https).
func TestHttpsRedirection(t *testing.T) {
	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)
	testcommon.ClearDockerEnv()
	packageDir, _ := os.Getwd()

	testDir := testcommon.CreateTmpDir(t.Name())
	appDir := filepath.Join(testDir, t.Name())
	err := fileutil.CopyDir(filepath.Join(packageDir, "testdata", t.Name()), appDir)
	assert.NoError(err)
	err = os.Chdir(appDir)
	assert.NoError(err)

	app, err := ddevapp.NewApp(appDir, true)
	assert.NoError(err)

	_ = app.Stop(true, false)

	t.Cleanup(func() {
		err = app.Stop(true, false)
		assert.NoError(err)
		err = os.Chdir(packageDir)
		assert.NoError(err)
		err = os.RemoveAll(testDir)
		assert.NoError(err)
	})

	expectations := []URLRedirectExpectations{
		{"https", "/subdir", "/subdir/"},
		{"https", "/redir_abs.php", "/landed.php"},
		{"https", "/redir_relative.php", "/landed.php"},
		{"http", "/subdir", "/subdir/"},
		{"http", "/redir_abs.php", "/landed.php"},
		{"http", "/redir_relative.php", "/landed.php"},
	}

	types := ddevapp.GetValidAppTypes()
	webserverTypes := []string{nodeps.WebserverNginxFPM, nodeps.WebserverApacheFPM}
	if os.Getenv("GOTEST_SHORT") != "" {
		types = []string{nodeps.AppTypePHP, nodeps.AppTypeDrupal8}
		webserverTypes = []string{nodeps.WebserverNginxFPM, nodeps.WebserverApacheFPM}
	}
	for _, projectType := range types {
		// TODO: Fix the laravel config so it can do the redir_abs.php successfully on nginx-fpm
		if projectType == nodeps.AppTypeLaravel {
			t.Log("Skipping laravel because it can't pass absolute redirect test, fix config")
			continue
		}
		for _, webserverType := range webserverTypes {
			app.WebserverType = webserverType
			app.Type = projectType
			err = app.WriteConfig()
			assert.NoError(err)

			// Do a start on the configured site.
			app, err = ddevapp.GetActiveApp("")
			assert.NoError(err)
			startErr := app.StartAndWait(5)
			assert.NoError(startErr, "app.Start() failed with projectType=%s, webserverType=%s", projectType, webserverType)
			if startErr != nil {
				appLogs, getLogsErr := ddevapp.GetErrLogsFromApp(app, startErr)
				assert.NoError(getLogsErr)
				t.Fatalf("app.StartAndWait failure; err=%v \n===== container logs ==\n%s\n", startErr, appLogs)
			}
			// Test for directory redirects under https and http
			for _, parts := range expectations {
				reqURL := parts.scheme + "://" + strings.ToLower(app.GetHostname()) + parts.uri
				//t.Logf("TestHttpsRedirection trying URL %s with webserver_type=%s", reqURL, webserverType)
				out, resp, err := testcommon.GetLocalHTTPResponse(t, reqURL)
				require.NotNil(t, resp, "resp was nil for projectType=%s webserver_type=%s url=%s, err=%v, out='%s'", projectType, webserverType, reqURL, err, out)
				if resp != nil {
					locHeader := resp.Header.Get("Location")

					expectedRedirect := parts.expectedRedirectURI
					// However, if we're hitting redir_abs.php (or apache hitting directory), the redirect will be the whole url.
					if strings.Contains(parts.uri, "redir_abs.php") || webserverType != nodeps.WebserverNginxFPM {
						expectedRedirect = parts.scheme + "://" + strings.ToLower(app.GetHostname()) + parts.expectedRedirectURI
					}
					// Except the php relative redirect is always relative.
					if strings.Contains(parts.uri, "redir_relative.php") {
						expectedRedirect = parts.expectedRedirectURI
					}
					assert.EqualValues(locHeader, expectedRedirect, "For project type=%s webserver_type=%s url=%s expected redirect %s != actual %s", projectType, webserverType, reqURL, expectedRedirect, locHeader)
				}
			}
		}
	}
	// Change back to package dir. Lots of things will have to be cleaned up
	// in defers, and for windows we have to not be sitting in them.
	err = os.Chdir(packageDir)
	assert.NoError(err)
}

// TestMultipleComposeFiles checks to see if a set of docker-compose files gets
// properly loaded in the right order, with .ddev/.ddev-docker-compose*yaml first and
// with docker-compose.override.yaml last.
func TestMultipleComposeFiles(t *testing.T) {
	// Set up tests and give ourselves a working directory.
	assert := asrt.New(t)
	pwd, _ := os.Getwd()

	testDir := testcommon.CreateTmpDir(t.Name())
	//_ = os.Chdir(testDir)
	defer testcommon.CleanupDir(testDir)
	defer testcommon.Chdir(testDir)()

	err := fileutil.CopyDir(filepath.Join(pwd, "testdata", t.Name(), ".ddev"), filepath.Join(testDir, ".ddev"))
	assert.NoError(err)

	// Make sure that valid yaml files get properly loaded in the proper order
	app, err := ddevapp.NewApp(testDir, true)
	assert.NoError(err)
	//nolint: errcheck
	defer app.Stop(true, false)

	err = app.WriteConfig()
	assert.NoError(err)
	_, err = app.ReadConfig(true)
	require.NoError(t, err)
	err = app.WriteDockerComposeYAML()
	require.NoError(t, err)

	app, err = ddevapp.NewApp(testDir, true)
	assert.NoError(err)
	//nolint: errcheck
	defer app.Stop(true, false)

	desc, err := app.Describe(false)
	assert.NoError(err)
	_ = desc

	files, err := app.ComposeFiles()
	assert.NoError(err)
	require.NotEmpty(t, files)
	assert.Equal(4, len(files))
	require.Equal(t, app.GetConfigPath(".ddev-docker-compose-base.yaml"), files[0])
	require.Equal(t, app.GetConfigPath("docker-compose.override.yaml"), files[len(files)-1])

	require.NotEmpty(t, app.ComposeYaml)
	require.True(t, len(app.ComposeYaml) > 0)

	// Verify that the env var DUMMY_BASE got set by docker-compose.override.yaml
	if services, ok := app.ComposeYaml["services"].(map[interface{}]interface{}); ok {
		if w, ok := services["web"].(map[interface{}]interface{}); ok {
			if env, ok := w["environment"].(map[interface{}]interface{}); ok {
				// The docker-compose.override should have won with the value of DUMMY_BASE
				assert.Equal("override", env["DUMMY_BASE"])
				// But each of the DUMMY_COMPOSE_ONE/TWO/OVERRIDE which are unique
				// should come through fine.
				assert.Equal("1", env["DUMMY_COMPOSE_ONE"])
				assert.Equal("2", env["DUMMY_COMPOSE_TWO"])
				assert.Equal("override", env["DUMMY_COMPOSE_OVERRIDE"])
			} else {
				t.Error("Failed to parse environment")
			}
		} else {
			t.Error("failed to parse web service")
		}

	} else {
		t.Error("Unable to access ComposeYaml[services]")
	}

	_, err = app.ComposeFiles()
	assert.NoError(err)
}

// TestGetAllURLs ensures the GetAllURLs function returns the expected number of URLs,
// and include the direct web container URLs.
func TestGetAllURLs(t *testing.T) {
	assert := asrt.New(t)

	site := TestSites[0]
	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s GetAllURLs", site.Name))

	testcommon.ClearDockerEnv()
	app := new(ddevapp.DdevApp)

	err := app.Init(site.Dir)
	assert.NoError(err)

	// Add some additional hostnames
	app.AdditionalHostnames = []string{"sub1", "sub2", "sub3"}

	err = app.WriteConfig()
	assert.NoError(err)

	err = app.StartAndWait(0)
	require.NoError(t, err)

	_, _, urls := app.GetAllURLs()

	// Convert URLs to map[string]bool
	urlMap := make(map[string]bool)
	for _, u := range urls {
		urlMap[u] = true
	}

	// We expect two URLs for each hostname (http/https) and two direct web container address.
	expectedNumUrls := len(app.GetHostnames())*2 + 2
	assert.Equal(len(urlMap), expectedNumUrls, "Unexpected number of URLs returned: %d", len(urlMap))

	// Ensure urlMap contains direct address of the web container
	webContainer, err := app.FindContainerByType("web")
	assert.NoError(err)
	require.NotEmpty(t, webContainer)

	expectedDirectAddress := app.GetWebContainerDirectHTTPSURL()
	if globalconfig.GetCAROOT() == "" {
		expectedDirectAddress = app.GetWebContainerDirectHTTPURL()
	}

	exists := urlMap[expectedDirectAddress]

	assert.True(exists, "URL list for app: %s does not contain direct web container address: %s", app.Name, expectedDirectAddress)

	// Multiple projects can't run at the same time with the fqdns, so we need to clean
	// up these for tests that run later.
	app.AdditionalFQDNs = []string{}
	app.AdditionalHostnames = []string{}
	err = app.WriteConfig()
	assert.NoError(err)

	err = app.Stop(true, false)
	assert.NoError(err)

	runTime()
}

// TestWebserverType checks that webserver_type:apache-fpm does the right thing
func TestWebserverType(t *testing.T) {
	assert := asrt.New(t)

	for _, site := range TestSites {
		runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s TestWebserverType", site.Name))

		app := new(ddevapp.DdevApp)

		err := app.Init(site.Dir)
		assert.NoError(err)

		// Copy our phpinfo into the docroot of testsite.
		pwd, err := os.Getwd()
		assert.NoError(err)
		err = fileutil.CopyFile(filepath.Join(pwd, "testdata", "servertype.php"), filepath.Join(app.AppRoot, app.Docroot, "servertype.php"))

		assert.NoError(err)
		for _, app.WebserverType = range []string{nodeps.WebserverApacheFPM, nodeps.WebserverNginxFPM} {

			err = app.WriteConfig()
			assert.NoError(err)

			testcommon.ClearDockerEnv()

			startErr := app.StartAndWait(30)
			//nolint: errcheck
			defer app.Stop(true, false)
			if startErr != nil {
				appLogs, getLogsErr := ddevapp.GetErrLogsFromApp(app, startErr)
				assert.NoError(getLogsErr)
				t.Fatalf("app.StartAndWait failure for WebserverType=%s; site.Name=%s; err=%v, logs:\n=====\n%s\n=====\n", app.WebserverType, site.Name, startErr, appLogs)
			}
			out, resp, err := testcommon.GetLocalHTTPResponse(t, app.GetWebContainerDirectHTTPURL()+"/servertype.php")
			require.NoError(t, err)

			expectedServerType := "Apache/2"
			if app.WebserverType == nodeps.WebserverNginxFPM {
				expectedServerType = "nginx"
			}
			require.NotEmpty(t, resp.Header["Server"])
			require.NotEmpty(t, resp.Header["Server"][0])
			assert.Contains(resp.Header["Server"][0], expectedServerType, "Server header for project=%s, app.WebserverType=%s should be %s", app.Name, app.WebserverType, expectedServerType)
			assert.Contains(out, expectedServerType, "For app.WebserverType=%s phpinfo expected servertype.php to show %s", app.WebserverType, expectedServerType)
			err = app.Stop(true, false)
			assert.NoError(err)
		}

		// Set the apptype back to whatever the default was so we don't break any following tests.
		testVar := os.Getenv("DDEV_TEST_WEBSERVER_TYPE")
		if testVar != "" {
			app.WebserverType = testVar
			err = app.WriteConfig()
			assert.NoError(err)
		}
		runTime()
	}
}

// TestInternalAndExternalAccessToURL checks we can access content
// from host and from inside container by URL (with port)
func TestInternalAndExternalAccessToURL(t *testing.T) {
	assert := asrt.New(t)

	runTime := util.TimeTrack(time.Now(), t.Name())

	site := TestSites[0]
	app := new(ddevapp.DdevApp)

	err := app.Init(site.Dir)
	assert.NoError(err)

	// Add some additional hostnames
	app.AdditionalHostnames = []string{"sub1", "sub2", "sub3"}
	app.AdditionalFQDNs = []string{"junker99.example.com"}

	for _, pair := range []testcommon.PortPair{{HTTPPort: "80", HTTPSPort: "443"}, {HTTPPort: "8080", HTTPSPort: "8443"}} {
		testcommon.ClearDockerEnv()
		app.RouterHTTPPort = pair.HTTPPort
		app.RouterHTTPSPort = pair.HTTPSPort
		err = app.WriteConfig()
		assert.NoError(err)

		// Make sure that project is absolutely not running
		err = app.Stop(true, false)
		assert.NoError(err)

		err = app.StartAndWait(5)
		assert.NoError(err)

		_, _, urls := app.GetAllURLs()

		// Convert URLs to map[string]bool
		urlMap := make(map[string]bool)
		for _, u := range urls {
			urlMap[u] = true
		}

		// We expect two URLs for each hostname (http/https) and two direct web container addresses.
		expectedNumUrls := len(app.GetHostnames())*2 + 2
		assert.Equal(len(urlMap), expectedNumUrls, "Unexpected number of URLs returned: %d", len(urlMap))

		_, _, URLList := app.GetAllURLs()
		URLList = append(URLList, "http://localhost", "http://localhost")
		for _, item := range URLList {
			// Make sure internal (web container) access is successful
			parts, err := url.Parse(item)
			require.NoError(t, err, "url.Parse of item=%v failed", item)
			require.NotNil(t, parts, "url.Parse of item=%v failed", item)
			// Only try it if not an IP address URL; those won't be right
			hostParts := strings.Split(parts.Host, ".")

			// Make sure access from host is successful
			// But "localhost" is only for inside container.
			if parts.Host != "localhost" {
				_, _ = testcommon.EnsureLocalHTTPContent(t, item+site.Safe200URIWithExpectation.URI, site.Safe200URIWithExpectation.Expect)
			}

			if _, err := strconv.ParseInt(hostParts[0], 10, 64); err != nil {
				out, _, err := app.Exec(&ddevapp.ExecOpts{
					Service: "web",
					Cmd:     "curl -sS --fail " + item + site.Safe200URIWithExpectation.URI,
				})
				assert.NoError(err, "failed curl to %s: %v", item+site.Safe200URIWithExpectation.URI, err)
				assert.Contains(out, site.Safe200URIWithExpectation.Expect)
			}
		}
	}

	out, err := exec.RunCommand(DdevBin, []string{"list"})
	assert.NoError(err)
	t.Logf("\n=========== output of ddev list ==========\n%s\n============\n", out)
	out, err = exec.RunCommand("docker", []string{"logs", "ddev-router"})
	assert.NoError(err)
	t.Logf("\n=========== output of docker logs ddev-router ==========\n%s\n============\n", out)

	// Set the ports back to the default was so we don't break any following tests.
	app.RouterHTTPSPort = "443"
	app.RouterHTTPPort = "80"
	app.AdditionalFQDNs = []string{}
	app.AdditionalHostnames = []string{}

	err = app.WriteConfig()
	assert.NoError(err)
	err = app.Stop(true, false)
	assert.NoError(err)

	runTime()
}

// TestCaptureLogs checks that app.CaptureLogs() works
func TestCaptureLogs(t *testing.T) {
	assert := asrt.New(t)

	site := TestSites[0]
	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s CaptureLogs", site.Name))

	app := ddevapp.DdevApp{}

	err := app.Init(site.Dir)
	assert.NoError(err)
	err = app.Start()
	assert.NoError(err)

	logs, err := app.CaptureLogs("web", false, "100")
	assert.NoError(err)

	assert.Contains(logs, "INFO spawned")

	err = app.Stop(true, false)
	assert.NoError(err)

	runTime()
}

// TestNFSMount tests ddev start functionality with nfs_mount_enabled: true
// This requires that the test machine must have NFS shares working
// Tests using both app-specific nfs_mount_enabled and global nfs_mount_enabled
func TestNFSMount(t *testing.T) {
	if nodeps.IsWSL2() {
		t.Skip("Skipping on WSL2")
	}
	if runtime.GOARCH == "arm64" && runtime.GOOS == "darwin" {
		t.Skip("Temporarily skipping on mac M1 because NFS has some trouble")
	}

	assert := asrt.New(t)
	app := &ddevapp.DdevApp{}

	// Make sure this leaves us in the original test directory
	testDir, _ := os.Getwd()
	//nolint: errcheck
	defer os.Chdir(testDir)

	site := TestSites[0]
	switchDir := site.Chdir()
	runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s %s", site.Name, t.Name()))

	err := app.Init(site.Dir)
	assert.NoError(err)

	defer func() {
		globalconfig.DdevGlobalConfig.NFSMountEnabledGlobal = false
		_ = globalconfig.WriteGlobalConfig(globalconfig.DdevGlobalConfig)
		app.NFSMountEnabled = false
		_ = app.WriteConfig()
		_ = app.Stop(true, false)
	}()

	t.Log("testing with global NFSMountEnabled")
	globalconfig.DdevGlobalConfig.NFSMountEnabledGlobal = true
	err = globalconfig.WriteGlobalConfig(globalconfig.DdevGlobalConfig)
	assert.NoError(err)

	// Run NewApp so that it picks up the global config, as it would in real life
	app, err = ddevapp.NewApp(site.Dir, false)
	assert.NoError(err)
	verifyNFSMount(t, app)

	t.Log("testing with app NFSMountEnabled")
	globalconfig.DdevGlobalConfig.NFSMountEnabledGlobal = false
	err = globalconfig.WriteGlobalConfig(globalconfig.DdevGlobalConfig)
	assert.NoError(err)

	// Run NewApp so that it picks up the global config, as it would in real life
	app, err = ddevapp.NewApp(site.Dir, false)
	assert.NoError(err)

	app.NFSMountEnabled = true
	verifyNFSMount(t, app)

	runTime()
	switchDir()
}

func verifyNFSMount(t *testing.T, app *ddevapp.DdevApp) {
	assert := asrt.New(t)

	err := app.Stop(true, false)
	assert.NoError(err)
	err = app.Start()
	//nolint: errcheck
	defer app.Stop(true, false)
	require.NoError(t, err)

	stdout, _, err := app.Exec(&ddevapp.ExecOpts{
		Service: "web",
		Dir:     "/var/www/html",
		Cmd:     "findmnt -T .",
	})
	assert.NoError(err)

	source := app.AppRoot
	if runtime.GOOS == "darwin" && fileutil.IsDirectory(filepath.Join("/System/Volumes/Data", app.AppRoot)) {
		source = filepath.Join("/System/Volumes/Data", app.AppRoot)
	}
	assert.Contains(stdout, ":"+dockerutil.MassageWindowsNFSMount(source))

	// Create a host-side dir symlink; give a second for it to sync, make sure it can be used in container.
	err = os.Symlink(".ddev", "nfslinked_.ddev")
	assert.NoError(err)
	// nolint: errcheck
	defer os.Remove("nfslinked_.ddev")

	time.Sleep(2 * time.Second)
	_, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "web",
		Dir:     "/var/www/html",
		Cmd:     "ls nfslinked_.ddev/config.yaml",
	})
	assert.NoError(err)

	// Create a host-side file symlink; give a second for it to sync, make sure it can be used in container.
	err = os.Symlink(".ddev/config.yaml", "nfslinked_config.yaml")
	assert.NoError(err)
	// nolint: errcheck
	defer os.Remove("nfslinked_config.yaml")

	time.Sleep(2 * time.Second)
	_, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "web",
		Dir:     "/var/www/html",
		Cmd:     "ls nfslinked_config.yaml",
	})
	assert.NoError(err)

	// Create a container-side dir symlink; give a second for it to sync, make sure it can be used on host.
	_, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "web",
		Dir:     "/var/www/html",
		Cmd:     "ln -s  .ddev nfscontainerlinked_ddev",
	})
	assert.NoError(err)

	// nolint: errcheck
	defer os.Remove("nfscontainerlinked_ddev")

	time.Sleep(2 * time.Second)
	assert.FileExists("nfscontainerlinked_ddev/config.yaml")

	// Create a container-side file symlink; give a second for it to sync, make sure it can be used on host.
	_, _, err = app.Exec(&ddevapp.ExecOpts{
		Service: "web",
		Dir:     "/var/www/html",
		Cmd:     "ln -s  .ddev/config.yaml nfscontainerlinked_config.yaml",
	})
	assert.NoError(err)

	// nolint: errcheck
	defer os.Remove("nfscontainerlinked_config.yaml")

	time.Sleep(2 * time.Second)
	assert.FileExists("nfscontainerlinked_config.yaml")
}

// TestHostDBPort tests to make sure that the host_db_port specification has the intended effect
func TestHostDBPort(t *testing.T) {
	assert := asrt.New(t)
	runTime := util.TimeTrack(time.Now(), t.Name())
	defer runTime()
	testDir, _ := os.Getwd()

	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()

	app, err := ddevapp.NewApp(site.Dir, false)
	assert.NoError(err)

	showportPath := app.GetConfigPath("commands/host/showport")
	err = os.MkdirAll(filepath.Dir(showportPath), 0755)
	assert.NoError(err)
	err = fileutil.CopyFile(filepath.Join(testDir, "testdata", t.Name(), "showport"), showportPath)
	assert.NoError(err)

	defer func() {
		_ = os.RemoveAll(showportPath)
		_ = app.Stop(true, false)
	}()

	// Make sure that everything works with and without
	// an explicitly specified hostDBPort
	for _, hostDBPort := range []string{"", "9998"} {
		app.HostDBPort = hostDBPort

		err = app.Start()
		require.NoError(t, err)

		desc, err := app.Describe(false)
		assert.NoError(err)

		dockerIP, err := dockerutil.GetDockerIP()
		assert.NoError(err)
		dbinfo := desc["dbinfo"].(map[string]interface{})
		dbPort := dbinfo["published_port"].(int)
		dbPortStr := strconv.Itoa(dbPort)

		if app.HostDBPort != "" {
			assert.EqualValues(app.HostDBPort, dbPortStr)
		}

		if !util.IsCommandAvailable("mysql") {
			t.Log("Skipping mysql check because mysql tool not available")
		} else {
			// Running mysql against the container ensures that we can get there via the values
			// in ddev describe
			out, err := exec.RunCommand("mysql", []string{"--user=db", "--password=db", "--host=" + dockerIP, fmt.Sprintf("--port=%d", dbPort), "--database=db", `--execute=SELECT 1;`})
			assert.NoError(err, "Failed to run mysql: %v", out)
			out = strings.Replace(out, "\r", "", -1)
			assert.Contains(out, "1\n1\n")
		}

		// Running the test host custom command "showport" ensures that the DDEV_HOST_DB_PORT
		// is getting in there available to host custom commands.
		_, _ = exec.RunCommand(DdevBin, []string{})
		out, err := exec.RunCommand(DdevBin, []string{"showport"})
		assert.NoError(err)
		assert.EqualValues("DDEV_HOST_DB_PORT="+dbPortStr, strings.Trim(out, "\n"))
	}
}

// TestPortSpecifications tests to make sure that one project can't step on the
// ports used by another
func TestPortSpecifications(t *testing.T) {
	if nodeps.IsWSL2() {
		t.Skip("Skipping on WSL2 because of inconsistent docker behavior acquiring ports")
	}
	assert := asrt.New(t)
	runTime := util.TimeTrack(time.Now(), fmt.Sprint("TestPortSpecifications"))
	defer runTime()
	testDir, _ := os.Getwd()

	site0 := TestSites[0]
	switchDir := site0.Chdir()
	defer switchDir()

	nospecApp := ddevapp.DdevApp{}
	err := nospecApp.Init(site0.Dir)
	assert.NoError(err)
	err = nospecApp.WriteConfig()
	require.NoError(t, err)
	// Since host ports were not explicitly set in nospecApp, they shouldn't be in globalconfig.
	require.Empty(t, globalconfig.DdevGlobalConfig.ProjectList[nospecApp.Name].UsedHostPorts)

	err = nospecApp.Start()
	assert.NoError(err)

	// Now that we have a working nospecApp with unspecified ephemeral ports, test that we
	// can't use those ports while nospecApp is running

	_ = os.Chdir(testDir)
	ddevDir, _ := filepath.Abs("./testdata/TestPortSpecifications/.ddev")

	specAppPath := testcommon.CreateTmpDir(t.Name() + "_specapp")

	err = fileutil.CopyDir(ddevDir, filepath.Join(specAppPath, ".ddev"))
	require.NoError(t, err, "could not copy to specAppPath %v", specAppPath)

	specAPP, err := ddevapp.NewApp(specAppPath, false)
	assert.NoError(err)

	t.Cleanup(func() {
		_ = specAPP.Stop(true, false)
		err = os.RemoveAll(specAppPath)
		assert.NoError(err)
	})

	// It should be able to WriteConfig and Start with the configured host ports it came up with
	err = specAPP.WriteConfig()
	assert.NoError(err)
	err = specAPP.Start()
	assert.NoError(err)
	//nolint: errcheck
	err = specAPP.Stop(false, false)
	require.NoError(t, err)
	// Verify that DdevGlobalConfig got updated properly
	require.NotEmpty(t, globalconfig.DdevGlobalConfig.ProjectList[specAPP.Name])
	require.NotEmpty(t, globalconfig.DdevGlobalConfig.ProjectList[specAPP.Name].UsedHostPorts)

	// However, if we change change the name to make it appear to be a
	// different project, we should not be able to config or start
	conflictApp, err := ddevapp.NewApp(specAppPath, false)
	assert.NoError(err)
	conflictApp.Name = "conflictapp"

	t.Cleanup(func() {
		err = conflictApp.Stop(true, false)
		assert.NoError(err)
	})
	err = conflictApp.WriteConfig()
	assert.Error(err)
	err = conflictApp.Start()
	assert.Error(err, "Expected error starting conflictApp=%v", conflictApp)

	// Now delete the specAPP and we should be able to use the conflictApp
	err = specAPP.Stop(true, false)
	assert.NoError(err)
	assert.Empty(globalconfig.DdevGlobalConfig.ProjectList[specAPP.Name])

	err = conflictApp.WriteConfig()
	assert.NoError(err)
	err = conflictApp.Start()
	assert.NoError(err)

	require.NotEmpty(t, globalconfig.DdevGlobalConfig.ProjectList[conflictApp.Name])
	require.NotEmpty(t, globalconfig.DdevGlobalConfig.ProjectList[conflictApp.Name].UsedHostPorts)
}

// TestDdevGetProjects exercises GetProjects()
// It's only here for profiling at this point
func TestDdevGetProjects(t *testing.T) {
	assert := asrt.New(t)
	runTime := util.TimeTrack(time.Now(), fmt.Sprint(t.Name()))
	defer runTime()

	apps, err := ddevapp.GetProjects(false)
	assert.NoError(err)
	_ = apps

}

// TestCustomCerts makes sure that added custom certificates are respected and used
func TestCustomCerts(t *testing.T) {
	assert := asrt.New(t)

	// Force router stop - shouldn't be necessary
	//dest := ddevapp.RouterComposeYAMLPath()
	//_, _, err := dockerutil.ComposeCmd([]string{dest}, "-p", ddevapp.RouterProjectName, "down")
	//assert.NoError(err)

	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()

	app, err := ddevapp.NewApp(site.Dir, false)
	assert.NoError(err)

	certDir := app.GetConfigPath("custom_certs")
	err = os.MkdirAll(certDir, 0755)
	assert.NoError(err)

	t.Cleanup(func() {
		_ = os.RemoveAll(certDir)
		_, _, err = app.Exec(&ddevapp.ExecOpts{
			Cmd: "rm /mnt/ddev-global-cache/custom_certs/" + app.GetHostname() + "*",
		})
		assert.NoError(err)
		err = app.Stop(true, false)
		assert.NoError(err)
	})

	// Start without cert and make sure normal DNS names are there
	err = app.Start()
	assert.NoError(err)
	stdout, _, err := app.Exec(&ddevapp.ExecOpts{
		Cmd: fmt.Sprintf("openssl s_client -connect %s:443 -servername %s </dev/null 2>/dev/null | openssl x509 -noout -text | perl -l -0777 -ne '@names=/\\bDNS:([^\\s,]+)/g; print join(\"\\n\", sort @names);'", app.GetHostname(), app.GetHostname()),
	})
	stdout = strings.Trim(stdout, "\n")
	// This should be our regular wildcard cert
	assert.Contains(stdout, "*.ddev.site")

	// Now stop it so we can install new custom cert.
	err = app.Stop(true, false)
	assert.NoError(err)

	// Create a certfile/key in .ddev/custom_certs with just one DNS name in it
	// mkcert --cert-file d9composer.ddev.site.crt --key-file d9composer.ddev.site.key d9composer.ddev.site
	out, err := exec.RunCommand("mkcert", []string{"--cert-file", filepath.Join(certDir, app.GetHostname()+".crt"), "--key-file", filepath.Join(certDir, app.GetHostname()+".key"), app.GetHostname()})
	assert.NoError(err, "mkcert command failed, out=%s", out)

	err = app.Start()
	assert.NoError(err)

	stdout, _, err = app.Exec(&ddevapp.ExecOpts{
		Cmd: fmt.Sprintf("openssl s_client -connect %s:443 -servername %s </dev/null 2>/dev/null | openssl x509 -noout -text | perl -l -0777 -ne '@names=/\\bDNS:([^\\s,]+)/g; print join(\"\\n\", sort @names);'", app.GetHostname(), app.GetHostname()),
	})
	stdout = strings.Trim(stdout, "\n")
	// If we had the regular cert, there would be several things here including *.ddev.site
	// But e should only see the hostname listed.
	assert.Equal(app.GetHostname(), stdout)
}

// TestDdevList tests the ddevapp.List() functionality
// It's only here for profiling at this point.
func TestDdevList(t *testing.T) {
	ddevapp.List(true, false, 1)
}

// TestEnvironmentVariables tests to make sure that documented environment variables appear
// in the web container and on the host.
func TestEnvironmentVariables(t *testing.T) {
	assert := asrt.New(t)
	pwd, _ := os.Getwd()
	customCmd := filepath.Join(pwd, "testdata", t.Name(), "showhostenvvar")
	site := TestSites[0]
	switchDir := site.Chdir()
	defer switchDir()

	app, err := ddevapp.NewApp(site.Dir, false)
	assert.NoError(err)
	customCmdDest := app.GetConfigPath("commands/host/" + "showhostenvvar")

	err = os.MkdirAll(filepath.Dir(customCmdDest), 0755)
	require.NoError(t, err)
	err = fileutil.CopyFile(customCmd, customCmdDest)
	require.NoError(t, err)

	// This set of webContainerExpectations should be maintained to match the list in the docs
	webContainerExpectations := map[string]string{
		"DDEV_DOCROOT":           app.GetDocroot(),
		"DDEV_HOSTNAME":          app.GetHostname(),
		"DDEV_PHP_VERSION":       app.PHPVersion,
		"DDEV_PRIMARY_URL":       app.GetPrimaryURL(),
		"DDEV_PROJECT":           app.Name,
		"DDEV_PROJECT_TYPE":      app.Type,
		"DDEV_ROUTER_HTTP_PORT":  app.RouterHTTPPort,
		"DDEV_ROUTER_HTTPS_PORT": app.RouterHTTPSPort,
		"DDEV_SITENAME":          app.Name,
		"DDEV_TLD":               app.ProjectTLD,
		"DDEV_WEBSERVER_TYPE":    app.WebserverType,
	}

	err = app.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		err = os.RemoveAll(customCmdDest)
		assert.NoError(err)
		err = app.Stop(true, false)
		assert.NoError(err)
	})

	for k, v := range webContainerExpectations {
		envVal, _, err := app.Exec(&ddevapp.ExecOpts{
			Cmd: fmt.Sprintf("echo ${%s}", k),
		})
		assert.NoError(err)
		envVal = strings.Trim(envVal, "\n")
		assert.Equal(v, envVal)
	}

	dbPort, err := app.GetPublishedPort("db")
	dbPortStr := strconv.Itoa(dbPort)
	if dbPortStr == "-1" || err != nil {
		dbPortStr = ""
	}
	if app.HostDBPort != "" {
		dbPortStr = app.HostDBPort
	}

	// This set of hostExpections should bne maintained in parallel with documentation
	hostExpectations := map[string]string{
		"DDEV_APPROOT":             app.AppRoot,
		"DDEV_DOCROOT":             app.GetDocroot(),
		"DDEV_HOST_DB_PORT":        dbPortStr,
		"DDEV_HOST_HTTPS_PORT":     app.HostHTTPSPort,
		"DDEV_HOST_WEBSERVER_PORT": app.HostWebserverPort,
		"DDEV_HOSTNAME":            app.GetHostname(),
		"DDEV_PHP_VERSION":         app.PHPVersion,
		"DDEV_PRIMARY_URL":         app.GetPrimaryURL(),
		"DDEV_PROJECT":             app.Name,
		"DDEV_PROJECT_TYPE":        app.Type,
		"DDEV_ROUTER_HTTP_PORT":    app.RouterHTTPPort,
		"DDEV_ROUTER_HTTPS_PORT":   app.RouterHTTPSPort,
		"DDEV_SITENAME":            app.Name,
		"DDEV_TLD":                 app.ProjectTLD,
		"DDEV_WEBSERVER_TYPE":      app.WebserverType,
	}
	for k, v := range hostExpectations {
		envVal, err := exec.RunCommand(DdevBin, []string{"showhostenvvar", k})
		assert.NoError(err, "could not run %s %s %s, result=%s", DdevBin, "showhostenvvar", k, envVal)
		envVal = strings.Trim(envVal, "\n")
		assert.Equal(v, envVal, "expected envvar $%s to equal '%s', but it was '%s'", k, v, envVal)
	}

}

// constructContainerName builds a container name given the type (web/db/dba) and the app
func constructContainerName(containerType string, app *ddevapp.DdevApp) (string, error) {
	container, err := app.FindContainerByType(containerType)
	if err != nil {
		return "", err
	}
	if container == nil {
		return "", fmt.Errorf("No container exists for containerType=%s app=%v", containerType, app)
	}
	name := dockerutil.ContainerName(*container)
	return name, nil
}

func removeAllErrCheck(path string, assert *asrt.Assertions) {
	err := os.RemoveAll(path)
	assert.NoError(err)
}
