#!/usr/bin/env python3
"""Static mockup of the waypoint-ops overview, for design discussion only.

Prints what the first screen could look like, drawn from the testdata scenarios as they
stand at 2026-11-21 14:02. Not the app: no files are read, every value is typed in.

    python3 .waypoint/design/mockups/overview.py              # overall view
    python3 .waypoint/design/mockups/overview.py --focus pilot
    python3 .waypoint/design/mockups/overview.py --no-color --width 150

Colors use the terminal's own 16-color palette, so they follow its theme. NO_COLOR is honoured.
"""

import argparse
import os
import shutil

# Style codes. Kept to the 16 standard colors plus bold and dim.
RESET = "\033[0m"
STYLE = {
    "title": "\033[1;7m",
    "head": "\033[1;4m",
    "dim": "\033[2m",
    "bold": "\033[1m",
    "flag": "\033[1;31m",
    "fail": "\033[31m",
    "wait": "\033[33m",
    "run": "\033[32m",
    "done": "\033[32m",
    "project": "\033[36m",
    "select": "\033[1;97;44m",
}

color = True


def paint(text, style):
    """Wrap text in a style when color is on."""
    if not color or not style:
        return text
    return STYLE[style] + text + RESET


def fit(text, width, right=False):
    """Pad or cut text to exactly width characters."""
    if len(text) > width:
        text = text[: width - 1] + "…"
    if right:
        return text.rjust(width)
    return text.ljust(width)


# Columns: key, header, width, right-aligned.
COLUMNS = [
    ("mark", "", 1, False),
    ("project", "PROJECT", 13, False),
    ("kind", "KIND", 7, False),
    ("name", "NAME", 15, False),
    ("tool", "TOOL", 11, False),
    ("model", "MODEL", 9, False),
    ("state", "STATE", 32, False),
    ("units", "UNITS", 18, False),
    ("subagents", "SUBAGENTS", 11, False),
    ("job", "JOB", 18, False),
    ("started", "STARTED", 11, False),
    ("elapsed", "ELAPSED", 7, True),
    ("tokens", "TOKENS", 6, True),
]

# Each row: values per column, plus the style of its state and mark.
LIVE = [
    {
        "mark": "!", "project": "pilot", "kind": "run", "name": "chain-test-died",
        "tool": "claude-code", "model": "fable-5.2",
        "state": "runner died during unit 2", "style": "flag",
        "units": [("1", "done"), (" done · ", None), ("1", "fail"), (" cut off", None)],
        "subagents": "—", "job": "—",
        "started": "11-20 11:11", "elapsed": "1h 08m", "tokens": "530k",
    },
    {
        "mark": "!", "project": "pilot", "kind": "run", "name": "chain-test",
        "tool": "claude-code", "model": "fable-5.2",
        "state": "STOPPED: unit 4 FAILED", "style": "flag",
        "units": [("3", "done"), (" done · ", None), ("1", "fail"), (" failed", None)],
        "subagents": "—", "job": "exit 1 · 15h53m",
        "started": "11-20 11:11", "elapsed": "17h 12m", "tokens": "5.2M",
        "selected": True,
    },
    {
        "mark": " ", "project": "pilot", "kind": "run", "name": "merge-prereq",
        "tool": "claude-code", "model": "fable-5.2",
        "state": "waiting: Sunday's scheduled roll", "style": "wait",
        "units": [("0 of 3 started", None)],
        "subagents": "—", "job": "—",
        "started": "11-21 11:27", "elapsed": "2h 35m", "tokens": "—",
    },
    {
        "mark": " ", "project": "orchard", "kind": "session", "name": "b6d20f81",
        "tool": "claude-code", "model": "opus-5.5",
        "state": "active", "style": "run",
        "units": [("—", None)],
        "subagents": "2 live / 6", "job": "—",
        "started": "11-21 13:21", "elapsed": "41m", "tokens": "8.4M",
    },
    {
        "mark": " ", "project": "pilot", "kind": "session", "name": "27e95c4d",
        "tool": "claude-code", "model": "opus-5.5",
        "state": "open, no end seen", "style": None,
        "units": [("12", "done"), (" done (pilot)", None)],
        "subagents": "0 live / 6", "job": "—",
        "started": "11-21 08:02", "elapsed": "6h 00m", "tokens": "31.7M",
    },
    {
        "mark": " ", "project": "relay", "kind": "session", "name": "3c8e4a17",
        "tool": "claude-code", "model": "opus-5.5",
        "state": "silent 25m", "style": "wait",
        "units": [("—", None)],
        "subagents": "0 live / 3", "job": "—",
        "started": "11-21 12:52", "elapsed": "1h 10m", "tokens": "4.9M",
    },
    {
        "mark": " ", "project": "waypoint-ops", "kind": "session", "name": "51fa2c86",
        "tool": "claude-code", "model": "opus-5.5",
        "state": "active", "style": "run",
        "units": [("—", None)],
        "subagents": "—", "job": "—",
        "started": "11-21 13:44", "elapsed": "18m", "tokens": "1.1M",
    },
]

