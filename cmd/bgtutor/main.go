// Command bgtutor is the Bulgarian Podcast Tutor, a sub-project of totalrecall.
//
//	bgtutor serve     run the MCP server (Streamable HTTP) over the episode library
//	bgtutor validate  check episode folders against bgtutor/FORMAT.md
//	bgtutor publish   mark a valid draft episode as ready
//
// Episodes are prepared by a coding agent (Claude Code, Codex, ...) following
// bgtutor/PREPARE.md, not by this binary. See bgtutor/README.md.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/snonux/totalrecall/internal"
)

func main() {
	root := &cobra.Command{
		Use:           "bgtutor",
		Short:         "Bulgarian Podcast Tutor: prepared podcast lessons served over MCP",
		Version:       internal.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String("data-dir", envOr("BGTUTOR_DATA_DIR", "bgtutor/data"),
		"library directory holding episodes/ and vocabulary/ (env BGTUTOR_DATA_DIR)")
	root.AddCommand(newServeCmd(), newValidateCmd(), newPublishCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "bgtutor:", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
