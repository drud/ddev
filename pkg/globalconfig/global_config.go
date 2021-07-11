package globalconfig

import (
	"context"
	"fmt"
	"github.com/drud/ddev/pkg/nodeps"
	"github.com/drud/ddev/pkg/output"
	"github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DdevGlobalConfigName is the name of the global config file.
const DdevGlobalConfigName = "global_config.yaml"

var (
	// DdevGlobalConfig is the currently active global configuration struct
	DdevGlobalConfig GlobalConfig
)

func init() {
	DdevGlobalConfig.ProjectList = make(map[string]*ProjectInfo)
}

type ProjectInfo struct {
	AppRoot       string   `yaml:"approot"`
	UsedHostPorts []string `yaml:"used_host_ports,omitempty,flow"`
}

// GlobalConfig is the struct defining ddev's global config
type GlobalConfig struct {
	OmitContainersGlobal     []string `yaml:"omit_containers,flow"`
	NFSMountEnabledGlobal    bool     `yaml:"nfs_mount_enabled"`
	InstrumentationOptIn     bool     `yaml:"instrumentation_opt_in"`
	RouterBindAllInterfaces  bool     `yaml:"router_bind_all_interfaces"`
	InternetDetectionTimeout int64    `yaml:"internet_detection_timeout_ms"`
	DeveloperMode            bool     `yaml:"developer_mode,omitempty"`
	InstrumentationUser      string   `yaml:"instrumentation_user,omitempty"`
	LastStartedVersion       string   `yaml:"last_started_version"`
	MkcertCARoot             string   `yaml:"mkcert_caroot"`
	UseHardenedImages        bool     `yaml:"use_hardened_images"`
	UseLetsEncrypt           bool     `yaml:"use_letsencrypt"`
	LetsEncryptEmail         string   `yaml:"letsencrypt_email"`
	AutoRestartContainers    bool     `yaml:"auto_restart_containers"`
	FailOnHookFailGlobal     bool     `yaml:"fail_on_hook_fail"`
	WebEnvironment           []string `yaml:"web_environment"`
	DisableHTTP2             bool     `yaml:"disable_http2"`

	ProjectList map[string]*ProjectInfo `yaml:"project_info"`
}

// GetGlobalConfigPath gets the path to global config file
func GetGlobalConfigPath() string {
	return filepath.Join(GetGlobalDdevDir(), DdevGlobalConfigName)
}

// ValidateGlobalConfig validates global config
func ValidateGlobalConfig() error {
	if !IsValidOmitContainers(DdevGlobalConfig.OmitContainersGlobal) {
		return fmt.Errorf("Invalid omit_containers: %s, must contain only %s", strings.Join(DdevGlobalConfig.OmitContainersGlobal, ","), strings.Join(GetValidOmitContainers(), ",")).(InvalidOmitContainers)
	}

	return nil
}

// ReadGlobalConfig reads the global config file into DdevGlobalConfig
func ReadGlobalConfig() error {
	globalConfigFile := GetGlobalConfigPath()

	// Can't use fileutil.FileExists() here because of import cycle.
	if _, err := os.Stat(globalConfigFile); err != nil {
		// ~/.ddev doesn't exist and running as root (only ddev hostname could do this)
		// Then create global config.
		if os.Geteuid() == 0 {
			logrus.Warning("not reading global config file because running with root privileges")
			return nil
		}
		if os.IsNotExist(err) {
			err := WriteGlobalConfig(DdevGlobalConfig)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	source, err := os.ReadFile(globalConfigFile)
	if err != nil {
		return fmt.Errorf("Unable to read ddev global config file %s: %v", source, err)
	}

	// ReadConfig config values from file.
	DdevGlobalConfig = GlobalConfig{InternetDetectionTimeout: nodeps.InternetDetectionTimeoutDefault}
	err = yaml.Unmarshal(source, &DdevGlobalConfig)
	if err != nil {
		return err
	}
	if DdevGlobalConfig.ProjectList == nil {
		DdevGlobalConfig.ProjectList = map[string]*ProjectInfo{}
	}
	// Set/read the CAROOT if it's unset or different from $CAROOT (perhaps $CAROOT changed)
	caRootEnv := os.Getenv("CAROOT")
	if DdevGlobalConfig.MkcertCARoot == "" || (caRootEnv != "" && caRootEnv != DdevGlobalConfig.MkcertCARoot) {
		DdevGlobalConfig.MkcertCARoot = readCAROOT()
	}
	// This is added just so we can see it in global; not checked.
	// Make sure that LastStartedVersion always has a valid value
	if DdevGlobalConfig.LastStartedVersion == "" {
		DdevGlobalConfig.LastStartedVersion = "v0.0"
	}
	// If they set the internetdetectiontimeout below default, just reset to default
	// and ignore the setting.
	if DdevGlobalConfig.InternetDetectionTimeout < nodeps.InternetDetectionTimeoutDefault {
		DdevGlobalConfig.InternetDetectionTimeout = nodeps.InternetDetectionTimeoutDefault
	}

	err = ValidateGlobalConfig()
	if err != nil {
		return err
	}
	return nil
}

// WriteGlobalConfig writes the global config into ~/.ddev.
func WriteGlobalConfig(config GlobalConfig) error {
	err := ValidateGlobalConfig()
	if err != nil {
		return err
	}
	cfgbytes, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	// Append current image information
	instructions := `
# You can turn off usage of the dba (phpmyadmin) container and/or
# ddev-ssh-agent containers with
# omit_containers["dba", "ddev-ssh-agent"]
# and you can opt in or out of sending instrumentation the ddev developers with
# instrumentation_opt_in: true # or false
#
# You can enable nfs mounting for all projects with
# nfs_mount_enabled: true
#
# You can inject environment variables into the web container with:
# web_environment:
# - SOMEENV=somevalue
# - SOMEOTHERENV=someothervalue

# In unusual cases the default value to wait to detect internet availability is too short.
# You can adjust this value higher to make it less likely that ddev will declare internet
# unavailable, but ddev may wait longer on some commands. This should not be set below the default 750
# ddev will ignore low values, as they're not useful
# internet_detection_timeout_ms: 750

# You can enable 'ddev start' to be interrupted by a failing hook with
# fail_on_hook_fail: true

# disable_http2: false
# Disable http2 on ddev-router if true

# instrumentation_user: <your_username> # can be used to give ddev specific info about who you are
# developer_mode: true # (defaults to false) is not used widely at this time.
# router_bind_all_interfaces: false  # (defaults to false)
#    If true, ddev-router will bind http/s, PHPMyAdmin, and MailHog ports on all
#    network interfaces instead of just localhost, so others on your local network can
#    access those ports. Note that this exposes the PHPMyAdmin and MailHog ports as well, which
#    can be a major security issue, so choose wisely. Consider omit_containers[dba] to avoid
#    exposing PHPMyAdmin.

# use_hardened_images: false
# With hardened images a container that is exposed to the internet is
# a harder target, although not as hard as a fully-secured host.
# sudo is removed, mailhog is removed, and since the web container
# is run only as the owning user, only project files might be changed
# if a CMS or PHP bug allowed creating or altering files, and
# permissions should not allow escalation.

# Let's Encrypt:
# This integration is entirely experimental; your mileage may vary.
# * Your host must be directly internet-connected.
# * DNS for the hostname must be set to point to the host in question
# * You must have router_bind_all_interfaces: true or else the Let's Encrypt certbot
#   process will not be able to process the IP address of the host (and nobody will be able to access your site)
# * You will need to add a startup script to start your sites after a host reboot.
# * If using several sites at a single top-level domain, you'll probably want to set
#   project_tld to that top-level domain. Otherwise, you can use additional-hostnames or
#   additional_fqdns.
#
# use_letsencrypt: false
# (Experimental, only useful on an internet-based server)
# Set to true if certificates are to be obtained via certbot on https://letsencrypt.org/

# letsencrypt_email: <email>
# Email to be used for experimental letsencrypt certificates

# auto_restart_containers: false
# Experimental
# If true, attempt to automatically restart projects/containers after reboot or docker restart.

# fail_on_hook_fail: false
# Decide whether 'ddev start' should be interrupted by a failing hook

`
	cfgbytes = append(cfgbytes, instructions...)

	err = os.WriteFile(GetGlobalConfigPath(), cfgbytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

// GetGlobalDdevDir returns ~/.ddev, the global caching directory
func GetGlobalDdevDir() string {
	userHome, err := homedir.Dir()
	if err != nil {
		logrus.Fatal("could not get home directory for current user. is it set?")
	}
	ddevDir := filepath.Join(userHome, ".ddev")

	// Create the directory if it is not already present.
	if _, err := os.Stat(ddevDir); os.IsNotExist(err) {
		// If they happen to be running as root/sudo, we won't create the directory
		// but act like we did. This should only happen for ddev hostname, which
		// doesn't need config or access to this dir anyway.
		if os.Geteuid() == 0 {
			return ddevDir
		}
		err = os.MkdirAll(ddevDir, 0755)
		if err != nil {
			logrus.Fatalf("Failed to create required directory %s, err: %v", ddevDir, err)
		}
	}
	// config.yaml is not allowed in ~/.ddev, can only result in disaster
	globalConfigYaml := filepath.Join(ddevDir, "config.yaml")
	if _, err := os.Stat(globalConfigYaml); err == nil {
		_ = os.Remove(filepath.Join(globalConfigYaml))
	}
	return ddevDir
}

// IsValidOmitContainers is a helper function to determine if a the OmitContainers array is valid
func IsValidOmitContainers(containerList []string) bool {
	for _, containerName := range containerList {
		if _, ok := ValidOmitContainers[containerName]; !ok {
			return false
		}
	}
	return true
}

// GetValidOmitContainers is a helper function that returns a list of valid containers for OmitContainers.
func GetValidOmitContainers() []string {
	s := make([]string, 0, len(ValidOmitContainers))

	for p := range ValidOmitContainers {
		s = append(s, p)
	}

	return s
}

// HostPostIsAllocated returns the project name that has allocated
// the port, or empty string.
func HostPostIsAllocated(port string) string {
	for project, item := range DdevGlobalConfig.ProjectList {
		if nodeps.ArrayContainsString(item.UsedHostPorts, port) {
			return project
		}
	}
	return ""
}

// CheckHostPortsAvailable checks GlobalDdev UsedHostPorts to see if requested ports are available.
func CheckHostPortsAvailable(projectName string, ports []string) error {
	for _, port := range ports {
		allocatedProject := HostPostIsAllocated(port)
		if allocatedProject != projectName && allocatedProject != "" {
			return fmt.Errorf("host port %s has already been allocated to project %s", port, allocatedProject)
		}
	}
	return nil
}

// GetFreePort gets an ephemeral port currently available, but also not
// listed in DdevGlobalConfig.UsedHostPorts
func GetFreePort(localIPAddr string) (string, error) {
	// Limit tries arbitrarily. It will normally succeed on first try.
	for i := 1; i < 1000; i++ {
		// From https://github.com/phayes/freeport/blob/master/freeport.go#L8
		// Ignores that the actual listener may be on a docker toolbox interface,
		// so this is just a heuristic.
		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		if err != nil {
			return "", err
		}

		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return "", err
		}
		port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
		// nolint: errcheck
		l.Close()

		// In the case of Docker Toolbox, the actual listening IP may be something else
		// like 192.168.99.100, so check that to make sure it's not currently occupied.
		conn, _ := net.Dial("tcp", localIPAddr+":"+port)
		if conn != nil {
			continue
		}

		if HostPostIsAllocated(port) != "" {
			continue
		}
		return port, nil
	}
	return "-1", fmt.Errorf("GetFreePort() failed to find a free port")

}

// ReservePorts adds the ProjectInfo if necessary and assigns the reserved ports
func ReservePorts(projectName string, ports []string) error {
	// If the project doesn't exist, add it.
	_, ok := DdevGlobalConfig.ProjectList[projectName]
	if !ok {
		DdevGlobalConfig.ProjectList[projectName] = &ProjectInfo{}
	}
	DdevGlobalConfig.ProjectList[projectName].UsedHostPorts = ports
	err := WriteGlobalConfig(DdevGlobalConfig)
	return err
}

// SetProjectAppRoot sets the approot in the ProjectInfo of global config
func SetProjectAppRoot(projectName string, appRoot string) error {
	// If the project doesn't exist, add it.
	_, ok := DdevGlobalConfig.ProjectList[projectName]
	if !ok {
		DdevGlobalConfig.ProjectList[projectName] = &ProjectInfo{}
	}
	// Can't use fileutil.FileExists because of import cycle.
	if _, err := os.Stat(appRoot); err != nil {
		return fmt.Errorf("project %s project root %s does not exist", projectName, appRoot)
	}
	if DdevGlobalConfig.ProjectList[projectName].AppRoot != "" && DdevGlobalConfig.ProjectList[projectName].AppRoot != appRoot {
		return fmt.Errorf("project %s project root is already set to %s, refusing to change it to %s; you can `ddev stop --unlist %s` and start again if the listed project root is in error", projectName, DdevGlobalConfig.ProjectList[projectName].AppRoot, appRoot, projectName)
	}
	DdevGlobalConfig.ProjectList[projectName].AppRoot = appRoot
	err := WriteGlobalConfig(DdevGlobalConfig)
	return err
}

// GetProject returns a project given name provided,
// or nil if not found.
func GetProject(projectName string) *ProjectInfo {
	project, ok := DdevGlobalConfig.ProjectList[projectName]
	if !ok {
		return nil
	}
	return project
}

// RemoveProjectInfo removes the ProjectInfo line for a project
func RemoveProjectInfo(projectName string) error {
	_, ok := DdevGlobalConfig.ProjectList[projectName]
	if ok {
		delete(DdevGlobalConfig.ProjectList, projectName)
		err := WriteGlobalConfig(DdevGlobalConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetGlobalProjectList returns the global project list map
func GetGlobalProjectList() map[string]*ProjectInfo {
	return DdevGlobalConfig.ProjectList
}

// GetCAROOT is just a wrapper on global config
func GetCAROOT() string {
	return DdevGlobalConfig.MkcertCARoot
}

// readCAROOT() verifies that the mkcert command is available and its CA keys readable.
// 1. Find out CAROOT
// 2. Look there to see if key/crt are readable
// 3. If not, see if mkcert is even available, return empty

func readCAROOT() string {

	_, err := exec.LookPath("mkcert")
	if err != nil {
		return ""
	}

	out, err := exec.Command("mkcert", "-CAROOT").Output()
	if err != nil {
		return ""
	}
	root := strings.Trim(string(out), "\n")
	if !fileIsReadable(filepath.Join(root, "rootCA-key.pem")) || !fileExists(filepath.Join(root, "rootCA.pem")) {
		return ""
	}

	return root
}

// fileIsReadable checks to make sure a file exists and is readable
// Copied from fileutil because of import cycles
func fileIsReadable(name string) bool {
	file, err := os.OpenFile(name, os.O_RDONLY, 0666)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

// fileExists checks a file's existence
// Copied from fileutil because of import cycles
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// IsInternetActiveAlreadyChecked just flags whether it's been checked
var IsInternetActiveAlreadyChecked = false

// IsInternetActiveResult is the result of the check
var IsInternetActiveResult = false

// IsInternetActiveNetResolver wraps the standard DNS resolver.
// In order to override net.DefaultResolver with a stub, we have to define an
// interface on our own since there is none from the standard library.
var IsInternetActiveNetResolver interface {
	LookupHost(ctx context.Context, host string) (addrs []string, err error)
} = net.DefaultResolver

//IsInternetActive checks to see if we have a viable
// internet connection. It just tries a quick DNS query.
// This requires that the named record be query-able.
// This check will only be made once per command run.
func IsInternetActive() bool {
	// if this was already checked, return the result
	if IsInternetActiveAlreadyChecked {
		return IsInternetActiveResult
	}

	timeout := time.Duration(DdevGlobalConfig.InternetDetectionTimeout) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	randomURL := nodeps.RandomString(10) + ".ddev.site"
	addrs, err := IsInternetActiveNetResolver.LookupHost(ctx, randomURL)

	// Internet is active (active == true) if both err and ctx.Err() were nil
	active := err == nil && ctx.Err() == nil
	if os.Getenv("DDEV_DEBUG") != "" {
		if active == false {
			output.UserErr.Println("Internet connection not detected, DNS may not work, see https://ddev.readthedocs.io/en/stable/users/faq/ for info.")
		}
		output.UserErr.Printf("IsInternetActive DEBUG: err=%v ctx.Err()=%v addrs=%v IsInternetactive==%v, randomURL=%v internet_detection_timeout_ms=%dms\n", err, ctx.Err(), addrs, active, randomURL, DdevGlobalConfig.InternetDetectionTimeout)
	}

	// remember the result to not call this twice
	IsInternetActiveAlreadyChecked = true
	IsInternetActiveResult = active

	return active
}
