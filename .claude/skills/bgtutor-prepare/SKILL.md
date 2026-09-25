---
name: bgtutor-prepare
description: Prepare a Bulgarian Podcast Tutor (bgtutor) episode from an English podcast transcript - split into paragraphs, translate to Bulgarian, write grammar and vocabulary notes, validate and publish. Use when asked to prepare, add or convert a podcast episode or transcript for bgtutor.
---

Follow `bgtutor/PREPARE.md` in this repository step by step. It is the single
source of truth for the procedure and the content rules; `bgtutor/FORMAT.md`
defines the file format, and `bgtutor/data/episodes/001-cooking-basics/` is a
finished example.

You do the translation and note writing yourself. Do not call an external LLM
API. Finish with `go run ./cmd/bgtutor validate <id>` passing and
`go run ./cmd/bgtutor publish <id>`.
