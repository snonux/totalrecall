# bgtutor: Bulgarian Podcast Tutor

A sub-project of totalrecall. It turns English podcasts into voice-guided
Bulgarian listening lessons: a voice AI (Claude or ChatGPT voice mode) pulls
prepared podcast content from this MCP server paragraph by paragraph,
translates it into Bulgarian live, reads it aloud, and teaches grammar and
vocabulary along the way.

```
English transcript ──► bgtutor prepare (Gemini, offline) ──► bgtutor/data/episodes/<id>/
                                                                  │
                     voice AI ◄── MCP over HTTPS ◄── bgtutor serve ┘
```

## Build

```bash
go build -o bgtutor ./cmd/bgtutor
```

## 1. Prepare an episode

Needs `GOOGLE_API_KEY`, like the rest of totalrecall.

```bash
./bgtutor prepare transcript.txt \
  --id 002-morning-news --title "Morning News" \
  --speaker "Host:Maria:news anchor" --speaker "Guest:Peter:economist" \
  --difficulty B1 --minutes 25 --audio episode.mp3
```

- The transcript is plain text; `Speaker: text` lines help but are not required.
- `--difficulty` is the CEFR level the notes are pitched at.
- `--reference notes.txt` passes a grammar reference or textbook excerpt so
  notes use your course's terminology (optional).
- Long transcripts are sent in chunks (`--chunk-words`, default 1200). The
  episode stays `draft` until every chunk succeeded and validated, so the server
  never serves a half-prepared episode.

The prompt lives in `internal/bgtutor/prepare/prompt.md`. The folder format is
in [FORMAT.md](FORMAT.md). `./bgtutor validate` checks every episode.

A small hand-prepared test episode ships in
`bgtutor/data/episodes/001-cooking-basics` (source transcript in
`bgtutor/examples/`).

## 2. Run the MCP server

```bash
./bgtutor serve                       # http://127.0.0.1:8080/mcp, no auth, local only
BGTUTOR_TOKEN=$(openssl rand -hex 32) ./bgtutor serve --addr 127.0.0.1:8080
```

| Setting | Flag / env | Default |
|---|---|---|
| Library directory | `--data-dir` / `BGTUTOR_DATA_DIR` | `bgtutor/data` |
| Listen address | `--addr` / `BGTUTOR_ADDR` | `127.0.0.1:8080` |
| Bearer token | `BGTUTOR_TOKEN` | none; required for non-localhost addresses |

With a token set, every `/mcp` request needs `Authorization: Bearer <token>`
or `?token=<token>` on the URL (for connector UIs that only take a URL).
`/healthz` is always open.

Transport is Streamable HTTP in stateless mode: the voice AI passes the episode
id and paragraph index on every call, and the server keeps no session state.

### Tools

| Tool | Input | Returns |
|---|---|---|
| `list_episodes` | none | Episodes with title, topic, difficulty, speakers, paragraph count, `ready` (and why not) |
| `get_paragraph` | `episode_id`, `index` (1-based) | One paragraph: `english` (primary), `bulgarian_reference` (fallback, with a note saying so), grammar and vocabulary notes, background, speaker, `position` ("3 of 9"), `is_last`, `next_index` |
| `save_vocabulary` | `term`, optional `kind` (word, phrase, rule), `translation`, `note`, `episode_id`, `paragraph_index` | The saved item; saving the same term again bumps `times_saved` |
| `list_vocabulary` | optional `query`, `kind`, `episode_id`, `limit` | Saved items, newest first |

Errors (unknown episode, index out of range, episode not prepared) come back
as tool errors with a message the AI can read out, e.g. "Valid indexes are 1
to 9". The session instructions for the voice AI are sent on initialize; see
`Instructions` in `internal/bgtutor/mcpserver/server.go`.

The vocabulary notebook is `bgtutor/data/vocabulary/saved.json` (git-ignored).

## Container image

```bash
docker build -f bgtutor/Dockerfile -t bgtutor:0.1.0 .   # from the repo root
docker run -e BGTUTOR_TOKEN=... -v $PWD/bgtutor/data:/data -p 8080:8080 bgtutor:0.1.0
```

The image runs only `bgtutor serve` (static binary on distroless), with the
library at `/data`. The k3s deployment lives in snonux/conf under `f3s/bgtutor`.

## 3. Connect a voice AI (not done yet)

Both Claude (custom connector) and ChatGPT (developer mode connector) need a
public HTTPS URL ending in `/mcp`. The plan is to run `bgtutor serve` with a
token behind an HTTPS tunnel or reverse proxy. A tunnel reaches the server via
localhost with a public `Host` header; that works when a token is set.
