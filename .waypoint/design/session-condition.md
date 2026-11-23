# Design: session condition and machine load

**Status:** Provisional, 2026-11-23. Built to try out. Asked for by the developer after the pilot
replaced its first fatigue reading with `session.condition`; choices agreed the same day: the board
shows levels and flags (not what each asks the session to do), the app works out fatigue only for
sessions without a record, and per-session CPU and memory go both on the board and in the detail
panel. Must work on Linux, macOS and Windows. GPUs are left for later.

## Session condition

The contract's `session.condition` (in the pilot's `design/session-events.md`, "Session condition")
carries three readings, each answering a different question, two flags and the measures behind
them:

- **Fatigue** (should the session hand over?): fresh, working, tiring, tired, exhausted. Set by
  compactions and the work they summarized away.
- **Drift** (is its picture of the project old?): current, aging, stale. Hours since its last state
  check, and commits it did not make.
- **Unsaved** (what would a stop lose?): none, some, lots. Work since the memory file was written.
- **Flags:** `just_compacted` (has not re-read `project.md` since), `compaction_possible` (context
  past 200k).

The app shows the latest record of each session and does not reinterpret it: thresholds and what
each value asks of the session belong to the project that wrote it.

### How it shows

- **Board, CONDITION column:** the fatigue word, colored by level (fresh and working dim, tiring and
  tired amber, exhausted red), then at most one flag, the first that applies: `compacted` (just
  compacted), `unsaved` (unsaved work while a compaction is possible), `stale` (drift stale). Rows
  do not move: these are warnings, not faults. A run's row shows its running unit's condition.
- **Detail panel:** one line per reading with its measures, then where the record came from and how
  old it is.
- **Machine panel:** how many open sessions are tiring, tired and exhausted.

### Sessions without a record

A session whose project writes no `session.condition` (no Waypoint, or hooks not yet updated) gets a
fatigue reading worked out from its transcript, marked `≈` on the board and "worked out from the
transcript" in the detail panel. The contract allows this: "a reader may derive its own from the
transcript and should say it did."

Fatigue is the one reading worth working out. The pilot's replay found the slips follow compactions,
and the transcript records every compaction and the work before it, so it comes out nearly the same
as the hook's. The other readings are left out because they would be guesses:

- **Unsaved** counts work since the Waypoint memory file was written; a project without Waypoint has
  no memory file.
- **Drift** needs the project's own state checks and a git query per session.
- **Just compacted** turns on a re-read of `project.md`. The detail panel says when the last
  compaction was instead.

`compaction_possible` is shown too, since it is exactly the context size the transcript reader
already reads.

The measures, as the pilot's hook counts them:

- **compactions:** compaction markers in the transcript.
- **work summarized away:** work items before each compaction, summed. A work item is an Edit,
  Write, MultiEdit or NotebookEdit outside `.waypoint/memory/`, a subagent started (Agent, Task), or
  a shell command that commits or writes files (an in-place edit, a redirect to a file, `tee`, a
  heredoc fed to an interpreter).
- **hours:** since the session's process started (a resume starts a new process, as the hook's clock
  restarts), or since its start when no process is known.

The thresholds are the pilot's on 2026-11-23, kept in `internal/model/condition.go`: exhausted at 8
compactions, tired at 4 or 100 work items summarized away, tiring at 1, working at 2 hours, fresh
before that. The pilot recalibrates its own from slips; these follow when the developer asks.

## Machine load

Read on every refresh, in the background, only if cheap. It is: on this machine one sample walks
about 120 processes in a few milliseconds, and the footer shows the app's own CPU so the cost stays
visible.

- **Machine panel:** CPU across all cores and memory in use, as gauges.
- **Per session and run:** CPU (percent of one core, as htop shows it) and memory, summed over the
  process and everything under it: the shells, builds and test runs it started. A session's process
  comes from Claude Code's running list or the pid in `session.started`; a run's is its runner, so
  it includes its units. Board column `CPU MEM`, fixed width, dropped early on a narrow screen; the
  detail panel adds the process count and when the process started.
- **First sample** shows memory only; CPU needs two samples.
- **Under WSL** the numbers are the Linux side's. A Windows program (Cursor on Windows, say) is not
  in them; the Windows build reads Windows' own.

### Per platform, without cgo

| | Process list with parents | Per-process CPU, memory, start | Machine |
|---|---|---|---|
| Linux | `/proc/*/stat`, one pass, all fields at once | same pass | `/proc/stat`, `/proc/meminfo` |
| macOS | one `sysctl kern.proc.all` | `proc_pidinfo` through gopsutil, only for processes under a session | gopsutil |
| Windows | one Toolhelp snapshot | `GetProcessTimes`, `GetProcessMemoryInfo` through gopsutil, likewise | gopsutil |

gopsutil v4.26.8 (the newest that builds on Go 1.25) loads macOS's libraries at run time, so builds
stay static. Process liveness, which the app needs for runners, jobs and sessions, moves from
`kill(pid, 0)` to a per-platform check, which is what made a Windows build impossible before.

## Windows build

The release adds Windows archives (zip) for amd64 and arm64. Claude Code's running list, its folder
(`%USERPROFILE%\.claude`) and the transcripts are found the same way. Run under WSL, the Linux build
sees WSL's sessions; run on Windows, the Windows build sees Windows' sessions. Each sees its own
side.

## Not yet

- GPUs. AMD and NVIDIA both have vendor libraries a game engine would use; reading them without cgo
  on three platforms is its own piece of work.
- The unit table does not show each unit's condition; the run's row shows the running unit's.
