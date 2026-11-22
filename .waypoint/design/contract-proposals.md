# Proposed changes to the session-events contract

**Status:** Approved by the developer 2026-11-21 for proposal. Not yet in the contract. The
contract is Waypoint's, drafted in the pilot repository (`.waypoint/design/session-events.md`,
the pilot project); this file holds waypoint-ops' requests until that document takes them.

**Timing:** apply to the pilot document after its 2026-11-22 chain finishes. A modified tracked
file in the pilot stops a waiting chain.

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

## Not proposed now

Freeze start and end events for machine jobs, a unit's session id at start, and other tools'
hooks: revisit when the code that would write them is built (`conops.md` §8).
