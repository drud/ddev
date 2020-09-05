package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/drud/ddev/pkg/ddevapp"
	"github.com/drud/ddev/pkg/exec"
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/globalconfig"
	"github.com/drud/ddev/pkg/util"
	"github.com/gobuffalo/packr/v2"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// Define the structure for flags, the long option is used as map key
type FlagDefinition = struct {
	Usage     string
	Shorthand string
}

type FlagsDefinitions = map[string]FlagDefinition

// addCustomCommands looks for custom command scripts in
// ~/.ddev/commands/<servicename> etc. and
// .ddev/commands/<servicename> and .ddev/commands/host
// and if it finds them adds them to Cobra's commands.
func addCustomCommands(rootCmd *cobra.Command) error {
	app, err := ddevapp.GetActiveApp("")
	if err != nil {
		return nil
	}

	sourceGlobalCommandPath := filepath.Join(globalconfig.GetGlobalDdevDir(), "commands")
	err = os.MkdirAll(sourceGlobalCommandPath, 0755)
	if err != nil {
		return nil
	}

	projectCommandPath := app.GetConfigPath("commands")
	// Make sure our target global command directory is empty
	targetGlobalCommandPath := app.GetConfigPath(".global_commands")
	_ = os.RemoveAll(targetGlobalCommandPath)

	err = fileutil.CopyDir(sourceGlobalCommandPath, targetGlobalCommandPath)
	if err != nil {
		return err
	}

	if !fileutil.FileExists(projectCommandPath) || !fileutil.IsDirectory(projectCommandPath) {
		return nil
	}

	commandsAdded := map[string]int{}
	for _, commandSet := range []string{projectCommandPath, targetGlobalCommandPath} {
		commandDirs, err := fileutil.ListFilesInDirFullPath(commandSet)
		if err != nil {
			return err
		}
		for _, serviceDirOnHost := range commandDirs {
			service := filepath.Base(serviceDirOnHost)

			// If the item isn't actually a directory, just skip it.
			if !fileutil.IsDirectory(serviceDirOnHost) {
				continue
			}
			commandFiles, err := fileutil.ListFilesInDir(serviceDirOnHost)
			if err != nil {
				return err
			}
			if runtime.GOOS == "windows" {
				windowsBashPath := util.FindWindowsBashPath()
				if windowsBashPath == "" {
					fmt.Println("Unable to find bash.exe in PATH, not loading custom commands")
					return nil
				}
			}

			for _, commandName := range commandFiles {
				// Use path.Join() for the inContainerFullPath because it'serviceDirOnHost about the path in the container, not on the
				// host; a Windows path is not useful here.
				inContainerFullPath := path.Join("/mnt/ddev_config", filepath.Base(commandSet), service, commandName)
				onHostFullPath := filepath.Join(commandSet, service, commandName)

				if strings.HasSuffix(commandName, ".example") || strings.HasPrefix(commandName, "README") || strings.HasPrefix(commandName, ".") || fileutil.IsDirectory(onHostFullPath) {
					continue
				}

				// If command has already been added, we won't work with it again.
				if _, ok := commandsAdded[commandName]; ok {
					util.Warning("not adding command %s (%s) because it was already added to project %s", commandName, onHostFullPath, app.Name)
					continue
				}

				// Any command we find will want to be executable on Linux
				_ = os.Chmod(onHostFullPath, 0755)
				if hasCR, _ := fileutil.FgrepStringInFile(onHostFullPath, "\r\n"); hasCR {
					util.Warning("command '%s' contains CRLF, please convert to Linux-style linefeeds with dos2unix or another tool, skipping %s", commandName, onHostFullPath)
					continue
				}
				directives := findDirectivesInScriptCommand(onHostFullPath)
				var description, usage, example, projectTypes, osTypes, hostBinaryExists string
				var flags = FlagsDefinitions{}

				description = commandName
				if val, ok := directives["Description"]; ok {
					description = val
				}

				if val, ok := directives["Usage"]; ok {
					usage = val
				}
				if val, ok := directives["Example"]; ok {
					example = "  " + strings.ReplaceAll(val, `\n`, "\n  ")
				}
				if val, ok := directives["Flags"]; ok {
					flags, err = parseFlags(val)
					if err != nil {
						util.Warning("Error '%s', command '%s' contains an invalid flags definition '%s', skipping add flags of %s", err, commandName, val, onHostFullPath)
					}
				}
				if val, ok := directives["ProjectTypes"]; ok {
					projectTypes = val
				}
				// If ProjectTypes is specified and we aren't of that type, skip
				if projectTypes != "" && !strings.Contains(projectTypes, app.Type) {
					continue
				}

				if val, ok := directives["OSTypes"]; ok {
					osTypes = val
				}
				// If OSTypes is specified and we aren't this isn't a specified OS, skip
				if osTypes != "" && !strings.Contains(osTypes, runtime.GOOS) {
					continue
				}

				if val, ok := directives["HostBinaryExists"]; ok {
					hostBinaryExists = val
				}
				// If hostBinaryExists is specified it doesn't exist here, skip
				if hostBinaryExists != "" && !fileutil.FileExists(hostBinaryExists) {
					continue
				}

				descSuffix := " (shell " + service + " container command)"
				if commandSet == targetGlobalCommandPath {
					descSuffix = " (global shell " + service + " container command)"
				}

				commandToAdd := &cobra.Command{
					Use:     usage,
					Short:   description + descSuffix,
					Example: example,
					FParseErrWhitelist: cobra.FParseErrWhitelist{
						UnknownFlags: true,
					},
				}

				for flag, flagDef := range flags {
					// Check usage is defined
					if flagDef.Usage == "" {
						util.Warning("No usage defined for flag '%s' of command '%s', skipping add flag defined in %s", flag, commandName, onHostFullPath)
						continue
					}

					// Check flag and shorthand does not already exist
					if commandToAdd.Flags().Lookup(flag) != nil {
						util.Warning("Flag '%s' already defined for command '%s', skipping add flag defined in %s", flag, commandName, onHostFullPath)
						continue
					}

					if commandToAdd.Flags().ShorthandLookup(flagDef.Shorthand) != nil {
						util.Warning("Shorthand '%s' already defined for command '%s', skipping add flag defined in %s", flagDef.Shorthand, commandName, onHostFullPath)
						continue
					}

					// Add flag to command
					commandToAdd.Flags().BoolP(flag, flagDef.Shorthand, false, flagDef.Usage)
				}

				if service == "host" {
					commandToAdd.Run = makeHostCmd(app, onHostFullPath, commandName)
				} else {
					commandToAdd.Run = makeContainerCmd(app, inContainerFullPath, commandName, service)
				}
				rootCmd.AddCommand(commandToAdd)

				commandsAdded[commandName] = 1
			}
		}
	}

	return nil
}

