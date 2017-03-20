package cmd

import (
	"log"
	"os"
	"strings"
	"testing"

	"github.com/drud/ddev/pkg/plugins/platform"
	"github.com/drud/ddev/pkg/testcommon"
	"github.com/drud/drud-go/utils/dockerutil"
	"github.com/drud/drud-go/utils/network"
	"github.com/drud/drud-go/utils/system"
	"github.com/stretchr/testify/assert"
)

var skipComposeTests bool

// TestDevAddWP tests a `drud Dev add` on a wp site
func TestDevAddSites(t *testing.T) {
	if skipComposeTests {
		t.Skip("Compose tests being skipped.")
	}
	assert := assert.New(t)
	for _, site := range DevTestSites {
		cleanup := testcommon.Chdir(site.Path)

		// test that you get an error when you run with no args
		args := []string{"start"}
		out, err := system.RunCommand(DdevBin, args)
		if err != nil {
			log.Println("Error Output from ddev start:", out)
		}
		assert.NoError(err)
		assert.Contains(string(out), "Your application can be reached at")

		app := platform.PluginMap[strings.ToLower(plugin)]
		err = app.Init()

		assert.Equal(true, dockerutil.IsRunning(app.ContainerName()+"-web"))
		assert.Equal(true, dockerutil.IsRunning(app.ContainerName()+"-db"))

		o := network.NewHTTPOptions("http://127.0.0.1/core/install.php")
		o.ExpectedStatus = 200
		o.Timeout = 180
		o.Headers["Host"] = app.HostName()
		err = network.EnsureHTTPStatus(o)
		assert.NoError(err)

		cleanup()
	}
}

func init() {
	if os.Getenv("SKIP_COMPOSE_TESTS") == "true" {
		skipComposeTests = true
	}
}
