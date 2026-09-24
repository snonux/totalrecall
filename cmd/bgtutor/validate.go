package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/snonux/totalrecall/internal/bgtutor"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Check every episode folder against the episode format",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dataDir, _ := cmd.Flags().GetString("data-dir")
			eps, err := bgtutor.NewLibrary(filepath.Join(dataDir, "episodes")).Episodes()
			if err != nil {
				return err
			}
			bad := 0
			for _, ep := range eps {
				if ep.Ready() {
					fmt.Printf("ok        %s (%d paragraphs)\n", ep.ID, len(ep.Paragraphs))
				} else {
					bad++
					fmt.Printf("NOT READY %s: %s\n", ep.ID, ep.Problem)
				}
			}
			if bad > 0 {
				return fmt.Errorf("%d of %d episodes are not ready", bad, len(eps))
			}
			fmt.Printf("%d episodes, all ready\n", len(eps))
			return nil
		},
	}
}