// makeHostCmd creates a command which will run on the host
func makeHostCmd(app *ddevapp.DdevApp, fullPath, name string) func(*cobra.Command, []string) {
	var windowsBashPath = ""
	if runtime.GOOS == "windows" {
		windowsBashPath = util.FindWindowsBashPath()
	}

	return func(cmd *cobra.Command, cobraArgs []string) {
		if app.SiteStatus() != ddevapp.SiteRunning {
			err := app.Start()
			if err != nil {
				util.Failed("Failed to start project for custom command: %v", err)
			}
		}
		app.DockerEnv()

		osArgs := []string{}
		if len(os.Args) > 2 {
			osArgs = os.Args[2:]
		}
		var err error
		// Load environment variables that may be useful for script.
		app.DockerEnv()
		if runtime.GOOS == "windows" {
			// Sadly, not sure how to have a bash interpreter without this.
			args := []string{fullPath}
			args = append(args, osArgs...)
			err = exec.RunInteractiveCommand(windowsBashPath, args)
		} else {
			err = exec.RunInteractiveCommand(fullPath, osArgs)
		}
		if err != nil {
			util.Failed("Failed to run %s %v; error=%v", name, strings.Join(osArgs, " "), err)
		}
	}
}

// makeContainerCmd creates the command which will app.Exec to a container command
func makeContainerCmd(app *ddevapp.DdevApp, fullPath, name string, service string) func(*cobra.Command, []string) {
	s := service
	if s[0:1] == "." {
		s = s[1:]
	}
	return func(cmd *cobra.Command, args []string) {
		if app.SiteStatus() != ddevapp.SiteRunning {
			err := app.Start()
			if err != nil {
				util.Failed("Failed to start project for custom command: %v", err)
			}
		}
		app.DockerEnv()

		osArgs := []string{}
		if len(os.Args) > 2 {
			osArgs = os.Args[2:]
		}
		_, _, err := app.Exec(&ddevapp.ExecOpts{
			Cmd:       fullPath + " " + strings.Join(osArgs, " "),
			Service:   s,
			Dir:       app.GetWorkingDir(s, ""),
			Tty:       isatty.IsTerminal(os.Stdin.Fd()),
			NoCapture: true,
		})

		if err != nil {
			util.Failed("Failed to run %s %v: %v", name, strings.Join(osArgs, " "), err)
		}
	}
}

