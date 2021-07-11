package fileutil

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"runtime"

	"github.com/drud/ddev/pkg/output"
	"github.com/drud/ddev/pkg/util"
)

// CopyFile copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file. The file mode will be copied from the source and
// the copied data is synced/flushed to stable storage. Credit @m4ng0squ4sh https://gist.github.com/m4ng0squ4sh/92462b38df26839a3ca324697c8cba04
func CopyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer util.CheckClose(in)
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("Failed to create file %v, err: %v", src, err)
	}
	defer util.CheckClose(out)
	_, err = io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("Failed to copy file from %v to %v err: %v", src, dst, err)
	}

	err = out.Sync()
	if err != nil {
		return err
	}

	// os.Chmod fails on long path (> 256 characters) on windows.
	// A description of this problem with golang is at https://github.com/golang/dep/issues/774#issuecomment-311560825
	// It could end up fixed in a future version of golang.
	if runtime.GOOS != "windows" {
		si, err := os.Stat(src)
		if err != nil {
			return err
		}

		err = os.Chmod(dst, si.Mode())
		if err != nil {
			return fmt.Errorf("Failed to chmod file %v to mode %v, err=%v", dst, si.Mode(), err)
		}
	}

	return nil
}

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
// Symlinks are ignored and skipped. Credit @r0l1 https://gist.github.com/r0l1/92462b38df26839a3ca324697c8cba04
func CopyDir(src string, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("CopyDir: source directory %s is not a directory", src)
	}

	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		return fmt.Errorf("CopyDir: destination %s already exists", dst)
	}

	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		return err
	}

	dirEntrySlice, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, de := range dirEntrySlice {

		srcPath := filepath.Join(src, de.Name())
		dstPath := filepath.Join(dst, de.Name())

		if de.IsDir() {
			err = CopyDir(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			deInfo, err := de.Info()
			if err != nil {
				return err
			}
			err = CopyFile(srcPath, dstPath)
			if err != nil && deInfo.Mode()&os.ModeSymlink != 0 {
				output.UserOut.Warnf("failed to copy symlink %s, skipping...\n", srcPath)
				continue
			}
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// FileExists checks a file's existence
func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// IsDirectory returns true if path is a dir, false on error or not directory
func IsDirectory(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fileInfo.IsDir()
}

// FileIsReadable checks to make sure a file exists and is readable
func FileIsReadable(name string) bool {
	file, err := os.OpenFile(name, os.O_RDONLY, 0666)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

// PurgeDirectory removes all of the contents of a given
// directory, leaving the directory itself intact.
func PurgeDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}

	defer util.CheckClose(dir)

	files, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}

	for _, file := range files {
		err = os.Chmod(filepath.Join(path, file), 0777)
		if err != nil {
			return err
		}
		err = os.RemoveAll(filepath.Join(path, file))
		if err != nil {
			return err
		}
	}
	return nil
}

// FgrepStringInFile is a small hammer for looking for a literal string in a file.
// It should only be used against very modest sized files, as the entire file is read
// into a string.
func FgrepStringInFile(fullPath string, needle string) (bool, error) {
	fullFileBytes, err := os.ReadFile(fullPath)
	if err != nil {
		return false, fmt.Errorf("failed to open file %s, err:%v ", fullPath, err)
	}
	fullFileString := string(fullFileBytes)
	return strings.Contains(fullFileString, needle), nil
}

// ListFilesInDir returns an array of files found in a directory
func ListFilesInDir(path string) ([]string, error) {
	var fileList []string
	dirEntrySlice, err := os.ReadDir(path)
	if err != nil {
		return fileList, err
	}

	for _, de := range dirEntrySlice {
		fileList = append(fileList, de.Name())
	}
	return fileList, nil
}

// ListFilesInDirFullPath returns an array of full path of files found in a directory
func ListFilesInDirFullPath(path string) ([]string, error) {
	var fileList []string
	dirEntrySlice, err := os.ReadDir(path)
	if err != nil {
		return fileList, err
	}

	for _, de := range dirEntrySlice {
		fileList = append(fileList, filepath.Join(path, de.Name()))
	}
	return fileList, nil
}

// RandomFilenameBase generates a temporary filename for use in testing or whatever.
// From https://stackoverflow.com/a/28005931/215713
func RandomFilenameBase() string {
	randBytes := make([]byte, 16)
	_, _ = rand.Read(randBytes)
	return hex.EncodeToString(randBytes)
}

// ReplaceStringInFile takes search and replace strings, an original path, and a dest path, returns error
func ReplaceStringInFile(searchString string, replaceString string, origPath string, destPath string) error {
	input, err := os.ReadFile(origPath)
	if err != nil {
		return err
	}

	output := bytes.Replace(input, []byte(searchString), []byte(replaceString), -1)

	// nolint: revive
	if err = os.WriteFile(destPath, output, 0666); err != nil {
		return err
	}
	return nil
}

// IsSameFile determines whether two paths refer to the same file/dir
func IsSameFile(path1 string, path2 string) (bool, error) {
	path1fi, err := os.Stat(path1)
	if err != nil {
		return false, err
	}
	path2fi, err := os.Stat(path2)
	if err != nil {
		return false, err
	}
	return os.SameFile(path1fi, path2fi), nil
}

// ReadFileIntoString just gets the contents of file into string
func ReadFileIntoString(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bytes), err
}

