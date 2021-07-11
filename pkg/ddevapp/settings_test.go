package ddevapp_test

import (
	"github.com/drud/ddev/pkg/nodeps"
	"github.com/drud/ddev/pkg/util"
	"path/filepath"
	"testing"

	"os"

	"fmt"

	"time"

	. "github.com/drud/ddev/pkg/ddevapp"
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/testcommon"
	asrt "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type settingsLocations struct {
	main  string
	local string
}

var drupalBackdropSettingsLocations = map[string]settingsLocations{
	nodeps.AppTypeDrupal6:  {main: "sites/default/settings.php", local: "sites/default/settings.ddev.php"},
	nodeps.AppTypeDrupal7:  {main: "sites/default/settings.php", local: "sites/default/settings.ddev.php"},
	nodeps.AppTypeDrupal8:  {main: "sites/default/settings.php", local: "sites/default/settings.ddev.php"},
	nodeps.AppTypeDrupal9:  {main: "sites/default/settings.php", local: "sites/default/settings.ddev.php"},
	nodeps.AppTypeBackdrop: {main: "settings.php", local: "settings.ddev.php"},
}

// TestWriteSettings tests writing app settings (like Drupal
// settings.php/settings.local.php
func TestWriteSettings(t *testing.T) {
	assert := asrt.New(t)

	expectations := map[string]string{
		nodeps.AppTypeBackdrop:  "settings.ddev.php",
		nodeps.AppTypeDrupal6:   "sites/default/settings.ddev.php",
		nodeps.AppTypeDrupal7:   "sites/default/settings.ddev.php",
		nodeps.AppTypeDrupal8:   "sites/default/settings.ddev.php",
		nodeps.AppTypeDrupal9:   "sites/default/settings.ddev.php",
		nodeps.AppTypeWordPress: "wp-config-ddev.php",
		nodeps.AppTypeTYPO3:     "typo3conf/AdditionalConfiguration.php",
	}
	dir := testcommon.CreateTmpDir(t.Name())

	app, err := NewApp(dir, true)
	assert.NoError(err)

	err = os.MkdirAll(filepath.Join(dir, app.Docroot, "sites", "default"), 0777)
	assert.NoError(err)

	err = os.MkdirAll(filepath.Join(dir, app.Docroot, "typo3conf"), 0777)
	assert.NoError(err)

	// TYPO3 wants LocalConfiguration.php to exist in the repo ahead of time.
	err = os.WriteFile(filepath.Join(dir, app.Docroot, "typo3conf", "LocalConfiguration.php"), []byte("<?php\n"), 0644)
	assert.NoError(err)

	for apptype, settingsRelativePath := range expectations {
		app.Type = apptype

		expectedSettingsFile := filepath.Join(dir, settingsRelativePath)
		_, err = os.Stat(expectedSettingsFile)
		assert.True(os.IsNotExist(err))
		createdFile, err := app.CreateSettingsFile()
		assert.NoError(err)
		assert.EqualValues(expectedSettingsFile, createdFile)
		_, err = os.Stat(expectedSettingsFile)
		assert.NoError(err)
		signatureFound, err := fileutil.FgrepStringInFile(expectedSettingsFile, DdevFileSignature)
		assert.NoError(err)
		assert.True(signatureFound, "Failed to find %s in %s", DdevFileSignature, expectedSettingsFile)
		err = os.Remove(expectedSettingsFile)
		assert.NoError(err)
	}

	err = os.RemoveAll(dir)
	assert.NoError(err)
	println("") // Just lets Goland find the PASS when done.
}

