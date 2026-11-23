# CONOPS: waypoint-ops

---

## 1. Background and Problem Statement

One developer runs agent work across several projects on one machine: interactive sessions in
Claude Code (and later other tools), the subagents those sessions spawn, and unattended run-mode
chains whose units run one after another, sometimes with machine jobs between them that last many
hours. The work shares one machine and one subscription's token limits.

Today the only view is a status page in the pilot project (`.claude/runner/chain_status.py`).
It covers one run in one project, and it gets its facts by pattern-matching files written for
people, so its output came out as long unstructured lines that were hard to read at a glance.
Nothing shows interactive sessions or subagents, nothing shows several projects at once, and the
shared limits are invisible. A failed unit or a runner that died is found when someone thinks to
look.

Waypoint now defines a machine-readable record of sessions and runs (the session-events contract,
below). That makes a proper reader possible.

---

## 2. Vision

waypoint-ops is a terminal app (Go and Bubble Tea) that shows, in one view, what every session and
run is doing across all the projects it is configured to watch: a common operational picture of
the machine's agent work. Healthy work takes one line. Conditions a project reports as problems
are highlighted and sorted to the top.

Guiding principles:

- **Read-only and local.** The app never writes into a project. Its own settings and history live
  in its own folders.
- **The contract is Waypoint's.** The app reads Waypoint's session-events contract and does not
  define the format. When it needs something the contract lacks, the change is proposed back to
  the contract, never added as a side format.
- **Sessions are the frame.** Interactive sessions, units spawned by a run-mode runner, and
  subagents are all sessions. A run is a group of units plus the machine jobs between them.
- **Tool-neutral.** Every event names the tool that wrote it, and the app shows it. Detail the
  contract does not carry comes from optional per-tool transcript readers; a session whose tool has
  no reader still shows.
- **Highlight, don't judge.** Acting on a state is the observed project's responsibility. The app
  brings reported conditions (a stopped run, a dead runner, a failed unit) to the person's
  attention; it does not decide what needs them.
- **All work first, one project on request.** The first screen is every project's work at once;
  the person can focus on a single project.
- **Structure over long lines.** A short summary header, a board with column headers, and a detail
  pane for the selected item, fitted to the terminal's width.

---

## 3. Users and Use Cases

The user is the developer who runs these projects, at a terminal on the same machine, often in a
window kept open beside their work. Representative questions:

1. Is anything wrong anywhere right now? (A glance at the header and the top rows.)
2. What is running on this machine across all projects, and how much of it is unattended?
3. A chain was left running overnight: did it finish, stop, or die, and if it stopped, which unit
   and why?
4. A run is waiting: on what (a scheduled event, a quiet window, a machine job), and for how long?
5. Which interactive sessions are open, which are active, and which have gone silent?
6. How many tokens has the machine used today, and which sessions used them?
7. What happened in one project over the last day? (Focus on it.)
8. Over the past weeks, how long did units take, how often did runs stop, what did they cost?
   (History, analysed outside the app.)

---

## 4. System Overview

Major parts, from the files up:

- **Settings.** A TOML file, `$XDG_CONFIG_HOME/waypoint-ops/config.toml` (default
  `~/.config/waypoint-ops/config.toml`): the project folders to watch (each with an optional
  display name), refresh interval (default 2 s), color (auto, always, never), history retention
  (default 90 days), and the thresholds used for highlighting (for example how long a session may
  be silent). Flags override the file: `--config`, `--refresh`, `--no-color`; `NO_COLOR` is
  honoured; project folders given on the command line replace the configured list.
- **Event reader.** For each project, reads `.waypoint/events/runs/*/events.jsonl` and
  `.waypoint/events/sessions/*.jsonl`, remembering how far it has read in each file.
- **Liveness checks.** Whether a run's `pid` file names a live process.
- **Model.** Projects, runs, units, jobs, sessions and subagents, built from the events and joined
  on session id. This is where states are derived and highlights decided.
- **Transcript readers (optional, per tool).** Claude Code's first: recent activity, context size,
  live token use, last write time. Found through an event's `transcript` field.
- **History writer.** Condensed CSV in `$XDG_DATA_HOME/waypoint-ops/` (default
  `~/.local/share/waypoint-ops/`).
- **Interface.** Bubble Tea and Lip Gloss: summary header, machine line, the board, the recent
  section, the detail pane, key bindings.

