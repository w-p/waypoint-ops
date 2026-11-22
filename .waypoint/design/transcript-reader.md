# Design: Claude Code transcript reader

**Status:** Provisional, 2026-11-22; built, awaiting review (see the end of this note). Built to review running, like the first slice
(`design/first-slice.md`); this note is updated after review. Intent: `conops.md` §2 (tool-neutral,
optional per-tool readers) and §5c (telling an active session from a silent one).

## What it adds

The contract reports a session's start, compactions, memory writes and end. Between those, an open
session is a blank: it reads as quiet even while it works. The reader follows the session's
transcript, found through `session.started`'s `transcript` field with format `claude-code-jsonl`,
and reports:

- **Turn:** working (the model is answering or a tool is running, with the tool's name and the
  file it works on) or waiting for input (the turn ended). Shown as the session's state; working
  sessions count as active, waiting ones as waiting.
- **Last activity:** the newest timestamped line. An open session silent past `quiet` is quiet.
- **Live tokens:** usage summed across replies, each reply counted once (Claude Code repeats a
  reply's usage on every line of it). Feeds the board's token column, the detail panel, today's
  total and the 24-hour graph, at the time each reply was written rather than at the session's end.
- **Context size:** input plus cache tokens of the latest reply: how full the context is.
- **Compactions** seen in the transcript.

It shows no prompt text, replies, code or command lines: only tool names and a file's base name.

## Rules

- Only sessions open now, or ended in the last 24 hours, are read (the graph needs the latter).
  Other tools, other formats, and sessions with no transcript show as before.
- Reading is incremental: an offset per file, a partial last line left for later, a shorter file
  read again from the start. Lines that cannot matter (attachments, request records) are skipped
  before decoding, so a 144 MB transcript is read once at start and then only its new lines.
- Lines after the app's clock are left unread until their time comes, so `-at` works on live
  transcripts as it does on events.
- Sidechain lines are ignored: subagents are their own sessions in the contract.
- Turn: a reply with a tool call means working on that tool; a reply that ends its turn, a
  stop-hook summary or an interrupted request means waiting for input; a prompt or tool result
  means working.
- Paths are taken as given; a leading `~/` is expanded, because the test data writes it
  (`contract-proposals.md` §3 asks for absolute paths).

## Packages

`internal/transcript` reads and holds activity per path. The app reads transcripts after scanning
events and passes the results to `model.Build`, which joins them to sessions by path. The model stays
free of file reading beyond what it is handed.

## Left out

Units' transcripts (a unit's session id is not known until it ends, `conops.md` §8 #5) and
subagents' transcripts. Telling a session waiting on a permission prompt from one running a long
tool: both read as working on that tool.

## Where it stands (2026-11-22, built)

Built as described. Checked against real transcripts: the largest (144 MB) reads in 1.3 s at
start, and its live total (693,549,134 tokens) matches the hooks' end total for that session, so
replies are counted once. Reads run in the background, one batch at a time, so a slow first read
never stalls the screen; `-once` reads before drawing. The test data's three open sessions now read
as waiting for input as of 11-21 14:02 rather than "open · last event". A scratch project pointed at
a live session showed "working · Bash" with its spinner, live tokens and context size.

Small additions: a `-select N` flag for reviewing a row's detail panel; the detail panel's
compaction count takes the higher of the hooks' and the transcript's.