// TestWriteDrushConfig test the drush config we write
func TestWriteDrushConfig(t *testing.T) {
	assert := asrt.New(t)
	app := &DdevApp{}

	for _, site := range TestSites {
		switchDir := site.Chdir()
		runTime := util.TimeTrack(time.Now(), fmt.Sprintf("%s WriteDrushrc", site.Name))

		testcommon.ClearDockerEnv()

		err := app.Init(site.Dir)
		assert.NoError(err)

		_, err = app.CreateSettingsFile()
		assert.NoError(err)

		startErr := app.Start()
		//nolint: errcheck
		defer app.Stop(true, false)
		if startErr != nil {
			logs, _ := GetErrLogsFromApp(app, startErr)
			t.Fatalf("app.Start failed, startErr=%v, logs=\n========\n%s\n===========\n", startErr, logs)
		}

		drushFilePath := filepath.Join(filepath.Dir(app.SiteSettingsPath), "drushrc.php")

		switch app.Type {
		case nodeps.AppTypeDrupal6, nodeps.AppTypeDrupal7, nodeps.AppTypeBackdrop:
			require.True(t, fileutil.FileExists(drushFilePath))
			optionFound, err := fileutil.FgrepStringInFile(drushFilePath, "options")
			assert.NoError(err)
			assert.True(optionFound)

		default:
			assert.False(fileutil.FileExists(drushFilePath), "Drush settings file (%s) should not exist but it does (app.Type=%s)", drushFilePath, app.Type)
		}

		runTime()
		switchDir()
	}
}

// TestDrupalBackdropIncludeSettingsDdevInNewSettingsFile verifies that when no settings.php file exists,
// a settings.php file is created that includes settings.ddev.php.
func TestDrupalBackdropIncludeSettingsDdevInNewSettingsFile(t *testing.T) {
	assert := asrt.New(t)

	dir := testcommon.CreateTmpDir(t.Name())

	app, err := NewApp(dir, true)
	assert.NoError(err)

	err = os.MkdirAll(filepath.Join(dir, app.Docroot, "sites", "default"), 0777)
	assert.NoError(err)

	for appType, relativeSettingsLocations := range drupalBackdropSettingsLocations {
		app.Type = appType

		relativeSettingsLocation := relativeSettingsLocations.main
		relativeSettingsDdevLocation := relativeSettingsLocations.local
		expectedSettingsLocation := filepath.Join(dir, relativeSettingsLocation)
		expectedSettingsDdevLocation := filepath.Join(dir, relativeSettingsDdevLocation)

		// Ensure that no settings.php exists
		_ = os.Remove(expectedSettingsLocation)

		// Ensure that no settings.ddev.php file exists
		_ = os.Remove(expectedSettingsDdevLocation)

		// Invoke the settings file creation process
		_, err := app.CreateSettingsFile()
		assert.NoError(err)

		// Ensure that a settings.php was created
		assert.True(fileutil.FileExists(expectedSettingsLocation))

		// Ensure that settings.php references settings.ddev.php
		settingsDdev := filepath.Base(relativeSettingsDdevLocation)
		newSettingsIncludesSettingsDdev, err := fileutil.FgrepStringInFile(expectedSettingsLocation, settingsDdev)
		assert.NoError(err)
		assert.True(newSettingsIncludesSettingsDdev, "Failed to find %s in %s", settingsDdev, expectedSettingsLocation)

		// Ensure that settings.ddev.php exists
		assert.True(fileutil.FileExists(expectedSettingsDdevLocation))
	}
}

