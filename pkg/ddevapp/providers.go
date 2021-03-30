package ddevapp

import (
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/nodeps"
	"os"
	"strings"
)

// IsValidProvider is a helper function to determine if a provider value is valid, returning
// true if the supplied provider is valid and false otherwise.
func (app *DdevApp) IsValidProvider(provider string) (bool, error) {
	pList, err := app.GetValidProviders()
	if err != nil {
		return false, err
	}
	return nodeps.ArrayContainsString(pList, provider), nil
}

// GetValidProviders is a helper function that returns a list of valid providers.
func (app *DdevApp) GetValidProviders() ([]string, error) {
	pPath := app.GetConfigPath("providers")
	providers := []string{}
	if !fileutil.IsDirectory(pPath) {
		return providers, nil
	}

	dirEntrySlice, err := os.ReadDir(pPath)
	if err != nil {
		return providers, err
	}
	for _, de := range dirEntrySlice {
		if strings.HasSuffix(de.Name(), ".yaml") {
			providers = append(providers, strings.TrimSuffix(de.Name(), ".yaml"))
		}
	}
	return providers, nil
}
