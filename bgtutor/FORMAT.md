# Episode folder format (v1)

The library is plain folders on disk. Adding an episode means dropping a folder
into `episodes/`. No database.

```
bgtutor/data/
├── episodes/
│   ├── 001-cooking-basics/
│   │   ├── meta.json          # required
│   │   ├── paragraphs.json    # required
│   │   └── audio.mp3          # optional, reference only (not served)
│   └── 002-.../
└── vocabulary/
    └── saved.json             # created by save_vocabulary on first use
```

The folder name is the **episode id**. Use `NNN-short-slug` (lowercase letters,
digits, dashes). The id is what the voice AI passes to `get_paragraph`.

## meta.json

```json
{
  "format_version": 1,
  "title": "Cooking Basics: Banitsa for Beginners",
  "topic": "Food and cooking",
  "description": "A host and a chef talk through making banitsa at home.",
  "difficulty": "A2",
  "duration_minutes": 3,
  "speakers": [
    {"label": "Host", "name": "Anna", "role": "podcast host"},
    {"label": "Guest", "name": "Georgi", "role": "chef"}
  ],
  "source": {"podcast": "...", "episode_url": "...", "audio_file": "audio.mp3"},
  "status": "ready",
  "prepared_at": "2026-09-24",
  "prepared_by": "Claude Code, following bgtutor/PREPARE.md"
}
```

| Field | Required | Notes |
|---|---|---|
| `format_version` | yes | Always `1` for now. |
| `title` | yes | Shown to the learner when choosing an episode. |
| `topic` | no | Short topic label. |
| `description` | no | One or two sentences. |
| `difficulty` | no | CEFR level of the Bulgarian (`A1` to `C2`). |
| `duration_minutes` | no | Length of the original audio. |
| `speakers` | no | `label` must match the `speaker` field used in paragraphs. |
| `source` | no | Free-form provenance. `audio_file` is relative to the folder. |
| `status` | yes | `ready` or `draft`. Only `ready` episodes can be played. |

The paragraph count is **not** stored in meta; the server counts
`paragraphs.json`, so the two can never disagree.

## paragraphs.json

A JSON array of paragraph records, ordered, with `index` starting at **1** and
increasing by one with no gaps.

```json
[
  {
    "index": 1,
    "speaker": "Host",
    "english": "So today we're going to talk about ...",
    "bulgarian_reference": "Днес ще говорим за ...",
    "grammar_notes": [
      "Future tense: 'ще' + present-tense form (ще говорим = we will talk). 'ще' never changes."
    ],
    "vocabulary_notes": [
      {"bg": "говоря", "en": "to speak, to talk", "note": "imperfective; perfective: поговоря"}
    ],
    "background": "Optional cultural or topical context, or null"
  }
]
```

| Field | Required | Notes |
|---|---|---|
| `index` | yes | 1-based position. |
| `speaker` | yes | Label from `meta.speakers`, e.g. `Host`, `Guest`, `Narrator`. |
| `english` | yes | Original transcript text. **Primary source** for the voice AI. |
| `bulgarian_reference` | yes | Reference translation, fallback only. |
| `grammar_notes` | yes | List of strings, 1 to 3 per paragraph. May be empty. |
| `vocabulary_notes` | yes | List of `{bg, en, note}`; `note` may be empty. May be empty. |
| `background` | no | String or `null`. |

### Paragraph size

A paragraph is one comfortable listening chunk: roughly **40 to 120 English
words**, one speaker, one idea. Long monologues are split at sentence
boundaries; very short back-and-forth turns may be merged only if they are by
the same speaker. This is a starting point and should be tuned after real
sessions (build step 7).

## vocabulary/saved.json

Written by the server. It is your personal notebook, so it is git-ignored.

```json
{
  "format_version": 1,
  "items": [
    {
      "id": 1,
      "term": "тесто",
      "kind": "word",
      "translation": "dough",
      "note": "neuter noun; тестото = the dough",
      "episode_id": "001-cooking-basics",
      "paragraph_index": 2,
      "context": "The English sentence it came from",
      "saved_at": "2026-09-24T10:30:00Z",
      "times_saved": 1
    }
  ]
}
```

`kind` is one of `word`, `phrase`, `rule`. Saving the same `term` + `kind`
again does not create a duplicate: it bumps `times_saved` and appends the new
note, which is a useful "this keeps tripping me up" signal for review.

## Validation

`bgtutor validate [id...]` checks folders against this spec and lists every
problem; `bgtutor publish <id>` flips a valid draft to `ready`. The server also validates on load; an invalid or `draft` episode is
listed with `ready: false` and a reason, and `get_paragraph` refuses it.
