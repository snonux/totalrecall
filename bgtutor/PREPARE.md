# Preparing a bgtutor episode

This is the procedure a coding agent follows to turn an English podcast
transcript into a bgtutor episode folder. It is written for any harness that
can read and write files and run a shell command: Claude Code (the
`bgtutor-prepare` skill points here), Codex, or a Claude cloud session. You,
the agent, do the splitting, translating and note writing yourself. No API
calls and no API keys are involved.

The file format is defined in [FORMAT.md](FORMAT.md). A finished example is
`bgtutor/data/episodes/001-cooking-basics/`, made from
`bgtutor/examples/001-cooking-basics.txt`. Read both before you start.

## Inputs

Ask the person for anything missing that you can't infer:

- **Transcript** (required): English text, ideally with `Speaker: text` lines.
  Audio alone is not enough; you need a transcript.
- **Episode id**: `NNN-short-slug`, the next free number in
  `bgtutor/data/episodes/` (e.g. `002-morning-news`).
- **Title, speakers** (label, name, role), topic, a one-sentence description,
  audio length in minutes. Infer them from the transcript when you can.
- **Learner level** (CEFR). Default `B1` if not given.
- **Course reference** (optional): a grammar book excerpt or course notes. If
  given, use its terminology and don't explain things beyond its level.

## Steps

1. Create `bgtutor/data/episodes/<id>/meta.json` with `"status": "draft"`,
   `"format_version": 1`, and `prepared_by` naming your harness and model
   (e.g. `"Claude Code, following bgtutor/PREPARE.md"`).
2. Split the transcript into paragraphs (rules below) and write
   `paragraphs.json`, numbered from 1. For a long transcript, work through it
   in sections and append to the file as you go; keep a running list of the
   vocabulary you have already explained so notes don't repeat.
3. If there is an audio file, copy it into the folder as `audio.<ext>` and set
   `source.audio_file` in meta.json.
4. Run `go run ./cmd/bgtutor validate <id>` from the repository root and fix
   every problem it lists.
5. Re-read your translations once as a native Bulgarian speaker would. Fix
   anything that sounds translated rather than spoken.
6. Run `go run ./cmd/bgtutor publish <id>`. This flips the status to `ready`;
   only then does the MCP server serve the episode.
7. Tell the person the episode id, the paragraph count, and how to upload it
   (on f3s: `just upload-episode <folder>` in `snonux/conf` `f3s/bgtutor`).

## Paragraphs

A voice tutor later reads each paragraph to the learner once, translating the
English into Bulgarian live and using your notes to teach.

- One paragraph = one speaker, one idea, roughly 40 to 120 English words.
- Split long monologues at sentence boundaries. Never merge different speakers.
  A short turn ("What do we need?") is its own paragraph.
- Keep `english` faithful to the transcript. You may drop filler ("um", "you
  know", false starts) and fix obvious transcription errors, but do not
  summarise or reword.
- `speaker` must be one of the labels in `meta.speakers`. If the transcript
  doesn't say who speaks, infer it from context.

## Per-paragraph content

- `bulgarian_reference`: a natural, idiomatic Bulgarian translation, as a
  native speaker would say it on a podcast. Spoken register, not literary.
  It is the tutor's fallback, so accuracy matters more than elegance. Use
  Bulgarian quotation marks „…“ and an en dash – for dashes.
- `grammar_notes`: 1 to 3 short notes (one or two sentences each, in English)
  on grammar that actually appears in *your* translation: verb aspect, the
  definite article, future with ще, aorist vs imperfect, the renarrative mood,
  clitic pronoun order, да-constructions, count forms after numbers, word
  order. Quote the Bulgarian words the note is about. Pitch them at the
  learner's level: explain what they would stumble over, skip what they know.
  No generic lectures.
- `vocabulary_notes`: 2 to 5 key or tricky words and idioms from your
  translation, as `{bg, en, note}`. Give verbs in the dictionary form (1st
  person singular present) with the aspect pair when useful (`покажа /
  показвам`); give noun gender when it isn't obvious. Prefer words the learner
  will meet again; skip obvious cognates like „телефон“. Don't repeat words
  already explained earlier in the episode. `note` may be an empty string.
- `background`: one or two English sentences of cultural or topical context
  when it helps understanding (a Bulgarian custom, a reference in the
  conversation), otherwise `null`.
