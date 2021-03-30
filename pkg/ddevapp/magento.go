package ddevapp

import (
	"embed"
	"fmt"
	"github.com/drud/ddev/pkg/archive"
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/nodeps"
	"github.com/drud/ddev/pkg/output"
	"github.com/drud/ddev/pkg/util"
	"os"
	"path/filepath"
)

// isMagentoApp returns true if the app is of type magento
func isMagentoApp(app *DdevApp) bool {
	if _, err := os.Stat(filepath.Join(app.AppRoot, "get.php")); err == nil {
		return true
	}
	return false
}

// isMagento2App returns true if the app is of type magento2
func isMagento2App(app *DdevApp) bool {
	if _, err := os.Stat(filepath.Join(app.AppRoot, "pub", "static.php")); err == nil {
		return true
	}
	return false
}

//go:embed magento_assets
var magentoConfigAssets embed.FS

// createMagentoSettingsFile manages creation and modification of local.xml.
func createMagentoSettingsFile(app *DdevApp) (string, error) {

	if fileutil.FileExists(app.SiteSettingsPath) {
		// Check if the file is managed by ddev.
		signatureFound, err := fileutil.FgrepStringInFile(app.SiteSettingsPath, DdevFileSignature)
		if err != nil {
			return "", err
		}

		// If the signature wasn't found, warn the user and return.
		if !signatureFound {
			util.Warning("%s already exists and is managed by the user.", app.SiteSettingsPath)
			return "", nil
		}
	} else {
		output.UserOut.Printf("No %s file exists, creating one", app.SiteSettingsPath)

		content, err := magentoConfigAssets.ReadFile("magento_assets/local.xml")
		if err != nil {
			return "", err
		}
		err = os.WriteFile(app.SiteSettingsPath, content, 0644)
		if err != nil {
			return "", err
		}
	}

	return app.SiteDdevSettingsFile, nil
}

// setMagentoSiteSettingsPaths sets the paths to settings.php for templating.
func setMagentoSiteSettingsPaths(app *DdevApp) {
	settingsFileBasePath := app.AppRoot
	app.SiteSettingsPath = filepath.Join(settingsFileBasePath, "app", "etc", "local.xml")
}

// magentoImportFilesAction defines the magento workflow for importing project files.
func magentoImportFilesAction(app *DdevApp, importPath, extPath string) error {
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

// getMagentoUploadDir will return a custom upload dir if defined, returning a default path if not.
func getMagentoUploadDir(app *DdevApp) string {
	if app.UploadDir == "" {
		return "media"
	}

	return app.UploadDir
}

// getMagento2UploadDir will return a custom upload dir if defined, returning a default path if not.
func getMagento2UploadDir(app *DdevApp) string {
	if app.UploadDir == "" {
		return "media"
	}

	return app.UploadDir
}

// createMagento2SettingsFile manages creation and modification of app/etc/env.php.
func createMagento2SettingsFile(app *DdevApp) (string, error) {

	if fileutil.FileExists(app.SiteSettingsPath) {
		// Check if the file is managed by ddev.
		signatureFound, err := fileutil.FgrepStringInFile(app.SiteSettingsPath, DdevFileSignature)
		if err != nil {
			return "", err
		}

		// If the signature wasn't found, warn the user and return.
		if !signatureFound {
			util.Warning("%s already exists and is managed by the user.", app.SiteSettingsPath)
			return "", nil
		}
	} else {
		output.UserOut.Printf("No %s file exists, creating one", app.SiteSettingsPath)

		content, err := magentoConfigAssets.ReadFile("magento_assets/env.php")
		if err != nil {
			return "", err
		}
		err = os.WriteFile(app.SiteSettingsPath, content, 0644)
		if err != nil {
			return "", err
		}
	}

	return app.SiteDdevSettingsFile, nil
}

// setMagento2SiteSettingsPaths sets the paths to settings.php for templating.
func setMagento2SiteSettingsPaths(app *DdevApp) {
	app.SiteSettingsPath = filepath.Join(app.AppRoot, "app", "etc", "env.php")
}

// magentoConfigOverrideAction overrides php_version for magento2, since it is incompatible
// with php7.3+
func magentoConfigOverrideAction(app *DdevApp) error {
	app.PHPVersion = nodeps.PHP56
	return nil
}
