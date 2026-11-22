# Plan: first slice

Builds `design/first-slice.md`. One person (the assistant), in order; each step ends with its check.

1. **Repository basics.** `/testdata/` in `.gitignore` (anchored, so per-package fixture folders
   stay tracked); `go mod init github.com/w-p/waypoint-ops`; a README stub saying what the app is
   and how to run it against the test data. Check: `git status` does not list `testdata/`.
2. **Events.** Line types for every v1 event, an incremental reader that keeps offsets and leaves a
   partial last line, the version check. Check: unit tests; reads all 734 test events with no error.
3. **Model.** Build projects, runs, units, jobs, sessions and subagents; derive states and flags;
   pid liveness; sorting. Check: table-driven tests for each run outcome, a died run, a waiting
   run, a job, a unit joined from both sources, an open session.
4. **Interface.** Overview, focus, detail pane, keys, refresh, color switch. Check: run against
   `testdata/projects/*` and against `replay.py` output; the screen matches the mockup's structure.
5. **Settings.** TOML file at the XDG path, defaults, flags override. Check: runs with no file,
   with a file, and with folder arguments.
6. **Review.** The developer runs it; findings go to the memory file and drive the next slice.

Step 1 needs the phase moved to Execution, which the developer approves.

**Progress (2026-11-21):** steps 1 to 5 done. Tests pass (`go test ./...`); checked by rendering
frames against `testdata/projects/*` at 14:02 on 11-21 (overall and focused, color on and off) and
against a replay of the test chain (waiting on its job, runner alive). Step 6, the developer's
review, is next. Nothing is committed.

**Progress (2026-11-22):** first review done. The developer asked for a more graphic, htop-like
screen; reworked as panels with gauges, an hourly token graph, unit bars, a bottom key bar and
spring animation (`design/first-slice.md`, "after the first review"). Narrow terminals now drop
columns. Tests pass. Waiting on the second review. Nothing is committed.
