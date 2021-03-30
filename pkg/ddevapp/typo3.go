package ddevapp

import (
	"fmt"

	"os"
	"path/filepath"

	"github.com/drud/ddev/pkg/archive"
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/output"
	"github.com/drud/ddev/pkg/util"
)

const typo3AdditionalConfigTemplate = `<?php

/**
 * ` + DdevFileSignature + `: Automatically generated TYPO3 AdditionalConfiguration.php file.
 * ddev manages this file and may delete or overwrite the file unless this comment is removed.
 * It is recommended that you leave this file alone.
 */

if (getenv('IS_DDEV_PROJECT') == 'true') {
    $GLOBALS['TYPO3_CONF_VARS'] = array_replace_recursive(
        $GLOBALS['TYPO3_CONF_VARS'],
        [
            'DB' => [
                'Connections' => [
                    'Default' => [
                        'dbname' => 'db',
                        'host' => 'db',
                        'password' => 'db',
                        'port' => '3306',
                        'user' => 'db',
                    ],
                ],
            ],
            // This GFX configuration allows processing by installed ImageMagick 6
            'GFX' => [
                'processor' => 'ImageMagick',
                'processor_path' => '/usr/bin/',
                'processor_path_lzw' => '/usr/bin/',
            ],
            // This mail configuration sends all emails to mailhog
            'MAIL' => [
                'transport' => 'smtp',
                'transport_smtp_server' => 'localhost:1025',
            ],
            'SYS' => [
                'trustedHostsPattern' => '.*.*',
                'devIPmask' => '*',
                'displayErrors' => 1,
            ],
        ]
    );
}
`

// createTypo3SettingsFile creates the app's LocalConfiguration.php and
// AdditionalConfiguration.php, adding things like database host, name, and
// password. Returns the fullpath to settings file and error
func createTypo3SettingsFile(app *DdevApp) (string, error) {
	if !fileutil.FileExists(app.SiteSettingsPath) {
		util.Warning("TYPO3 does not seem to have been set up yet, missing %s (%s)", filepath.Base(app.SiteSettingsPath), app.SiteSettingsPath)
	}

	// TYPO3 ddev settings file will be AdditionalConfiguration.php (app.SiteDdevSettingsFile).
	// Check if the file already exists.
	if fileutil.FileExists(app.SiteDdevSettingsFile) {
		// Check if the file is managed by ddev.
		signatureFound, err := fileutil.FgrepStringInFile(app.SiteDdevSettingsFile, DdevFileSignature)
		if err != nil {
			return "", err
		}

		// If the signature wasn't found, warn the user and return.
		if !signatureFound {
			util.Warning("%s already exists and is managed by the user.", filepath.Base(app.SiteDdevSettingsFile))
			return app.SiteDdevSettingsFile, nil
		}
	}

	output.UserOut.Printf("Generating %s file for database connection.", filepath.Base(app.SiteDdevSettingsFile))
	if err := writeTypo3SettingsFile(app); err != nil {
		return "", fmt.Errorf("failed to write TYPO3 AdditionalConfiguration.php file: %v", err.Error())
	}

	return app.SiteDdevSettingsFile, nil
}

// writeTypo3SettingsFile produces AdditionalConfiguration.php file
// It's assumed that the LocalConfiguration.php already exists, and we're
// overriding the db config values in it. The typo3conf/ directory will
// be created if it does not yet exist.
func writeTypo3SettingsFile(app *DdevApp) error {
	filePath := app.SiteDdevSettingsFile

	// Ensure target directory is writable.
	dir := filepath.Dir(filePath)
	var perms os.FileMode = 0755
	if err := os.Chmod(dir, perms); err != nil {
		if !os.IsNotExist(err) {
			// The directory exists, but chmod failed.
			return err
		}

		// The directory doesn't exist, create it with the appropriate permissions.
		if err := os.Mkdir(dir, perms); err != nil {
			return err
		}
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	contents := []byte(typo3AdditionalConfigTemplate)
	err = os.WriteFile(filePath, contents, 0644)
	if err != nil {
		return err
	}
	util.CheckClose(file)
	return nil
}

// getTypo3UploadDir will return a custom upload dir if defined, returning a default path if not.
func getTypo3UploadDir(app *DdevApp) string {
	if app.UploadDir == "" {
		return "fileadmin"
	}

	return app.UploadDir
}

// Typo3Hooks adds a TYPO3-specific hooks example for post-import-db
const Typo3Hooks = `#  post-start:
#    - exec: composer install -d /var/www/html
`

// getTypo3Hooks for appending as byte array
func getTypo3Hooks() []byte {
	// We don't have anything new to add yet.
	return []byte(Typo3Hooks)
}

// setTypo3SiteSettingsPaths sets the paths to settings.php/settings.local.php
// for templating.
func setTypo3SiteSettingsPaths(app *DdevApp) {
	settingsFileBasePath := filepath.Join(app.AppRoot, app.Docroot)
	var settingsFilePath, localSettingsFilePath string
	settingsFilePath = filepath.Join(settingsFileBasePath, "typo3conf", "LocalConfiguration.php")
	localSettingsFilePath = filepath.Join(settingsFileBasePath, "typo3conf", "AdditionalConfiguration.php")
	app.SiteSettingsPath = settingsFilePath
	app.SiteDdevSettingsFile = localSettingsFilePath
}

// isTypoApp returns true if the app is of type typo3
func isTypo3App(app *DdevApp) bool {
	if _, err := os.Stat(filepath.Join(app.AppRoot, app.Docroot, "typo3")); err == nil {
		return true
	}
	return false
}

// typo3ImportFilesAction defines the TYPO3 workflow for importing project files.
// The TYPO3 import-files workflow is currently identical to the Drupal workflow.
func typo3ImportFilesAction(app *DdevApp, importPath, extPath string) error {
	destPath := filepath.Join(app.GetAppRoot(), app.GetDocroot(), app.GetUploadDir())

	// parent of destination dir should exist
	if !fileutil.FileExists(filepath.Dir(destPath)) {
		return fmt.Errorf("unable to import to %s: parent directory does not exist", destPath)
	}

	// parent of destination dir should be writable.
	if err := os.Chmod(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// If the destination path exists, remove it as was warned
	if fileutil.FileExists(destPath) {
		if err := os.RemoveAll(destPath); err != nil {
			return fmt.Errorf("failed to cleanup %s before import: %v", destPath, err)
		}
	}

	if isTar(importPath) {
		if err := archive.Untar(importPath, destPath, extPath); err != nil {
			return fmt.Errorf("failed to extract provided archive: %v", err)
		}

		return nil
	}

	if isZip(importPath) {
		if err := archive.Unzip(importPath, destPath, extPath); err != nil {
			return fmt.Errorf("failed to extract provided archive: %v", err)
		}

		return nil
	}

	//nolint: revive
	if err := fileutil.CopyDir(importPath, destPath); err != nil {
		return err
	}

	return nil
}
