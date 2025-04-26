package examples

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"myceliumweb.org/mycelium/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestExamples(t *testing.T) {
	makeGoExec(t, "../build/out/sp", "../cmd/sp")
	makeGoExec(t, "../build/out/myc", "../cmd/myc")
	ents, err := os.ReadDir(".")
	require.NoError(t, err)

	for _, ent := range ents {
		if ent.IsDir() {
			exampleDir := ent.Name()
			t.Run(ent.Name(), func(t *testing.T) {
				if strings.HasSuffix(ent.Name(), "sp-gui") {
					// sp-gui opens a window
					t.SkipNow()
				}

				t.Parallel()
				ctx := testutil.Context(t)
				cmd := exec.CommandContext(ctx, "./main.sh")
				cmd.Env = []string{"PATH=/bin/:../../build/out"}
				cmd.Stdout = os.Stderr
				cmd.Stderr = os.Stderr
				cmd.Dir = exampleDir
				require.NoError(t, cmd.Run())
			})
		}
	}
}

func makeGoExec(t testing.TB, out, mainPkg string) {
	ctx := testutil.Context(t)
	cmd := exec.CommandContext(ctx, "go", "build", "-o", out, mainPkg)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())
}