// AppendStringToFile takes a path to a file and a string to append
// and it appends it, returning err
func AppendStringToFile(path string, appendString string) error {
	f, err := os.OpenFile(path,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(appendString); err != nil {
		return err
	}
	return nil
}

type XSymContents struct {
	LinkLocation string
	LinkTarget   string
}

// FindSimulatedXsymSymlinks searches the basePath provided for files
// whose first line is XSym, which is used in cifs filesystem for simulated
// symlinks.
func FindSimulatedXsymSymlinks(basePath string) ([]XSymContents, error) {
	symLinks := make([]XSymContents, 0)
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		//TODO: Skip a directory named .git? Skip other arbitrary dirs or files?
		if !info.IsDir() {
			if info.Size() == 1067 {
				contents, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				lines := strings.Split(string(contents), "\n")
				if lines[0] != "XSym" {
					return nil
				}
				if len(lines) < 4 {
					return fmt.Errorf("Apparent XSym doesn't have enough lines: %s", path)
				}
				// target is 4th line
				linkTarget := filepath.Clean(lines[3])
				symLinks = append(symLinks, XSymContents{LinkLocation: path, LinkTarget: linkTarget})
			}
		}
		return nil
	})
	return symLinks, err
}

// ReplaceSimulatedXsymSymlinks walks a list of XSymContents and makes real symlinks
// in their place. This is only valid on Windows host, only works with Docker for Windows
// (cifs filesystem)
func ReplaceSimulatedXsymSymlinks(links []XSymContents) error {
	for _, item := range links {
		err := os.Remove(item.LinkLocation)
		if err != nil {
			return err
		}
		err = os.Symlink(item.LinkTarget, item.LinkLocation)
		if err != nil {
			return err
		}
	}
	return nil
}

// CanCreateSymlinks tests to see if it's possible to create a symlink
func CanCreateSymlinks() bool {
	tmpdir := os.TempDir()
	linkPath := filepath.Join(tmpdir, RandomFilenameBase())
	// This doesn't attempt to create the real file; we don't need it.
	err := os.Symlink(filepath.Join(tmpdir, "realfile.txt"), linkPath)
	//nolint: errcheck
	defer os.Remove(linkPath)
	if err != nil {
		return false
	}
	return true
}

// ReplaceSimulatedLinks walks the path provided and tries to replace XSym links with real ones.
func ReplaceSimulatedLinks(path string) {
	links, err := FindSimulatedXsymSymlinks(path)
	if err != nil {
		util.Warning("Error finding XSym Symlinks: %v", err)
	}
	if len(links) == 0 {
		return
	}

	if !CanCreateSymlinks() {
		util.Warning("This host computer is unable to create real symlinks, please see the docs to enable developer mode:\n%s\nNote that the simulated symlinks created inside the container will work fine for most projects.", "https://ddev.readthedocs.io/en/stable/users/developer-tools/#windows-os-and-ddev-composer")
		return
	}

	err = ReplaceSimulatedXsymSymlinks(links)
	if err != nil {
		util.Warning("Failed replacing simulated symlinks: %v", err)
	}
	replacedLinks := make([]string, 0)
	for _, l := range links {
		replacedLinks = append(replacedLinks, l.LinkLocation)
	}
	util.Success("Replaced these simulated symlinks with real symlinks: %v", replacedLinks)
	return
}

// RemoveContents removes contents of passed directory
// From https://stackoverflow.com/questions/33450980/how-to-remove-all-contents-of-a-directory-using-golang
func RemoveContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}
	return nil
}

// TemplateStringToFile takes a template string, runs templ.Execute on it, and writes it out to file
func TemplateStringToFile(content string, vars map[string]interface{}, targetFilePath string) error {

	templ := template.New("templateStringToFile:" + targetFilePath)
	templ, err := templ.Parse(content)
	if err != nil {
		return err
	}

	var doc bytes.Buffer
	err = templ.Execute(&doc, vars)
	if err != nil {
		return err
	}

	f, err := os.Create(targetFilePath)
	if err != nil {
		return err
	}
	defer util.CheckClose(f)

	_, err = f.WriteString(doc.String())
	if err != nil {
		return nil
	}
	return nil
}
