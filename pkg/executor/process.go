package executor

import (
	"os"

	"github.com/kubeshop/testkube/pkg/process"
	"github.com/kubeshop/testkube/pkg/runner/output"
)

// Run runs executor process wrapped in json line output
// wraps stdout lines into JSON chunks we want it to have common interface for agent
// stdin <- testkube.Execution, stdout <- stream of json logs
// LoggedExecuteInDir will put wrapped JSON output to stdout AND get RAW output into out var
// json logs can be processed later on watch of pod logs
func Run(dir string, command string, arguments ...string) (out []byte, err error) {
	return process.LoggedExecuteInDir(dir, output.NewJSONWrapWriter(os.Stdout), command, arguments...)
}
