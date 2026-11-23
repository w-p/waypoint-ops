# Changelog

What changed between releases, newest first.

## Unreleased

- **Session condition.** A CONDITION column shows how tired each session is (fresh, working,
  tiring, tired, exhausted) and flags one that just compacted, has unsaved work while a compaction
  could come, or has a stale picture of its project. It comes from Waypoint's `session.condition`
  where a project writes it; for any other Claude Code session, the app works out its fatigue from
  the transcript's compactions and marks it `≈`. The detail panel shows the measures behind each
  reading.
- **Machine load.** A machine panel shows CPU and memory in use, how many sessions are tiring or
  worse, and what the app itself uses. Each session and run shows the CPU and memory of its
  process and everything it started (builds, tests, units).
- **Windows.** Windows builds are back, and every release is tested on Linux, macOS and Windows.
- **The screen lays itself out for its size,** as a responsive web page does. The panels across
  the top flex to fill the width (gauges and the token graph widen with them) and stack, two by two
  and then one above another, as the screen narrows, instead of disappearing. The `s` key (or
  `stats = false` in the settings) hides them, leaving one summary line. On a wide screen the detail panel moves beside the board, and long detail lines
  wrap instead of being cut off. The board's columns shrink to fit (text cut with `…`) instead of
  disappearing, and a column holding only short cells gives its room to the state column.
- The machine count of runners alive no longer counts a finished run's runner.
- A process id that was reused by another program is no longer taken for a session's.

## 0.2.0 (2026-11-22)

- **Session events from Waypoint's hooks.** A session waiting on a permission prompt is
  highlighted in amber and one waiting for input shows as waiting, for any tool whose hooks write
  `session.waiting`, not only Claude Code. A session whose recorded process is gone, with no end
  written, moves to finished work. A resumed session stays one session with its original start.
- **Run units, live.** The run's row shows what its running unit is doing ("unit 2 · Edit
  view.go"), and the unit table shows its live tokens and tool.
- **Models** come from the hooks' `session.model` events as well as the transcript.
- To see these, restart sessions in a project after updating its Waypoint hooks; sessions started
  earlier write no events.

## 0.1.0 (2026-11-22)

The first release: one terminal view of every agent session and run across your projects.

- **Overview.** Panels for status, run progress and token use over the last 24 hours, above a
  board of all live work: runs with their units and jobs, interactive sessions with their
  subagents. Problems sort to the top in red. Finished work stays in view for 48 hours.
- **Focus and detail.** `f` narrows the board to one project; the panel below shows the selected
  run's units or the selected session's activity, context size and tokens.
- **Claude Code, live.** Sessions are followed through their transcripts: working (and on which
  tool and file) or waiting for input, with live token counts and subagents. The app also asks
  Claude Code which sessions are running: one waiting on a permission prompt is highlighted in
  amber, and one that has closed is shown as gone.
- **Other work.** Claude Code sessions outside your watched projects are shown dimmed below the
  rest; `o` hides them.
- **Read-only.** The app never writes into a project. It reads Waypoint's session-events files
  and Claude Code's own records.

Settings live in `~/.config/waypoint-ops/config.toml` (see the README). Builds are for Linux and
macOS; on Windows, run it under WSL.