RECENT = [
    {
        "mark": " ", "project": "pilot", "kind": "run", "name": "runner-smoke",
        "tool": "claude-code", "model": "fable-5.2",
        "state": "DONE", "style": "done",
        "units": [("2", "done"), (" done", None)],
        "subagents": "—", "job": "—",
        "started": "11-20 09:46", "elapsed": "1m 52s", "tokens": "410k",
    },
    {
        "mark": " ", "project": "orchard", "kind": "session", "name": "e09b7f35",
        "tool": "claude-code", "model": "opus-5.5",
        "state": "ended", "style": None,
        "units": [("—", None)],
        "subagents": "0 live / 14", "job": "—",
        "started": "11-21 09:15", "elapsed": "3h 02m", "tokens": "22.0M",
    },
]

DETAIL_UNITS = [
    ("1", "Roll check", "DONE", "11-20 11:11", "3m 32s", "16", "530k", ""),
    ("2", "Merge GA prerequisites", "DONE", "11-20 11:14", "1h 04m", "37", "3.2M", ""),
    ("3", "Rehearse the merged tree", "WAITING", "11-20 12:19", "2m 53s", "20", "968k",
     "pid 31836 rehearsal"),
    ("4", "Judge the rehearsal, push", "FAILED", "11-21 04:22", "1m 02s", "16", "590k",
     "rehearsal FAIL: charleston-harbor roll, ENC chart server timed out (8 attempts)"),
]

STATUS_STYLE = {"DONE": "done", "WAITING": "wait", "FAILED": "fail", "BLOCKED": "fail"}


def columns(focus):
    """The columns shown; the project column is dropped when one project is in focus."""
    if focus:
        return [c for c in COLUMNS if c[0] != "project"]
    return COLUMNS


def header_line(cols):
    cells = []
    for key, title, width, right in cols:
        cells.append(paint(fit(title, width, right), "head" if title else None))
    return " ".join(cells)


def units_cell(parts, width):
    """Units are drawn from styled parts, so counts can carry their own color."""
    plain = "".join(text for text, _ in parts)
    if len(plain) > width:
        return fit(plain, width)
    out = "".join(paint(text, style) for text, style in parts)
    return out + " " * (width - len(plain))


def row_line(row, cols, dim=False):
    cells = []
    for key, _, width, right in cols:
        if key == "units":
            cells.append(units_cell(row["units"], width))
            continue
        text = fit(row[key], width, right)
        if key == "mark" and row["mark"] == "!":
            style = "flag"
        elif key == "state":
            style = row["style"]
        elif key == "project":
            style = "project"
        elif key in ("tool", "model", "started"):
            style = "dim"
        else:
            style = None
        if dim and key not in ("state", "mark"):
            style = "dim"
        cells.append(paint(text, style))
    line = " ".join(cells)
    if row.get("selected"):
        marker = paint("▸", "bold")
        return marker + line
    return " " + line


