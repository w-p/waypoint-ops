# Changelog

What changed between releases, newest first.

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
