# waypoint-ops

One terminal view of every agent session and run-mode run across the projects on this machine.
Problems a project reports (a stopped run, a runner that died, a failed unit) are highlighted and
sorted to the top; healthy work takes one line. It only reads: it never writes into a project.

It reads Waypoint's session-events contract (each project's `.waypoint/events/`), version 1.

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

[[projects]]
path = "/path/to/project"
name = "project"    # optional; defaults to the folder name
```

Keys (also on the bar at the bottom): arrows (or `j`/`k`) move, `f` focuses the selected row's
project, `esc` shows all projects again, `r` shows or hides finished work, `q` quits.

Flags: `-refresh`, `-recent`, `-color`, `-no-color` (or set `NO_COLOR`), `-config`, `-focus`.
For looking at recorded data: `-at 2026-11-21T14:02:00-05:00` views it as of that time, `-once`
prints a single frame (`-width`, `-height`) instead of running the screen, and `-select 3` starts
with that board row selected.

Claude Code sessions whose events name a transcript are followed live: the board shows whether
each is working (and on which tool) or waiting for input, with live token counts and context size.
Only tool names and file base names are shown, never prompts, replies or commands.

## Status

Working version, being reshaped by review. Not yet: history files, and transcripts of run units
and subagents. Below about 175 columns the board drops its least needed columns.