// TestDrupalBackdropIncludeSettingsDdevInExistingSettingsFile verifies that when a settings.php file already exists,
// it is modified to include settings.ddev.php
func TestDrupalBackdropIncludeSettingsDdevInExistingSettingsFile(t *testing.T) {
	assert := asrt.New(t)

	dir := testcommon.CreateTmpDir(t.Name())

	app, err := NewApp(dir, true)
	assert.NoError(err)

	err = os.MkdirAll(filepath.Join(dir, app.Docroot, "sites", "default"), 0777)
	assert.NoError(err)

	for appType, relativeSettingsLocations := range drupalBackdropSettingsLocations {
		app.Type = appType

		relativeSettingsLocation := relativeSettingsLocations.main
		relativeSettingsDdevLocation := relativeSettingsLocations.local
		expectedSettingsLocation := filepath.Join(dir, relativeSettingsLocation)
		expectedSettingsDdevLocation := filepath.Join(dir, relativeSettingsDdevLocation)

		// Ensure that no settings.php exists
		_ = os.Remove(expectedSettingsLocation)

		// Ensure that no settings.ddev.php file exists
		_ = os.Remove(expectedSettingsDdevLocation)

		// Create a settings.php that does not include settings.ddev.php
		originalContents := "// this file is not empty\n"
		err = os.WriteFile(expectedSettingsLocation, []byte(originalContents), 0644)
		assert.NoError(err)

		// Invoke the settings file creation process
		_, err = app.CreateSettingsFile()
		assert.NoError(err)

		// Ensure that settings.php exists
		assert.True(fileutil.FileExists(expectedSettingsLocation))

		// Ensure that settings.ddev.php exists
		assert.True(fileutil.FileExists(expectedSettingsDdevLocation))

		// Ensure that settings.php references settings.ddev.php
		settingsDdev := filepath.Base(relativeSettingsDdevLocation)
		existingSettingsIncludesSettingsDdev, err := fileutil.FgrepStringInFile(expectedSettingsLocation, settingsDdev)
		assert.NoError(err)
		assert.True(existingSettingsIncludesSettingsDdev, "Failed to find %s in %s, apptype=%s", settingsDdev, expectedSettingsLocation, appType)

		// Ensure that settings.php includes original contents
		modifiedSettingsIncludesOriginalContents, err := fileutil.FgrepStringInFile(expectedSettingsLocation, originalContents)
		assert.NoError(err)
		assert.True(modifiedSettingsIncludesOriginalContents, "Failed to find %s in %s", originalContents, expectedSettingsLocation)
	}
}

// TestDrupalBackdropCreateGitIgnoreIfNoneExists verifies that if no .gitignore file exists in the directory
// containing settings.php and settings.ddev.php, a .gitignore is created that includes settings.ddev.php.
func TestDrupalBackdropCreateGitIgnoreIfNoneExists(t *testing.T) {
	assert := asrt.New(t)

	dir := testcommon.CreateTmpDir(t.Name())

	app, err := NewApp(dir, true)
	assert.NoError(err)

	err = os.MkdirAll(filepath.Join(dir, app.Docroot, "sites", "default"), 0777)
	assert.NoError(err)

	for appType, relativeSettingsLocations := range drupalBackdropSettingsLocations {
		app.Type = appType

		relativeSettingsDdevLocation := relativeSettingsLocations.local
		expectedSettingsDdevLocation := filepath.Join(dir, relativeSettingsDdevLocation)
		expectedGitIgnoreLocation := filepath.Join(filepath.Dir(expectedSettingsDdevLocation), ".gitignore")
		fmt.Println(expectedGitIgnoreLocation)

		// Ensure that no .gitignore exists
		_ = os.Remove(expectedGitIgnoreLocation)

		// Invoke the settings file creation process
		_, err = app.CreateSettingsFile()
		assert.NoError(err)

		// Ensure that a .gitignore exists (except for backdrop, which has settings in project root)
		if app.Type != nodeps.AppTypeBackdrop {
			assert.True(fileutil.FileExists(expectedGitIgnoreLocation))

			// Ensure that the new .gitignore includes settings.ddev.php
			settingsDdev := filepath.Base(relativeSettingsDdevLocation)
			newGitIgnoreIncludesSettingsDdev, err := fileutil.FgrepStringInFile(expectedGitIgnoreLocation, settingsDdev)
			assert.NoError(err)
			assert.True(newGitIgnoreIncludesSettingsDdev, "Failed to find %s in %s", settingsDdev, expectedGitIgnoreLocation)
		}
	}
}

