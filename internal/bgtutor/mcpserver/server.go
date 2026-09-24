// Package mcpserver exposes the Bulgarian Podcast Tutor library over MCP
// (Streamable HTTP). The server is stateless: the voice AI passes the episode
// id and paragraph index on every call, so no session state is kept.
package mcpserver

import (
	"context"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/snonux/totalrecall/internal"
	"github.com/snonux/totalrecall/internal/bgtutor"
)

// Instructions is sent to the voice AI on initialize. It encodes the live
// session flow from the project plan: pick an episode, go paragraph by
// paragraph, translate English live, fall back to the reference only if needed.
const Instructions = `You are a friendly Bulgarian listening tutor. This server holds English podcast
episodes, prepared paragraph by paragraph, and the learner's vocabulary notebook.

Session flow:
1. Call list_episodes, read out the ready episodes by title, and ask which one
   the learner wants to hear.
2. Call get_paragraph with index 1. Only fetch the next paragraph (next_index)
   when the learner is ready to move on. Never fetch ahead or fetch the whole
   episode. Stop after the paragraph with is_last = true.
3. For each paragraph: translate "english" into natural Bulgarian yourself and
   read it aloud, signalling the speaker ("водещият казва...", "гостът
   отговаря...") and shifting tone slightly per speaker. Adapt the difficulty to
   the learner. "english" is the source; use "bulgarian_reference" only as a
   fallback when unsure, or when the learner asks for it.
4. Teach, don't lecture: after reading, point out one or two things from
   grammar_notes, vocabulary_notes or background, briefly.
5. The learner may ask you to repeat, slow down, translate a word or phrase to
   English, or explain grammar. When a word or rule is new to them, offer to
   save it and call save_vocabulary with the episode_id and paragraph index.
6. For review sessions, call list_vocabulary.`

// New builds the MCP server over dataDir, which holds episodes/ and
// vocabulary/saved.json (see bgtutor/FORMAT.md).
func New(dataDir string) *mcp.Server {
	lib := bgtutor.NewLibrary(filepath.Join(dataDir, "episodes"))
	nb := bgtutor.NewNotebook(filepath.Join(dataDir, "vocabulary", "saved.json"), lib)
	t := &tools{lib: lib, nb: nb}

	s := mcp.NewServer(&mcp.Implementation{
		Name:    "bulgarian-podcast-tutor",
		Title:   "Bulgarian Podcast Tutor",
		Version: internal.Version,
	}, &mcp.ServerOptions{Instructions: Instructions})

	mcp.AddTool(s, &mcp.Tool{
		Name: "list_episodes",
		Description: "List the podcast episodes in the library with title, topic, difficulty, " +
			"speakers and paragraph count. Only episodes with ready=true can be played.",
		Annotations: readOnly(),
	}, t.listEpisodes)
	mcp.AddTool(s, &mcp.Tool{
		Name: "get_paragraph",
		Description: "Return ONE paragraph of an episode, plus a position marker ('3 of 12'), " +
			"is_last and next_index. Stateless: pass the index each time, starting at 1. " +
			"'english' is the primary source to translate live; 'bulgarian_reference' is a " +
			"fallback only. Includes grammar_notes, vocabulary_notes and background for teaching.",
		Annotations: readOnly(),
	}, t.getParagraph)
	mcp.AddTool(s, &mcp.Tool{
		Name: "save_vocabulary",
		Description: "Save a Bulgarian word, phrase or grammar rule the learner doesn't know yet " +
			"to their vocabulary notebook. Pass episode_id and paragraph_index so the source " +
			"sentence is stored for context. Saving the same term again is safe.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, t.saveVocabulary)
	mcp.AddTool(s, &mcp.Tool{
		Name: "list_vocabulary",
		Description: "List saved vocabulary for review, newest first. Optional filters: query " +
			"(matches term, translation or note), kind, episode_id, limit.",
		Annotations: readOnly(),
	}, t.listVocabulary)
	return s
}

func readOnly() *mcp.ToolAnnotations { return &mcp.ToolAnnotations{ReadOnlyHint: true} }

type tools struct {
	lib *bgtutor.Library
	nb  *bgtutor.Notebook
}

type listEpisodesIn struct{}

type listEpisodesOut struct {
	Count    int                      `json:"count"`
	Episodes []bgtutor.EpisodeSummary `json:"episodes"`
}

func (t *tools) listEpisodes(_ context.Context, _ *mcp.CallToolRequest, _ listEpisodesIn) (*mcp.CallToolResult, listEpisodesOut, error) {
	eps, err := t.lib.ListEpisodes()
	if err != nil {
		return nil, listEpisodesOut{}, err
	}
	if eps == nil {
		eps = []bgtutor.EpisodeSummary{}
	}
	return nil, listEpisodesOut{Count: len(eps), Episodes: eps}, nil
}

type getParagraphIn struct {
	EpisodeID string `json:"episode_id" jsonschema:"episode id from list_episodes, e.g. 001-cooking-basics"`
	Index     int    `json:"index" jsonschema:"1-based paragraph index; start at 1 and use next_index to continue"`
}

func (t *tools) getParagraph(_ context.Context, _ *mcp.CallToolRequest, in getParagraphIn) (*mcp.CallToolResult, *bgtutor.ParagraphPayload, error) {
	p, err := t.lib.Paragraph(in.EpisodeID, in.Index)
	return nil, p, err
}

type saveVocabularyIn struct {
	Term           string `json:"term" jsonschema:"the Bulgarian word or phrase, or a short name for the grammar rule"`
	Kind           string `json:"kind,omitempty" jsonschema:"one of word, phrase, rule (default word)"`
	Translation    string `json:"translation,omitempty" jsonschema:"English meaning"`
	Note           string `json:"note,omitempty" jsonschema:"explanation, e.g. aspect, gender, or the rule itself"`
	EpisodeID      string `json:"episode_id,omitempty" jsonschema:"episode the term came from"`
	ParagraphIndex int    `json:"paragraph_index,omitempty" jsonschema:"paragraph the term came from"`
}

func (t *tools) saveVocabulary(_ context.Context, _ *mcp.CallToolRequest, in saveVocabularyIn) (*mcp.CallToolResult, *bgtutor.SaveResult, error) {
	res, err := t.nb.Save(bgtutor.SaveRequest(in))
	return nil, res, err
}

type listVocabularyIn struct {
	Query     string `json:"query,omitempty" jsonschema:"substring to search in term, translation and note"`
	Kind      string `json:"kind,omitempty" jsonschema:"only this kind: word, phrase or rule"`
	EpisodeID string `json:"episode_id,omitempty" jsonschema:"only items saved from this episode"`
	Limit     int    `json:"limit,omitempty" jsonschema:"maximum number of items to return (default 50, max 500)"`
}

func (t *tools) listVocabulary(_ context.Context, _ *mcp.CallToolRequest, in listVocabularyIn) (*mcp.CallToolResult, *bgtutor.ListResult, error) {
	res, err := t.nb.List(bgtutor.ListRequest(in))
	return nil, res, err
}
