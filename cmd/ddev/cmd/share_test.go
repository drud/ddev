package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	asrt "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"testing"
)

// TestShareCmd tests `ddev share`
func TestShareCmd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping because unreliable on Windows due to DNS lookup failure")
	}
	assert := asrt.New(t)
	urlRead := false

	site := TestSites[0]
	defer site.Chdir()()

	// Configure ddev/ngrok to use json output to stdout
	cmd := exec.Command(DdevBin, "config", "--ngrok-args", "-log stdout -log-format=json")
	err := cmd.Start()
	require.NoError(t, err)
	err = cmd.Wait()
	require.NoError(t, err)

	cmd = exec.Command(DdevBin, "share", "--use-http")
	cmdReader, err := cmd.StdoutPipe()
	require.NoError(t, err)
	scanner := bufio.NewScanner(cmdReader)

	// Make absolutely sure the ngrok process gets killed off, because otherwise
	// the testbot (windows) can remain occupied forever.
	// nolint: errcheck
	defer pKill(cmd)

	// Read through the ngrok json output until we get the url it has opened
	go func() {
		for scanner.Scan() {
			logLine := scanner.Text()
			logData := make(map[string]string)

			err := json.Unmarshal([]byte(logLine), &logData)
			if err != nil {
				switch err.(type) {
				case *json.SyntaxError:
					continue
				default:
					t.Errorf("failed unmarshalling %v: %v", logLine, err)
					break
				}
			}
			// If URL is provided, try to hit it and look for expected response
			if url, ok := logData["url"]; ok {
				resp, err := http.Get(url + site.Safe200URIWithExpectation.URI)
				if err != nil {
					t.Logf("http.Get on url=%s failed, err=%v", url+site.Safe200URIWithExpectation.URI, err)
					err = pKill(cmd)
					assert.NoError(err)
					return
				}
				//nolint: errcheck
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				assert.NoError(err)
				assert.Contains(string(body), site.Safe200URIWithExpectation.Expect)
				urlRead = true
				err = pKill(cmd)
				assert.NoError(err)
				return
			}
		}
	}()
	err = cmd.Start()
	require.NoError(t, err)
	err = cmd.Wait()
	t.Logf("cmd.Wait() err: %v", err)
	assert.True(urlRead)
	_ = cmdReader.Close()
}

// pKill kills a started cmd; If windows, it shells out to the
// taskkill command.
func pKill(cmd *exec.Cmd) error {
	var err error
	if cmd == nil {
		return fmt.Errorf("pKill: cmd is nill")
	}
	if runtime.GOOS == "windows" {
		// Windows has a completely different process model, no SIGCHLD,
		// no killing of subprocesses. I wasn't successful in finding a way
		// to properly kill a process set using golang; rfay 20190622
		kill := exec.Command("TASKKILL", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		kill.Stderr = os.Stderr
		kill.Stdout = os.Stdout
		err = kill.Run()
	} else {
		err = cmd.Process.Kill()
	}
	return err
}