// TestDrupalBackdropGitIgnoreAlreadyExists verifies that if a .gitignore already exists in the directory
// containing settings.php and settings.ddev.php, it is not modified.
func TestDrupalBackdropGitIgnoreAlreadyExists(t *testing.T) {
	assert := asrt.New(t)

	dir := testcommon.CreateTmpDir(t.Name())

	app, err := NewApp(dir, true)
	assert.NoError(err)

	err = os.MkdirAll(filepath.Join(dir, app.Docroot, "sites", "default"), 0777)
	assert.NoError(err)

	for appType, relativeSettingsLocations := range drupalBackdropSettingsLocations {
		app.Type = appType

		relativeSettingsDdevLocation := relativeSettingsLocations.local
		expectedSettingsDdevLocation := filepath.Join(dir, relativeSettingsDdevLocation)
		expectedGitIgnoreLocation := filepath.Join(filepath.Dir(expectedSettingsDdevLocation), ".gitignore")
		fmt.Println(expectedGitIgnoreLocation)

		// Ensure that a .gitignore already exists and has some contents
		originalContents := "not empty"
		settingsFile, err := os.Create(expectedGitIgnoreLocation)
		assert.NoError(err)
		_, err = settingsFile.Write([]byte(originalContents))
		assert.NoError(err)

		// Invoke the settings file creation process
		_, err = app.CreateSettingsFile()
		assert.NoError(err)

		// Ensure that .gitignore still exists
		assert.True(fileutil.FileExists(expectedGitIgnoreLocation))

		// Ensure that the new .gitignore has not been modified to include settings.ddev.php
		settingsDdev := relativeSettingsDdevLocation
		existingGitIgnoreIncludesSettingsDdev, err := fileutil.FgrepStringInFile(expectedGitIgnoreLocation, settingsDdev)
		assert.NoError(err)
		assert.False(existingGitIgnoreIncludesSettingsDdev, "Found unexpected %s in %s", settingsDdev, expectedGitIgnoreLocation)
	}
}

// TestDrupalBackdropOverwriteDdevSettings ensures that if a settings.ddev.php file already exists, it is overwritten by the
// settings creation process.
func TestDrupalBackdropOverwriteDdevSettings(t *testing.T) {
	assert := asrt.New(t)

	dir := testcommon.CreateTmpDir(t.Name())

	app, err := NewApp(dir, true)
	assert.NoError(err)

	err = os.MkdirAll(filepath.Join(dir, app.Docroot, "sites", "default"), 0777)
	assert.NoError(err)

	for appType, relativeSettingsLocations := range drupalBackdropSettingsLocations {
		app.Type = appType

		relativeSettingsDdevLocation := relativeSettingsLocations.local
		expectedSettingsDdevLocation := filepath.Join(dir, relativeSettingsDdevLocation)

		// Ensure that a settings.ddev.php file exists, WITH the #ddev-generated signature
		originalContents := "not empty " + DdevFileSignature
		settingsFile, err := os.Create(expectedSettingsDdevLocation)
		assert.NoError(err)
		_, err = settingsFile.Write([]byte(originalContents))
		assert.NoError(err)

		// Invoke the settings file creation process
		_, err = app.CreateSettingsFile()
		assert.NoError(err)

		// Ensure settings.ddev.php was overwritten; It had the signature in it
		// so it was valid to overwrite. The original string should no longer be there.
		containsOriginalString, err := fileutil.FgrepStringInFile(expectedSettingsDdevLocation, originalContents)
		assert.NoError(err)
		assert.False(containsOriginalString, "The file should not have contained the original string %s and it did not.", originalContents)

		// Now do the whole thing again, but this time the settings.ddev.php does *not* have
		// the #ddev-generated signature, so the file will be respected and not replaced
		originalContents = "nearly empty "
		settingsFile, err = os.Create(expectedSettingsDdevLocation)
		assert.NoError(err)
		_, err = settingsFile.Write([]byte(originalContents))
		assert.NoError(err)

		// Invoke the settings file creation process
		_, err = app.CreateSettingsFile()
		assert.NoError(err)

		// Ensure settings.ddev.php was overwritten with new contents
		containsOriginalString, err = fileutil.FgrepStringInFile(expectedSettingsDdevLocation, originalContents)
		assert.NoError(err)
		assert.True(containsOriginalString, "Did not find %s in the settings file; it should have still been there", originalContents)
	}
}
