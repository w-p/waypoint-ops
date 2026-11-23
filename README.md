# waypoint-ops

One terminal view of every agent session and run-mode run across the projects on this machine.
Problems a project reports (a stopped run, a runner that died, a failed unit) are highlighted and
sorted to the top; healthy work takes one line. It only reads: it never writes into a project.

It reads Waypoint's session-events contract (each project's `.waypoint/events/`), version 1.

## Install

Download the archive for your system from the GitHub releases page (Linux, macOS and Windows,
Intel and ARM), check it against `checksums.txt`, unpack it, and put `waypoint-ops` on your PATH.
`waypoint-ops -version` says which release you have. Under WSL, the Linux build sees WSL's
sessions and processes; the Windows build sees Windows' own.

## Build and run

    go build -o waypoint-ops ./cmd/waypoint-ops
    ./waypoint-ops /path/to/project /path/to/another

Without folders it watches the projects listed in `~/.config/waypoint-ops/config.toml`
(or under `$XDG_CONFIG_HOME`):

```toml
refresh = "2s"      # how often to reread the projects
color = "auto"      # auto, always or never
recent = "48h"      # how long finished work stays on screen
quiet = "15m"       # an open session with no event for this long is shown as quiet
other_work = true   # show Claude Code sessions that have no Waypoint events, dimmed
stats = true        # show the panels across the top (counts, machine, runs, tokens)

[[projects]]
path = "/path/to/project"
name = "project"    # optional; defaults to the folder name
```

Keys (also on the bar at the bottom): arrows (or `j`/`k`) move, `f` focuses the selected row's
project, `esc` shows all projects again, `r` shows or hides finished work, `o` shows or hides other work, `s` shows or hides the stats
panels across the top, `q` quits.

Flags: `-refresh`, `-recent`, `-color`, `-no-color` (or set `NO_COLOR`), `-config`, `-focus`.
For looking at recorded data: `-at 2026-11-21T14:02:00-05:00` views it as of that time, `-once`
prints a single frame (`-width`, `-height`) instead of running the screen, and `-select 3` starts
with that board row selected.

Claude Code sessions and run units whose events name a transcript are followed live: the board shows whether
each is working (and on which tool) or waiting for input, with live token counts and context size.
Only tool names and file base names are shown, never prompts, replies or commands.
Every 5 seconds the app also asks Claude Code which sessions are running (`claude agents --json`):
a session waiting on a permission prompt or a dialog is highlighted in amber, and one that is no
longer running is shown as gone. Running Claude Code sessions that no event names show under the
watched project they run in; those outside every watched project (projects without Waypoint,
scratch folders) are other work: dimmed, below the rest, under their folder's name. This is skipped when viewing the past
with `-at`.

Each session's condition is shown in the CONDITION column: its fatigue (fresh, working, tiring,
tired, exhausted) and at most one flag (`compacted`, `unsaved`, `stale`), from the latest
`session.condition` its project writes; the detail panel adds drift and unsaved work with the
measures behind each. A session whose project writes no such record gets a fatigue reading worked
out from its transcript's compactions, marked `≈`.

The machine panel shows CPU and memory in use, how many open sessions are tiring or worse, and
what the app itself uses. The `CPU MEM` column shows each session's process and everything under
it (a run's: its runner and units). Sampling reads what the operating system already keeps and
takes a few milliseconds.

## Releasing

Add a section for the new version to `CHANGELOG.md`, commit, then tag and push the tag:

    git tag v0.2.0 && git push origin v0.2.0

The release workflow tests, builds the six archives (zip for Windows) with their checksums, and
publishes a GitHub release whose notes are that changelog section. Every push and pull request to
`main` runs formatting, vet, the tests with the race detector, and the builds, then the tests and
one frame on macOS and Windows.

## Status

Working version, being reshaped by review. Not yet: history files.
Subagents are read from their transcripts beside the session's. The screen lays itself out for its
size: the top panels flex, then stack two by two and one above another as the screen narrows; from
230 columns the detail panel sits beside the board; the board's columns shrink, the state column
last, and the least needed are dropped only when even that does not fit. The top panels become one
summary line when hidden with `s`, or when stacking them would leave the board fewer than 12 rows.
GPUs are not shown yet.