// findDirectivesInScriptCommand() Returns a map of directives and their contents
// found in the named script
func findDirectivesInScriptCommand(script string) map[string]string {
	f, err := os.Open(script)
	if err != nil {
		util.Failed("Failed to open %s: %v", script, err)
	}

	// nolint errcheck
	defer f.Close()

	var directives = make(map[string]string)

	// Splits on newlines by default.
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "## ") && strings.Contains(line, ":") {
			line = strings.Replace(line, "## ", "", 1)
			parts := strings.SplitN(line, ":", 2)
			parts[1] = strings.Trim(parts[1], " ")
			directives[parts[0]] = parts[1]
		}
	}

	if err := scanner.Err(); err != nil {
		return nil
	}

	return directives
}

// populateExamplesCommandsHomeadditions grabs packr2 assets
// When the items in the assets directory are changed, the packr2 command
// must be run again in this directory (cmd/ddev/cmd) to update the saved
// embedded files.
// "make packr2" can be used to update the packr2 cache.
func populateExamplesCommandsHomeadditions() error {
	app, err := ddevapp.GetActiveApp("")
	if err != nil {
		return nil
	}
	box := packr.New("customcommands", "./dotddev_assets")

	list := box.List()
	for _, file := range list {
		localPath := app.GetConfigPath(file)
		sigFound, err := fileutil.FgrepStringInFile(localPath, ddevapp.DdevFileSignature)
		if sigFound || err != nil {
			content, err := box.Find(file)
			if err != nil {
				return err
			}
			err = os.MkdirAll(filepath.Dir(localPath), 0755)
			if err != nil {
				return err
			}
			err = ioutil.WriteFile(localPath, content, 0755)
			if err != nil {
				return err
			}
		}
	}

	// This brings in both the commands and the homeadditions files
	box = packr.New("global_dotddev", "./global_dotddev_assets")

	list = box.List()
	globalDdevDir := globalconfig.GetGlobalDdevDir()
	for _, file := range list {
		localPath := filepath.Join(globalDdevDir, file)
		sigFound, err := fileutil.FgrepStringInFile(localPath, ddevapp.DdevFileSignature)
		if sigFound || err != nil {
			content, err := box.Find(file)
			if err != nil {
				return err
			}
			err = os.MkdirAll(filepath.Dir(localPath), 0755)
			if err != nil {
				return err
			}
			err = ioutil.WriteFile(localPath, content, 0755)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// parseFlags() Returns a map of flags found in the named script before
func parseFlags(flags string) (FlagsDefinitions, error) {
	var defs = FlagsDefinitions{}

	if err := json.Unmarshal([]byte(flags), &defs); err != nil {
		return nil, err
	}

	return defs, nil
}
