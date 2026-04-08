package main

import (
	"codeberg.org/snonux/totalrecall/internal/cli"
	"codeberg.org/snonux/totalrecall/internal/story"
)

// newStoryRunner wires a story.Runner from CLI flags and API keys.
func newStoryRunner(flags *cli.Flags) story.StoryRunner {
	return story.NewRunner(&story.RunnerConfig{
		APIKey:         cli.GetGoogleAPIKey(),
		TextModel:      flags.NanoBananaTextModel,
		ImageModel:     flags.NanoBananaModel,
		ImageTextModel: flags.NanoBananaTextModel,
		OutputDir:      ".",
		Style:          flags.StoryStyle,
		Theme:          flags.StoryTheme,
		UltraRealistic: storyUltraRealistic(flags.StoryNoUltraRealistic, flags.StoryUltraRealistic),
		NarratorVoice:  flags.NarratorVoice,
		NarrateEnabled: flags.NarrateEnabled,
		Slug:           flags.StorySlug,
	})
}

// storyUltraRealistic converts the --ultra-realistic / --no-ultra-realistic
// bool flags into a *bool for RunnerConfig.
//   - --ultra-realistic → pointer to true (force photorealistic panels)
//   - --no-ultra-realistic → pointer to false (force standard comic style)
//   - neither flag set → nil (runner picks randomly 50/50 each run)
func storyUltraRealistic(noUltraRealistic, ultraRealistic bool) *bool {
	if ultraRealistic {
		v := true
		return &v
	}
	if noUltraRealistic {
		v := false
		return &v
	}
	return nil // nil → random pick in NewRunner
}
