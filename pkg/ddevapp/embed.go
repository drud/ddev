package ddevapp

import (
	"embed"
	"github.com/drud/ddev/pkg/fileutil"
	"os"
	"path"
	"path/filepath"
)

// CopyEmbedAssets copies files in the named embed.FS sourceDir to the local targetDir (full path)
func CopyEmbedAssets(fsys embed.FS, sourceDir string, targetDir string) error {
	subdirs, err := fsys.ReadDir(sourceDir)
	if err != nil {
		return err
	}
	for _, d := range subdirs {
		sourcePath := path.Join(sourceDir, d.Name())
		if d.IsDir() {
			err = CopyEmbedAssets(fsys, path.Join(sourceDir, d.Name()), path.Join(targetDir, d.Name()))
			if err != nil {
				return err
			}
		} else {
			localPath := filepath.Join(targetDir, d.Name())

			sigFound, err := fileutil.FgrepStringInFile(localPath, DdevFileSignature)
			if sigFound || err != nil {
				content, err := fsys.ReadFile(sourcePath)
				if err != nil {
					return err
				}
				err = os.MkdirAll(filepath.Dir(localPath), 0755)
				if err != nil {
					return err
				}
				err = os.WriteFile(localPath, content, 0755)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
