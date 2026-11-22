# Design: first slice (a running overview to review)

**Status:** Provisional, approved 2026-11-21 as a throwaway. This is a visual project: the developer
reviews the running app and the design changes to match, without a separate approval round for each
change. The code follows the latest review, not this document; this document is updated after each
review to say where things stand. Intent and scope stay with `conops.md`.

## What it does

`waypoint-ops [folder ...]` reads session-events v1 from each folder (a project root, or a folder
holding `.waypoint/events/` directly, as in `testdata/projects/<name>/`) and shows the overview
from `conops.md` §5e: title bar, summary line, machine line, board, recent section, detail pane.
It refreshes on an interval, moves with the arrow keys, focuses a project with `f`, returns with
`esc`, quits with `q`. Colors follow `conops.md` §5e and turn off with `--no-color` or `NO_COLOR`.

Without folder arguments it reads the settings file (`conops.md` §4). Flags: `--refresh`,
`--no-color`, `--config`.

## Packages

| Package | Knows |
|---|---|
| `cmd/waypoint-ops` | flags, settings, starting the program |
| `internal/config` | the settings file and its defaults |
| `internal/events` | the contract: line types, reading files incrementally, version check |
| `internal/model` | projects, runs, units, jobs, sessions, subagents; deriving state and flags from events; liveness |
| `internal/ui` | the Bubble Tea program: layout, columns, colors, keys |

The model takes events and a clock and returns a snapshot; the interface only draws snapshots.
This keeps state derivation testable without a terminal and lets a web view reuse it later.

## State rules

As in `conops.md` §5c and §5d, with these specifics for the slice:

- A run with no `run.finished` is live if its `pid` file names a running process (checked in
  `/proc`), otherwise died. No command-line check (`contract-proposals.md` §2).
- An interactive session with no `session.ended` is shown as "open, no end seen" with the time
  since its last event. Waiting, active and gone come with the contract proposals.
- Units and subagents are joined to their parent by session id; a unit's status comes from the
  runner's `unit.ended`.
- Recent: finished within 24 hours of now. Flagged rows sort first whether live or recent.

## Left out of the slice

History CSV, the Claude Code transcript reader, and the settings file's thresholds. Each comes
after review.

## Where it stands (2026-11-21, before the first review)

Built as described, with these choices made while building:

- Finished work stays 48 hours (`recent`), so the test data's died chain is still on screen a day
  later. An open session with no event for 15 minutes (`quiet`) is shown as quiet, in yellow.
  Hook events are sparse, so most open sessions read as quiet until activity events exist
  (`contract-proposals.md` §1).
- The units column counts every status, pending included ("1 done · 1 cut off · 2 pending").
  Finished items show a plain subagent count; live ones show "live / total".
- A run that died shows its elapsed time with a "+": the death was not recorded, so it ran at least
  that long.
- The selected row is drawn in reverse video without its cell colors.
- Two flags for reviewing recorded data: `-at` (view as of a time; the clock runs on from there)
  and `-once` (print one frame).
- With replay, a job's pid is the one recorded in the fixture, which is not running, so the job
  shows "pid gone, no end". The replay only keeps the runner's pid alive.

## Where it stands (2026-11-22, after the first review)

The developer asked for: no key hints at the top of any view (`esc` least of all); the selection as a
full-row bright bar; a blank row above the recent divider, as above the detail panel; and a freer,
more graphic look in the manner of htop and bpytop, using Lip Gloss and Harmonica. Now:

- **Panels.** Rounded boxes with the title set into the top edge and a note at the right. Across the
  top: a status panel (verdict counts beside machine counts), a runs panel (one progress gauge per
  run, filled in eighths of a cell, colored by state), and a tokens panel (the last 24 hours as one
  bar per hour, in a teal-to-violet gradient so red stays reserved for flags). Below: the work panel
  (the board) and the detail panel. Under 30 rows the top panels shrink to one summary line; under
  170 columns the hourly bars go one cell wide, under 130 the tokens panel goes.
- **Board.** A filled header bar. Rows span the full panel width. The selected row is a bright cyan
  bar of plain dark text; flagged rows carry a faint red tint (only on 256-color terminals and up).
  The mark column shows a pulsing `!` for flagged work, a spinner for work in progress, `‖` for
  waiting, `·` for quiet sessions and `✓` for finished work.
- **Units and subagents as pictures.** The units column is a bar with one glyph per unit in runbook
  order: `■` done, `◆` running, `◇` waiting, `✕` failed, blocked or cut off, `□` pending, then
  done of total. Each status has its own shape, so the bar reads without color. Subagents are dots:
  filled for live, hollow for ended, up to five, then live of total.
- **Narrow terminals.** Columns leave in this order until the board fits: tool, started, model, job,
  tokens, subagents, elapsed, kind. The state column takes any spare width. This settles the drop
  order provisionally (`conops.md` §8 #3).
- **Keys.** Only on a bar along the bottom, htop style (key, then its label on a colored block);
  `esc All` appears there only while focused. The clock and refresh interval sit at its right.
- **Animation (Harmonica).** Springs fill the run gauges and raise the token bars when they first
  appear and move them when values change; the spinner and the flag's pulse run at 10 frames a
  second, rising to 30 while a spring is moving. `-once` draws everything at rest.
- **Palette.** Truecolor hex colors (a dark slate theme) that Lip Gloss steps down on lesser
  terminals; `--no-color` keeps every shape and word and uses reverse video for the selection
  and the key labels.
- **Tokens over time** come from the model's new `Machine.Spend` (totals reported in the last 24
  hours with their times). Tokens are only reported when a unit or session ends, so a long session
  lands as one tall bar at its end.

## Tests

Table-driven tests for state derivation in `internal/model`, using small synthetic events written
in the test; `internal/ui` tests the springs and the column layout. The root `testdata/` is for running the app by hand and with `replay.py`.
