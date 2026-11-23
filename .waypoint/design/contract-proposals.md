# Proposed changes to the session-events contract

**Status:** Adopted. All four proposals are in the contract as of 2026-11-22, and the pilot's
runner and hooks write them; the app reads them (see "Adopted" at the end). The contract is
Waypoint's, drafted in the pilot repository (`.waypoint/design/session-events.md`); this file
keeps the reasoning.

**Timing:** apply to the pilot document after its 2026-11-22 chain finishes. A modified tracked
file in the pilot stops a waiting chain. Sent to the interactive pilot session on 2026-11-22, with
§4 and the runner findings, to apply when the chain is done.

Each item below names the contract's open item it answers, the proposed text, and why the reader
needs it.

## 1. Telling waiting, quiet and gone apart (contract Open, first item)

Today hooks write `session.ended` on a normal end only; a closed window or a crash writes nothing,
and nothing says a session is waiting on its person.

Proposed additions to "Session events (writer: hook)":

| Event | Fields |
|---|---|
| `session.waiting` | `session`, `reason`: `input` (the agent finished its turn and waits for the person) or `permission` (it waits on a permission prompt) |
| `session.active` | `session`. Written by hooks on activity (a prompt, a tool call), at most once a minute per session |

Proposed addition to `session.started`: optional `pid`, the tool's own process id, when the hook
can know it.

Proposed addition to "Liveness and control": an interactive session is gone when its `pid` is no
longer running and no `session.ended` was written. Without a `pid`, a reader may only say how long
the session has been silent.

Why: the overview highlights sessions waiting on their person, shows active and silent sessions,
and stops showing closed ones as open. For Claude Code, the Stop hook gives `input`, the
Notification hook gives `permission`, and the prompt and tool hooks give `session.active`.

Claude Code's writers, checked against its hook documentation on 2026-11-22:

- `permission`: the `PermissionRequest` hook fires just before the dialog shows (and when a call
  would be denied because no prompt is possible). A hook that returns no decision only observes,
  so it can write the event without changing the prompt. `Notification` with type
  `permission_prompt` also fires, but only after about 6 seconds, and so is the fallback.
- `input`: the `Stop` hook fires when the agent finishes responding. `Notification` with type
  `idle_prompt` fires about 60 seconds later if nobody has typed.
- Nothing fires when the person answers a prompt. The next `PreToolUse`, `PostToolUse` or
  `UserPromptSubmit` shows the session moved on, so readers should treat any later event from the
  session (including `session.active`) as ending the wait.

Where this matters most, after Claude Code's own list (2026-11-22). For an interactive Claude
Code session on this machine, the app already gets these answers by asking Claude Code directly
(`claude agents --json`: busy, idle, or waiting with what it waits for, and a pid;
`design/transcript-reader.md`), and that list knows about permission prompts sooner and more surely
than any hook. For those sessions `session.waiting` and `session.active` are a fallback. They are
still wanted for:

- other tools (Cursor, Codex), which have no such list that the app can ask;
- the record: the list says only what is happening now, so past waits (how long a run sat on a
  prompt overnight) exist only as events;
- readers that do not ask Claude Code at all.

The `pid` field goes slightly beyond the heartbeat the developer approved. A heartbeat alone
cannot tell a session waiting overnight from a closed window, because neither writes anything. A
pid can. To check when the hook is written: whether the hook's parent process is the tool itself
or a shell between them.

## 2. Liveness by process id only, jobs included (contract "Liveness and control")

Proposed replacement for the first rule: "A run is live while `pid` names a running process. A
run with no `run.finished` and a dead pid died; readers say so."

Proposed addition: "A job is running while the `pid` in its `job.started` names a running process.
A job with no `job.ended` and a dead pid ended without a record; readers say so."

