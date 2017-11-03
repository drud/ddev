package platform

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/fsouza/go-dockerclient"
	"github.com/gosuri/uitable"
	"github.com/lextoumbourou/goodhosts"
	log "github.com/sirupsen/logrus"

	"errors"

	"github.com/drud/ddev/pkg/dockerutil"
	"github.com/drud/ddev/pkg/exec"
	"github.com/drud/ddev/pkg/fileutil"
	"github.com/drud/ddev/pkg/util"
	gohomedir "github.com/mitchellh/go-homedir"
)

// GetApps returns a list of ddev applictions keyed by platform.
func GetApps() map[string][]App {
	apps := make(map[string][]App)
	for platformType := range PluginMap {
		labels := map[string]string{
			"com.ddev.platform":          "ddev",
			"com.docker.compose.service": "web",
		}
		sites, err := dockerutil.FindContainersByLabels(labels)

		if err == nil {
			for _, siteContainer := range sites {
				site, err := GetPluginApp(platformType)
				// This should absolutely never happen, so just fatal on the off chance it does.
				if err != nil {
					log.Fatalf("could not get application for plugin type %s", platformType)
				}
				approot, ok := siteContainer.Labels["com.ddev.approot"]
				if !ok {
					break
				}
				_, ok = apps[platformType]
				if !ok {
					apps[platformType] = []App{}
				}

				err = site.Init(approot)
				if err != nil {
					// Cast 'site' from type App to type LocalApp, so we can manually enter AppConfig values.
					siteStruct, ok := site.(*LocalApp)
					if !ok {
						log.Fatalf("Failed to cast siteStruct(type App) to *LocalApp{}. site=%v", site)
					}

					siteStruct.AppConfig.Name = siteContainer.Labels["com.ddev.site-name"]
					siteStruct.AppConfig.AppType = siteContainer.Labels["com.ddev.app-type"]
				}
				apps[platformType] = append(apps[platformType], site)
			}
		}
	}

	return apps
}

// RenderAppTable will format a table for user display based on a list of apps.
func RenderAppTable(platform string, apps []App) {
	if len(apps) > 0 {
		fmt.Printf("%v %s %v found.\n", len(apps), platform, util.FormatPlural(len(apps), "site", "sites"))
		table := CreateAppTable()
		for _, site := range apps {
			RenderAppRow(table, site)
		}
		fmt.Println(table)
		fmt.Println(PrintRouterStatus())
	}
}

// CreateAppTable will create a new app table for describe and list output
func CreateAppTable() *uitable.Table {
	table := uitable.New()
	table.MaxColWidth = 140
	table.Separator = "  "
	table.AddRow("NAME", "TYPE", "LOCATION", "URL", "STATUS")
	return table
}

// RenderHomeRootedDir shortens a directory name to replace homedir with ~
func RenderHomeRootedDir(path string) string {
	userDir, err := gohomedir.Dir()
	util.CheckErr(err)
	result := strings.Replace(path, userDir, "~", 1)
	result = strings.Replace(result, "\\", "/", -1)
	return result
}

// RenderAppRow will add an application row to an existing table for describe and list output.
func RenderAppRow(table *uitable.Table, site App) {
	shortRoot := RenderHomeRootedDir(site.AppRoot())
	status := site.SiteStatus()

	switch {
	case strings.Contains(status, SiteStopped):
		status = color.YellowString(status)
	case strings.Contains(status, SiteNotFound):
		status = color.RedString(status)
	case strings.Contains(status, SiteDirMissing):
		status = color.RedString(status)
	case strings.Contains(status, SiteConfigMissing):
		status = color.RedString(status)
	default:
		status = color.CyanString(status)
	}

	table.AddRow(
		site.GetName(),
		site.GetType(),
		shortRoot,
		site.URL(),
		status,
	)
}

