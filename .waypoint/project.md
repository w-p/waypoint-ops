# Project state

<!-- This file is the current-state snapshot of the project. The AI maintains it.
     Update the phase, shipped list, and deferred list as work progresses.
     Keep ground truth docs current so any session can orient quickly.
     The Phase line is machine-read by the adapters' hooks: keep exactly one line
     starting **Phase:** naming Ideation, Planning, or Execution. -->

**Phase:** Execution — first slice done and in daily use (`plan/first-slice.md`); changes come from use; next: the Cursor check (planned for 2026-11-23, not started), then more of the pilot's real runs.

**Shipped:**

<!-- Bullet list of completed features or milestones. Be specific enough that a future
     reader understands what was built without having to read the code.
     Link to the relevant .waypoint/features/*.feat.md document where one exists. -->

- *(nothing shipped yet)*. First slice built and reworked after the first review: panels with run
  gauges and an hourly token graph, the board with unit bars and a full-row selection, focus,
  detail panel, bottom key bar, spring animation, column dropping, color switch, settings file
  (`plan/first-slice.md`). Committed 2026-11-22 (`93c6023`).
- Claude Code transcript reader, built and awaiting review: working or waiting for input with the
  tool in use, live tokens over time, context size (`design/transcript-reader.md`). Committed
  (`27081bc`).
- Claude Code's running-session list (`claude agents --json`), built and awaiting review: sessions
  waiting on a permission prompt highlighted in amber, busy or idle, gone when no longer running.
- Other work, built and awaiting review: running Claude Code sessions no event names show under the
  watched project they run in; those outside every watched project are dimmed below the rest, `o`
  or `other_work` to hide them; a unit session the runner's events do not name joins its running
  unit (`design/other-work.md`). Committed with the running-session list (`082bca2`).
- Subagents read from their own transcripts (running, handed back), so sessions without events
  show their subagents (`design/transcript-reader.md`).
- Releases and CI, set up but not yet pushed: CI on every push and pull request; a `vX.Y.Z` tag
  builds Linux and macOS archives and publishes a GitHub release from `CHANGELOG.md`
  (`design/releases.md`). The GitHub repository is public; history was scrubbed of private
  project names before the first push (memory 2026-11-22).
- Reads the adopted contract additions: repeated `session.started` (resume), `session.waiting`,
  `session.active`, `session.model`, the session pid (gone when dead), and units' transcripts live
  from `unit.started` (`design/contract-proposals.md`, "Adopted"). Released as v0.2.0.
- Session condition and machine load, built 2026-11-23 to try out: a CONDITION column (fatigue
  and one flag) from the pilot's `session.condition`, a fatigue reading worked out from the
  transcript (marked `≈`) for sessions without one, a machine panel (CPU, memory, sessions tiring or
  worse, the app's own use) and per-session and per-run CPU and memory. Windows builds again; CI
  tests on macOS and Windows (`design/session-condition.md`). The screen lays itself out for its
  size: top panels flex and stack, `s` hides them, the detail panel sits beside the board from 230
  columns, board columns shrink before any is dropped (`design/first-slice.md`). Released as v0.3.0;
  v0.3.1 shows a run's unit under way in its gauge, labels counts "done" and shows a running unit's
  model.
- In daily use from 2026-11-22 against the developer's 11 Waypoint projects (settings file outside
  the repo). From 2026-11-23 the pilot's runner writes run events and its sessions write
  `session.condition`; its mock run that day checked the runs panel, unit gauges and condition
  readings live. Other projects show live Claude Code sessions only.

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
- Releases and CI: `.waypoint/design/releases.md`
- Other work: `.waypoint/design/other-work.md`
- Pending contract proposals (approved, not yet in the contract): `.waypoint/design/contract-proposals.md`

**Plan**

<!-- Links to plan documents in .waypoint/plan/.
     Leave blank until Planning phase begins. -->

- First slice: `.waypoint/plan/first-slice.md` (design: `.waypoint/design/first-slice.md`, provisional)
