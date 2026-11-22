# Project state

<!-- This file is the current-state snapshot of the project. The AI maintains it.
     Update the phase, shipped list, and deferred list as work progresses.
     Keep ground truth docs current so any session can orient quickly.
     The Phase line is machine-read by the adapters' hooks: keep exactly one line
     starting **Phase:** naming Ideation, Planning, or Execution. -->

**Phase:** Execution — building the first slice (`plan/first-slice.md`) to review running; its design is provisional and changes with each review (approved 2026-11-21).

**Shipped:**

<!-- Bullet list of completed features or milestones. Be specific enough that a future
     reader understands what was built without having to read the code.
     Link to the relevant .waypoint/features/*.feat.md document where one exists. -->

- *(nothing shipped yet)*. First slice built and reworked after the first review: panels with run
  gauges and an hourly token graph, the board with unit bars and a full-row selection, focus,
  detail panel, bottom key bar, spring animation, column dropping, color switch, settings file
  (`plan/first-slice.md`). Committed 2026-11-22 (`1b16d3e`).
- Claude Code transcript reader, built and awaiting review: working or waiting for input with the
  tool in use, live tokens over time, context size (`design/transcript-reader.md`). Committed
  (`4f34aa3`).
- Claude Code's running-session list (`claude agents --json`), built and awaiting review: sessions
  waiting on a permission prompt highlighted in amber, busy or idle, gone when no longer running.
- Other work, built and awaiting review: running Claude Code sessions no event names show under the
  watched project they run in; those outside every watched project are dimmed below the rest, `o`
  or `other_work` to hide them; a unit session the runner's events do not name joins its running
  unit (`design/other-work.md`). Committed with the running-session list (`7516ada`).
- In daily use from 2026-11-22 against the developer's 11 Waypoint projects (settings file outside
  the repo). No project writes events yet, so the board shows live Claude Code sessions only; runs
  appear once the pilot's runner writes events (`contract-proposals.md` §4).

**Approach**

<!-- Bullet list of key technical or architectural decisions that govern how the project
     is being built. These are the "why things are the way they are" notes. -->

- Go terminal app on Bubble Tea and Lip Gloss; read-only and local.
- Reads Waypoint's session-events contract v1; gaps are proposed back to the contract, never
  solved with side formats (`conops.md` §5i).
- Sessions are the frame (interactive, unit, subagent); a run is units plus machine jobs.
- Tool-neutral; per-tool transcript readers are optional, Claude Code's first.
- Many projects from the data model up; the first screen is all work across projects, with focus
  on one project as a filter of the same board.
- The app highlights conditions projects report; it does not judge what needs the person and keeps
  no acknowledgement state.
- Board rows are runs and interactive sessions; units, subagents and jobs are columns. The look
  follows htop and bpytop: framed panels, gauges, graphs, a bottom key bar, light animation
  (Harmonica). Current state: `design/first-slice.md`.
- Polls on the refresh interval (file-change notices are unreliable on the WSL-mounted Windows
  drive).
- Settings in TOML under the XDG config folder; history as monthly CSV files under the XDG data
  folder, pruned by whole month.

**Deferred**

<!-- Bullet list of work that was explicitly decided not to do yet.
     Include a brief reason or pointer to the relevant discussion. -->

- Creating a run's `HOLD` from the app: possibly later; v1 is strictly read-only.
- Web view: later, from the same files and model.
- Parquet history: only if analysis needs it.
- Transcript readers beyond Claude Code.
- How finished work leaves the overview: to be judged live during Execution (`conops.md` §8 #7).
- A setting for the subscription's token limit: not in v1; added once real usage shows the number.
- Remaining open questions in `conops.md` §8: answered as design proceeds or when their writers
  are built.

**Ground truth docs**

<!-- Links to the key reference documents for this project.
     At minimum: conops, opord. Add design docs and feature docs as they are produced. -->

- Intent: `.waypoint/conops.md`
- Standing orders: `.waypoint/opord.md`
- Event contract (Waypoint's, drafted in the pilot repo): `.waypoint/design/session-events.md` at
  the pilot project
- Overview mockup: `.waypoint/design/mockups/overview.py`
- Transcript reader: `.waypoint/design/transcript-reader.md`
- Other work: `.waypoint/design/other-work.md`
- Pending contract proposals (approved, not yet in the contract): `.waypoint/design/contract-proposals.md`

**Plan**

<!-- Links to plan documents in .waypoint/plan/.
     Leave blank until Planning phase begins. -->

- First slice: `.waypoint/plan/first-slice.md` (design: `.waypoint/design/first-slice.md`, provisional)
