Session condition (the pilot's `session.condition`, plus fatigue worked out from transcripts) and machine load built; Windows builds again; CI tests on macOS and Windows; the screen now lays itself out responsively. Not yet committed.

## Morning: the pilot's fatigue, then its rethink

- The pilot added `session.fatigue` (worst of hours, compactions, context size, prompts since the
  memory file). Recommended showing the hook's reading and working out a reading from transcripts
  for other sessions. The developer stopped to rethink it with the pilot.
- The pilot replaced it with `session.condition`: fatigue from compaction history only (its replay
  of its own sessions found slips step up after the first compaction; context size showed no
  relation), drift, unsaved work, flags `just_compacted` and `compaction_possible`, and the
  measures.

## Midday: what to build (developer)

- Asked to make it easy to see what is going on, and to show CPU, RAM and GPU if sampling is cheap.
- Proposed: a CONDITION cell (fatigue word plus one flag), three detail lines, counts; fatigue only
  worked out for sessions without a record (the evidence-backed reading, and nearly exact from the
  transcript); unsaved, drift and just-compacted left out because they need Waypoint's memory file,
  the project's state checks or a `project.md` re-read. Machine CPU and memory from the OS; per
  session, the process tree. GPU only where `nvidia-smi` exists (not on this machine).
- **Developer decided:** levels and flags on the board (not what each asks the session to do),
  fatigue only, CPU and memory both on the board and in the detail panel. Must work on Mac, Linux
  and Windows. GPUs later (the developer notes AMD has a way, as game engines read it).

## Afternoon: built

- `internal/procs`: one pass of `/proc` on Linux (1.9 ms per sample here); one `sysctl` on macOS and
  one Toolhelp snapshot on Windows for parents, with gopsutil filling CPU and memory only for
  processes under a session. gopsutil v4.26.8 is the newest that builds on Go 1.25 (v4.26.9 wants
  Go 1.26); it pulled x/sys to v0.41.0. No cgo anywhere. Liveness moved there too, which unblocked
  the Windows build.
- Windows checked from WSL by running the Windows test binaries of every package (all pass) and a
  frame of the Windows build (Windows' own CPU and 31G memory; Claude Code for Windows found, no
  sessions there). A sample takes about 12 ms on Windows. macOS builds but is untested here; CI now
  runs the tests and one frame on macOS and Windows runners.
- Transcript reader counts work items and work summarized away as the pilot's hook does; Go's
  regexp has no lookbehind, so the redirect rule is a small scanner.
- Thresholds copied from the pilot into `internal/model/condition.go`.
- Pid reuse: Claude Code's list still carries a session from March whose pid is now a thread of an
  unrelated program. A sampled process that started after the event naming it is now ignored.
- First real frame: this session reads `≈ tired` (3 compactions, 131 work items summarized away);
  the pilot's open session shows its own record (`working · unsaved`).
- Board at 200 columns drops only the tool column; at 180 also started and model.

## Evening: flexing columns

- The developer suggested a flexbox-like layout instead of dropping columns. Each column now has a
  preferred width, a minimum and a grow share, and asks for no more than its widest cell; the state
  column grows and gives way last. Columns drop only when all minimums would not fit (about 155
  columns with full cells). At 150 only the tool column goes; at 120 tool, started and model. A
  test checks every width from 70 to 260 fills exactly with no column under its minimum.
- The developer tried it and meant the panels: they should resize and relayout like a responsive
  web page. Now the top panels flex and wrap to a second row on screens of 44 rows or more (else
  the least needed panel goes); from 230 columns the detail panel sits beside the board; detail
  lines wrap under their label. A test checks frames fill the screen exactly from 80 to 320
  columns and 24 to 60 rows (it caught the one-line header not being padded). Also: runners alive
  no longer counts a finished run's runner (a frame showed "1 of 0 alive").
- The developer: keep the top panels on narrow screens and stack them; add a key to hide the stats
  for people who do not want them. Panels now go in as few rows as fit, never dropped (one per row
  at 60 columns); `s` toggles them, `stats = false` in the settings starts hidden. They give way to
  the summary line only under 40 columns or when the board and detail would keep fewer than 12
  rows, and the line says so.

## Next

- The pilot runs a mock workload to try it out.
- Release once the developer has seen it.
