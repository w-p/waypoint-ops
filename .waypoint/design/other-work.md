# Design: other work

**Status:** Provisional, 2026-11-22. Built to review running, like the first slice. Asked for by the
developer: "non-waypoint work would be dimmed somehow… and maybe we could have an option to
filter", with more tools (Cursor) on the way.

## What it adds

Claude Code's running list (`design/transcript-reader.md`) reports every Claude Code session on
this machine, not just the ones in watched projects with Waypoint's hooks. A session in that list
that no event names is shown either way:

- **Inside a watched project's folder:** a session of that project, drawn like any other, with
  "no events yet" in its detail panel. Changed the same day after the developer started using the
  app on real projects: no project writes events yet, so dimming these would have dimmed
  everything.
- **Outside every watched project:** other work (a project without Waypoint, a scratch folder).
  The overview shows it so the picture covers everything the machine is doing, without mistaking
  it for Waypoint work.

## How it shows

- **Dimmed rows**, below the Waypoint work in the board, kind `other`. The project column is the
  folder's name.
- **State as for any session:** working on a tool, waiting for input, from Claude Code's status and
  the session's transcript (found by its id in this machine's Claude Code folder).
- **A blocked session stays amber.** A permission prompt is Claude Code's report, not a judgement
  of the app, and it is the one thing about other work worth seeing at a glance.
- **Tokens count.** Their spend goes into today's total and the graph, since it is the same
  subscription.
- **Filter:** `o` hides or shows other work everywhere (board, counts, tokens). Sessions in watched
  projects are never hidden. The settings file's
  `other_work = false` starts with it hidden. Shown by default.
- **Only while running.** With no events, nothing is left when the process exits, so other work
  leaves the board then. Not under `-at`, since the list describes only the present.

## Units are not other work

A running unit is a headless Claude Code session and appears in the list. It is matched to its
unit by the session id in `unit.started` or in the hook's own `session.started` (kind `unit`, parent
the run), and so is never shown as other work. A hook-written unit session whose id the runner's
events do not carry is joined to the run's running unit that has no id, rather than shown as a
session of its own (`contract-proposals.md` §4 asks the runner for the id at start).

## Other tools

Other work comes from a tool's own list of its running sessions. Claude Code is the only tool with
one so far; Cursor and others are added when their readers are built, under the same rules.