Why: the current rule asks that the process's command line be the runner, which a reader cannot
check portably and which fails against the test replay (it holds its own pid). Job pids are
already in the events; using them is the only way to tell a running job from a dead one while the
run waits. Pid reuse is left to readers: comparing the process's start time with the event's `ts`
guards against it without a contract change.

## 3. Transcript paths are absolute (contract "Shared optional objects")

No change to the contract text, which already says absolute. The fixtures generated on 2026-11-21
write `~/...`; the generator is fixed to write absolute paths. Readers expand a leading `~` anyway.

## 4. A unit's session id at start (contract Open, last item)

The contract leaves it open whether a unit's session id can be known when the unit starts. It can,
for Claude Code: `claude --session-id <uuid>` (checked in 2.1.323) runs a session under an id the
caller chooses. A headless `claude -p` session also appears in Claude Code's running list from its
first seconds, with its session id, pid and folder (checked 2026-11-22; it reports
`kind: "interactive"`, so the list alone cannot tell it from a person's session).

Proposed change to `unit.started`: "`session` carries the id when the tool lets the runner choose
it (Claude Code: `--session-id`). Optional `transcript`, when the runner knows where the tool will
write it." And remove the open item.

Why: with the id at start, the reader joins the unit to its live transcript and to Claude Code's
running list while the unit works, rather than learning who it was when it ends. It also keeps a
running unit from looking like unrelated work: the app now shows Claude Code sessions that have no
events as other work (`design/other-work.md`), and a unit it cannot place would show twice. Today
the hook's own `session.started` (kind `unit`, parent the run) gives the id at start when hooks are
installed; the runner's event makes it independent of hooks.

A second case, reported by the pilot session on 2026-11-22: the pilot's own
`.claude/runner/chain_status.sh` picks a unit's transcript by folder and recency, and during
merge-prereq it reported an interactive session's commands (a merge and push) as unit 2's. A
runner-chosen session id fixes both readers. waypoint-ops' own fallback (a running unit with no id
claims the earliest listed session in its project's folder started with it) has the same weakness,
narrowed by the start time; it stays only until the runner writes the id.

## Not proposed now

Freeze start and end events for machine jobs, and other tools' hooks: revisit when the code that
would write them is built (`conops.md` §8).

## Adopted (2026-11-22)

The pilot took all four into the contract and writes them: the runner launches units with
`--session-id` and puts the session and an absolute transcript path in `unit.started`; hooks write
`session.started` (with `pid` and `source`: startup, resume or clear), `session.waiting`,
`session.active` (at most once a minute, and at once after a wait), compactions, memory writes and
`session.ended` with tokens. One correction from the pilot: event files are git-ignored and never
travel with the repository, so the "sessions on another machine" case in §1 does not exist and was
removed.

How the app reads them:

- **A repeated `session.started`** (resume or clear) continues the same session: the start time
  stays, and a session that had ended is open again; its next `session.ended` reports the whole.
- **`session.waiting`** holds until any later event from the session. `permission` is flagged for
  attention in amber, like a permission prompt from Claude Code's list; `input` is waiting. Claude
  Code's own list wins when it has the session, because it is read every few seconds.
- **`session.active`** counts as activity, so a session without a transcript reader shows active or
  silent by it.
- **The session `pid`** decides gone when Claude Code's list cannot (another tool, or the list not
  read): a dead pid with no `session.ended` is gone. Pid reuse is not guarded against yet.
- **A unit's transcript** is followed from `unit.started`, so the run row shows what the running
  unit does ("unit 2 · Edit view.go"), the unit table shows its live tokens and tool, and its token
  use lands on the graph when it happens. Its subagents come from its transcript folder, as for
  sessions.
- **`session.model`** (added by the pilot the same evening) sets the session's model. The hook
  reads it from the transcript after the fact, so it can land after a `session.waiting` or just
  before `session.ended`; it neither ends a wait nor counts as activity.
- **`job.ended`** carries `exit_code` when the pilot's job wrapper recorded it; without it, the code
  is unknown.