A web view reading the same files is a later, separate front end over the same model.

The repository layout follows Go convention and is settled in design; roughly `cmd/waypoint-ops/`
for the program and `internal/` packages for config, events, model, transcript readers, history
and the interface. `testdata/` holds fixtures built from real sessions and is never committed
(section 7).

---

## 5. Concept of Operations

### 5a. Starting

The person runs `waypoint-ops`. It loads the settings, checks each project folder exists (a
missing folder is shown as such, not fatal), reads every event file from the start, then redraws
on each refresh. `q` quits. Running `waypoint-ops /tmp/ops-live/*` points it at a replay folder
instead of the configured projects.

### 5b. Reading events

On each refresh the app reads only what was appended since the last read. It polls rather than
waiting for file-change notices, because notices are not reliable for files on the Windows drive
mounted in WSL. A partial last line (a write in progress) is left for the next refresh. Unknown
event types and fields are ignored. A file whose `v` is a newer major version than the app knows
is reported and not read further, as the contract requires.

### 5c. Deriving state

- **Run:** running (a unit is active), waiting (with the reason and detail from `run.waiting`),
  finished (with its outcome), or died (no `run.finished` and the pid is gone).
- **Unit:** pending, running, or ended with its status (`DONE`, `WAITING`, `BLOCKED`, `FAILED`,
  `NO_STATUS`) and one-line reason. A unit can appear both in a run's events and in a hook-written
  session file of kind `unit`; status comes from the runner's `unit.ended`, and memory writes and
  compactions from the session file.
- **Job:** running or ended, with exit code, run time and frozen time.
- **Interactive session:** active, silent (no activity past a threshold), ended, or open with no
  end seen. Telling "silent" from "gone" needs a transcript reader today; the contract question is
  in section 8.
- **Subagent:** counted under its parent as live or finished.

### 5d. Highlighting

A row is flagged when a project has reported one of these conditions:

- a run finished `STOPPED`, `HALT`, `CAP` or `TIMEOUT`;
- a run died;
- a unit ended `BLOCKED` or `FAILED`;
- later, a session waiting on its person (once the contract carries it).

Waiting, silent, and permission denials are shown in a softer color without flagging. `HELD` and
manual pauses are the person's own doing and are not flagged. Flags carry no acknowledgement
state; how finished work leaves the overview is decided by watching it live (section 8).

### 5e. The overview

The first screen. A mockup is in `.waypoint/design/mockups/overview.py` (run it to see the colors).

- **Title bar:** app name, scope (all projects, or the focused project), time, refresh interval.
- **Summary line:** counts of flagged, active and waiting items and the number of projects.
- **Machine line:** the shared limits: sessions open, subagents live, jobs running, tokens today
  (per tool that reports them), runners alive.
- **Board:** one row per run and per interactive session, across all projects. Columns: flag,
  project, kind, name, tool, model, state, units, subagents, job, started, elapsed, tokens. Rows
  sort flagged first, then active, then waiting, then quiet; within each, most recent activity
  first. Units and subagents are columns, not rows; the detail pane or an expand key opens them.
- **Recent section:** finished work, dimmed, collapsible.
- **Detail pane:** the selected row. For a run: outcome, a unit table (number, name, status,
  started, took, turns, tokens, reason), the job, the runbook path. For a session: tool, model,
  subagents, memory writes, compactions, and transcript activity where a reader exists.
- **Width:** the board fills the terminal. Below roughly 175 columns, low-priority columns drop
  out in a fixed order.
- **Color:** a fixed palette (truecolor, stepped down on lesser terminals): red for flagged and
  nothing else, amber for waiting or silent, green for active or done, blue for running units, cyan
  for project names, dim for secondary details and finished work; gauges and graphs use their own
  cool gradient (`design/first-slice.md`). Off with `--no-color` or `NO_COLOR`, where marks, glyph
  shapes and status words carry the meaning alone.

### 5f. Focus

`f` focuses the project of the selected row (or picks one from a list); `esc` returns to all
projects. Focus is the same board filtered to one project, with the project column dropped and the
recent section expanded into that project's history.

### 5g. History

When a session or run finishes, the app appends one row to its month's file: `sessions-YYYY-MM.csv`
or `runs-YYYY-MM.csv`. Rows are keyed by id, so rereading the same events never duplicates them.
Retention deletes whole month files older than the configured period.

