package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/snonux/totalrecall/internal/bgtutor"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [EPISODE_ID...]",
		Short: "Check episode folders against bgtutor/FORMAT.md (all episodes if none given)",
		Long: `Check episode folders against bgtutor/FORMAT.md and list every problem.
A draft episode with no problems is reported as valid; publish it with
"bgtutor publish EPISODE_ID".`,
		RunE: func(cmd *cobra.Command, ids []string) error {
			eps, err := loadEpisodes(cmd, ids)
			if err != nil {
				return err
			}
			bad := 0
			for _, ep := range eps {
				if !reportEpisode(ep) {
					bad++
				}
			}
			if bad > 0 {
				return fmt.Errorf("%d of %d episodes have problems", bad, len(eps))
			}
			return nil
		},
	}
}

func newPublishCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "publish EPISODE_ID",
		Short: "Validate a draft episode and mark it ready, so the MCP server serves it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !bgtutor.ValidEpisodeID(args[0]) {
				return fmt.Errorf("invalid episode id %q", args[0])
			}
			ep, err := bgtutor.Publish(filepath.Join(episodesDir(cmd), args[0]))
			if err != nil {
				reportEpisode(ep)
				return err
			}
			fmt.Printf("published %s (%d paragraphs)\n", ep.ID, len(ep.Paragraphs))
			return nil
		},
	}
}

// loadEpisodes loads the named episodes, or all of them when ids is empty.
func loadEpisodes(cmd *cobra.Command, ids []string) ([]*bgtutor.Episode, error) {
	dir := episodesDir(cmd)
	if len(ids) == 0 {
		return bgtutor.NewLibrary(dir).Episodes()
	}
	eps := make([]*bgtutor.Episode, 0, len(ids))
	for _, id := range ids {
		if !bgtutor.ValidEpisodeID(id) {
			return nil, fmt.Errorf("invalid episode id %q", id)
		}
		eps = append(eps, bgtutor.LoadEpisode(filepath.Join(dir, id)))
	}
	return eps, nil
}

// reportEpisode prints one status line plus every content error, and
// reports whether the episode's content is valid.
func reportEpisode(ep *bgtutor.Episode) bool {
	switch {
	case len(ep.Errors) > 0:
		fmt.Printf("INVALID %s\n", ep.ID)
		for _, e := range ep.Errors {
			fmt.Printf("  - %s\n", e)
		}
		return false
	case ep.Ready():
		fmt.Printf("ready   %s (%d paragraphs)\n", ep.ID, len(ep.Paragraphs))
	default:
		fmt.Printf("valid   %s (%d paragraphs, status %q; run bgtutor publish %s)\n",
			ep.ID, len(ep.Paragraphs), ep.Meta.Status, ep.ID)
	}
	return true
}

func episodesDir(cmd *cobra.Command) string {
	dataDir, _ := cmd.Flags().GetString("data-dir")
	return filepath.Join(dataDir, "episodes")
}