def rule(width, label=""):
    if not label:
        return paint("─" * width, "dim")
    text = f"── {label} "
    return paint(text + "─" * max(0, width - len(text)), "dim")


def title_bar(width, focus):
    scope = f"focus: {focus}   [esc] all projects" if focus else "all projects   [f] focus"
    left = f" waypoint-ops   {scope}"
    right = "2026-11-21 14:02   refresh 2s "
    gap = max(1, width - len(left) - len(right))
    return paint(left + " " * gap + right, "title")


def counts_line(focus):
    flagged = paint("2 flagged", "flag")
    running = paint("2 active", "run")
    waiting = paint("2 waiting", "wait")
    projects = "1 project" if focus else "5 projects"
    return f" {flagged} · {running} · {waiting} · {projects}"


def limits_line():
    label = paint("machine", "bold")
    return (f" {label}   4 sessions open · 2 subagents live · 0 jobs running · "
            f"tokens today 78.6M (claude-code) · runners alive 0 of 1 unfinished")


def detail(width):
    lines = [rule(width, "chain-test · pilot · .waypoint/plan/runs/chain-test.md")]
    outcome = paint("STOPPED", "flag")
    lines.append(f" outcome  {outcome}   unit 4: FAILED   finished 11-21 04:23 (9h 39m ago)"
                 f"   cap 4 · may push: 4")
    lines.append("")
    head = ["#", "UNIT", "STATUS", "STARTED", "TOOK", "TURNS", "TOKENS", "REASON"]
    widths = [2, 26, 8, 11, 7, 5, 7, max(10, width - 82)]
    rights = [False, False, False, False, True, True, True, False]
    cells = [paint(fit(h, w, r), "head") for h, w, r in zip(head, widths, rights)]
    lines.append(" " + " ".join(cells))
    for unit in DETAIL_UNITS:
        cells = []
        for i, (value, w, r) in enumerate(zip(unit, widths, rights)):
            text = fit(value, w, r)
            if i == 2:
                text = paint(text, STATUS_STYLE.get(value))
            elif i == 3:
                text = paint(text, "dim")
            cells.append(text)
        lines.append(" " + " ".join(cells))
    lines.append("")
    lines.append(" job      unit 3 · pid 31836 · exit 1 · ran 15h 53m · frozen 10h 00m by the quiet window")
    lines.append(paint(" keys     ↑↓ move   enter expand units   f focus project   r recent   "
                       "c colors   q quit", "dim"))
    return lines


def screen(width, focus):
    cols = columns(focus)
    live = [r for r in LIVE if not focus or r["project"] == focus]
    recent = [r for r in RECENT if not focus or r["project"] == focus]
    out = [title_bar(width, focus), counts_line(focus), limits_line(), ""]
    out.append(" " + header_line(cols))
    for row in live:
        out.append(row_line(row, cols))
    label = "recent, last 24h" + ("" if focus else f": {len(recent)} finished   [r] hide")
    out.append(rule(width, label))
    for row in recent:
        out.append(row_line(row, cols, dim=True))
    out.append("")
    out.extend(detail(width))
    return out


def main():
    global color
    parser = argparse.ArgumentParser(description="waypoint-ops overview mockup")
    parser.add_argument("--focus", help="show one project, e.g. pilot")
    parser.add_argument("--no-color", action="store_true")
    parser.add_argument("--width", type=int, help="screen width (default: terminal width)")
    args = parser.parse_args()
    if args.no_color or os.environ.get("NO_COLOR"):
        color = False
    width = args.width or shutil.get_terminal_size((180, 40)).columns
    for line in screen(width, args.focus):
        print(line)


if __name__ == "__main__":
    main()