- Sessions: project, tool, tool version, kind, id, parent, run, unit, model, started, ended,
  duration, how it ended (ended, gone, unknown), status, reason, turns, input, cache write, cache
  read and output tokens, compactions, memory writes, permission denials.
- Runs: project, run, runbook, started, finished, outcome, units done, units total, wall time,
  waiting time, job time, frozen time, token totals.

Parquet comes later if analysis needs it.

### 5h. Developing against test data

`testdata/` holds 734 events from real sessions in contract v1 (five projects, four runs covering
a long job with a freeze and a failed unit, a quick finished chain, a chain waiting to start, and a
chain whose runner died; 18 interactive sessions, 12 units, 84 subagents). `testdata/replay.py`
plays them into a scratch folder in scaled time with live pid files, for designing running,
waiting and frozen states. Tests committed to the repository use small synthetic fixtures written
for the purpose.

### 5i. Changing the contract

Gaps the app finds are written up as proposals to the session-events design in the pilot
repository (`.waypoint/design/session-events.md`, the pilot project) until it moves into
Waypoint's core (github.com/w-p/waypoint). The app adopts a change once the contract carries it.
Pending proposals are kept in `.waypoint/design/contract-proposals.md`.

---

## 6. Scope and Boundaries

**In scope (v1):**

- Terminal app reading session-events v1 from any number of configured project folders.
- Overview of all projects, focus on one, detail pane, keyboard navigation, self-refresh.
- Runs, units, jobs, interactive sessions and subagents; liveness from pid files.
- Highlighting of reported problem conditions.
- Claude Code transcript reader.
- History CSV with retention.
- Color with `--no-color` and `NO_COLOR`; layout fitted to terminal width.

**Out of scope (v1):**

- Writing anything into a project, including creating a run's `HOLD` (may be offered later).
- Defining or extending the event format inside this app.
- Readers for the pilot runner's older output layout.
- Transcript readers for tools other than Claude Code.
- Web view (later, from the same files and model).
- Parquet history (later, if needed).
- Phone or desktop notifications.
- Watching other machines.

---

## 7. Assumptions and Constraints

- The machine is Linux under WSL, with projects on the Windows drive mounted at `/mnt/c`; file
  change notices are unreliable there, so the app polls.
- Each event file has one writer, which appends whole lines; the app may see a partial last line.
- Process liveness is read from `/proc`.
- The contract is a draft and will change; the app follows it and ignores what it does not know.
- The pilot runner writes contract v1 after its 2026-11-22 chain finishes. Until then, live run
  data comes only from replay.
- Interactive sessions write events only once Waypoint's adapter hooks write them.
- Go standard library first; external packages are Bubble Tea, Lip Gloss and a TOML parser,
  added when needed.
- `testdata/` names real projects and contains a home path. It is listed in `.gitignore` and never
  committed.
- Nothing personal (names, home paths) is committed to this repository.

---

## 8. Open Questions

| # | Question | Owner | Needed By |
|---|----------|-------|-----------|
| 1 | Will the contract take the proposals in `.waypoint/design/contract-proposals.md` (waiting and activity events, a session pid, liveness by pid with jobs included)? Approved by the developer; awaiting the contract. | Waypoint contract | Answered 2026-11-22: adopted (`design/contract-proposals.md`) |
| 2 | How does finished work leave the overview: a time window, or each project's last run until a newer one starts? | Developer | Judged live, during Execution |
| 3 | In what order do columns drop on narrow terminals? (Proposed: tool, model, started first.) | Developer | Design |
| 4 | A job frozen by a quiet window is only known afterwards (`frozen_s`). Should the runner write an event when a freeze starts and ends? | Waypoint contract | Later |
| 5 | Can a unit's session id be known at start, so its transcript can be followed live? | Waypoint contract | Answered 2026-11-22: yes, `--session-id`; adopted (`design/contract-proposals.md` §4) |
| 6 | Do Cursor and other tools expose hooks, and what do their headless modes report? | Waypoint contract | When their writers are built |

Decided 2026-11-21: no setting for the subscription's token limit in v1; the machine line shows
tokens used today, and a limit is added once real numbers show what it should be. Transcript paths
are absolute as the contract says; the fixture generator is corrected.