// Cleanup will clean up ddev apps even if the composer file has been deleted.
func Cleanup(app App) error {
	client := dockerutil.GetDockerClient()

	// Find all containers which match the current site name.
	labels := map[string]string{
		"com.ddev.site-name": app.GetName(),
	}
	containers, err := dockerutil.FindContainersByLabels(labels)
	if err != nil {
		return err
	}

	// First, try stopping the listed containers if they are running.
	for i := range containers {
		if containers[i].State == "running" || containers[i].State == "restarting" || containers[i].State == "paused" {
			containerName := containers[i].Names[0][1:len(containers[i].Names[0])]
			fmt.Printf("Stopping container: %s\n", containerName)
			err = client.StopContainer(containers[i].ID, 60)
			if err != nil {
				return fmt.Errorf("could not stop container %s: %v", containerName, err)
			}
		}
	}

	// Try to remove the containers once they are stopped.
	for i := range containers {
		containerName := containers[i].Names[0][1:len(containers[i].Names[0])]
		removeOpts := docker.RemoveContainerOptions{
			ID:            containers[i].ID,
			RemoveVolumes: true,
			Force:         true,
		}
		fmt.Printf("Removing container: %s\n", containerName)
		if err = client.RemoveContainer(removeOpts); err != nil {
			return fmt.Errorf("could not remove container %s: %v", containerName, err)
		}
	}

	volumes, err := client.ListVolumes(docker.ListVolumesOptions{})
	if err != nil {
		return err
	}

	for _, volume := range volumes {
		if volume.Labels["com.docker.compose.project"] == "ddev"+strings.ToLower(app.GetName()) {
			err := client.RemoveVolume(volume.Name)
			if err != nil {
				return fmt.Errorf("could not remove volume %s: %v", volume.Name, err)
			}
		}
	}

	return StopRouter()
}

// CheckForConf checks for a config.yaml at the cwd or parent dirs.
func CheckForConf(confPath string) (string, error) {
	if fileutil.FileExists(confPath + "/.ddev/config.yaml") {
		return confPath, nil
	}
	pathList := strings.Split(confPath, "/")

	for range pathList {
		confPath = filepath.Dir(confPath)
		if fileutil.FileExists(confPath + "/.ddev/config.yaml") {
			return confPath, nil
		}
	}

	return "", errors.New("no .ddev/config.yaml file was found in this directory or any parent")
}

// ddevContainersRunning determines if any ddev-controlled containers are currently running.
func ddevContainersRunning() (bool, error) {
	containers, err := dockerutil.GetDockerContainers(false)
	if err != nil {
		return false, err
	}

	for _, container := range containers {
		if _, ok := container.Labels["com.ddev.platform"]; ok {
			return true, nil
		}
	}
	return false, nil
}

// SetOfflineMode enables offline mode for ddev. When offline mode is
// set, a tracking file is created and hosts entries are created for
// currently running sites. Any new sites started during offline mode
// will prompt for creating host entries.
func SetOfflineMode() error {
	var addHostnames []string
	dockerIP := getDockerIP()

	err := ioutil.WriteFile(offlineFile, []byte(""), 0644)
	if err != nil {
		return err
	}

	hosts, err := goodhosts.NewHosts()
	if err != nil {
		log.Fatalf("could not open hostfile. %s", err)
	}

	labels := map[string]string{
		"com.ddev.platform":          "ddev",
		"com.docker.compose.service": "web",
	}
	sites, err := dockerutil.FindContainersByLabels(labels)
	if err != nil {
		return err
	}

	for _, site := range sites {
		hostname := site.Labels["com.ddev.hostname"]
		if !hosts.Has(dockerIP, hostname) {
			addHostnames = append(addHostnames, hostname)
		}
	}

	if len(addHostnames) == 0 {
		return nil
	}

	_, err = osexec.Command("sudo", "-h").Output()
	if (os.Getenv("DRUD_NONINTERACTIVE") != "") || err != nil {
		util.Warning("You must manually add the following entries to your hosts file:\n%s %s", dockerIP, strings.Join(addHostnames, " "))
		return nil
	}

	ddevFullpath, err := os.Executable()
	util.CheckErr(err)
	fmt.Println("ddev needs to add entries to your hostfile for offline mode.\nIt will require root privileges via the sudo command, so you may be required\nto enter your password for sudo. ddev is about to issue the command:")
	hostnameArgs := []string{ddevFullpath, "hostname", dockerIP}
	hostnameArgs = append(hostnameArgs, addHostnames...)
	command := strings.Join(hostnameArgs, " ")
	util.Warning(fmt.Sprintf("    sudo %s", command))
	fmt.Println("Please enter your password if prompted.")
	err = exec.RunCommandPipe("sudo", hostnameArgs)
	return err
}

// UnsetOfflineMode removes the tracking file for offline mode in order
// to disable it.
func UnsetOfflineMode() error {
	err := os.Remove(offlineFile)
	return err
}

func getDockerIP() string {
	dockerIP := "127.0.0.1"
	dockerHostRawURL := os.Getenv("DOCKER_HOST")
	if dockerHostRawURL != "" {
		dockerHostURL, err := url.Parse(dockerHostRawURL)
		if err != nil {
			log.Errorf("Failed to parse $DOCKER_HOST: %v, err: %v", dockerHostRawURL, err)
		}
		dockerIP = dockerHostURL.Hostname()
	}
	return dockerIP
}
