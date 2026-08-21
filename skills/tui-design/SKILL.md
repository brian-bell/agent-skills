---
name: tui-design
description: Drive a terminal UI toward a visual prototype by running it in tmux, capturing its rendered output with capture-pane, and iterating until it matches. Use whenever the user wants a TUI to look like a prototype, screenshot, or mockup, asks to polish or pixel-nudge TUI layout (borders, padding, centering, headers, tabs), or reports that a full-screen terminal app "doesn't look right", even if they don't mention tmux. Also use when a TUI needs a real TTY to render and you must see its actual output.
---

# TUI Iterate

Close the loop between a terminal UI and its target design without asking the
user for screenshots after every change. You run the app yourself in tmux,
capture what it actually renders, compare that against the prototype, and keep
adjusting until they match. The user only needs to look when you believe an
element is done.

## Establish the target

Identify the prototype before touching code. It is usually one of:

- an image or screenshot (read the file, or use one pasted in conversation)
- an HTML/text mockup in the repo (`prototype/`, `design/`, `docs/` are common)
- a written description of the desired layout

If you cannot find it, ask. Guessing the target defeats the loop.

Then decompose the target into discrete visual elements (outer border, header
band, nav tabs, pane dividers, footer, ...) and confirm the scope: which
element is being worked on now. Match one element at a time; broad "make it
all match" passes produce churn and regressions. Small alignment details are
exactly what the user cares about here, so record them per element: border
style and joins, padding above vs below text, vertical centering, active-state
indicators, where lines should meet.

## Run the app in tmux

Full-screen TUIs (Bubble Tea, ratatui, curses) need a real TTY and will not
render in a plain pipe. tmux provides the TTY and a stable, sizable viewport:

```bash
tmux kill-session -t tui-design-skill 2>/dev/null
tmux new-session -d -s tui-design-skill -x 200 -y 50 '<run command>'
sleep 1   # let the first frame render
```

Point the app at a temp or scratch data directory instead of the user's real
data, via config flag, env var, or whatever the project supports. The loop,
and any reviewer driving the app, can then press keys, open screens, and
trigger actions without risking real state.

- Pick `-x`/`-y` deliberately and keep them fixed across the whole loop so
  captures are comparable. Match the prototype's approximate aspect if known.
- Use the project's real run command (check its skill, Makefile, or README).
  Rebuild before relaunching when the app is compiled.
- Navigate to the screen under work with
  `tmux send-keys -t tui-design-skill <keys>`. Compare the screen the prototype
  shows, not just the launch screen.

## Capture and compare

```bash
tmux capture-pane -p -t tui-design-skill          # plain text: layout, spacing, joins
tmux capture-pane -p -e -t tui-design-skill       # with escape codes: colors, styles
```

Read the capture and compare it to the prototype element by element. The plain
capture is the source of truth for geometry: count columns and rows to check
padding bands such as pad-text-pad, confirm vertical centering by
comparing blank rows above and below text, and check that borders reach their
corners and junctions. The `-e` capture answers color and emphasis questions.

Common mismatches worth checking every pass, because they are easy to miss in
text form: borders that stop one cell short of a junction, asymmetric padding
that reads as "uncentered", missing left padding when a cursor or indicator is
absent, stray blank rows between stacked bands, and active-state styling that
only renders when a pane has focus. Catch that last one by tabbing between
panes and capturing both focused and unfocused states.

## The loop

1. Change the code for the current element only.
2. Rebuild, kill and relaunch the tmux session, re-navigate, re-capture.
3. Compare capture vs prototype. Fixed? Move to the next detail. Not fixed or
   regressed elsewhere? Adjust and repeat.
4. When the element matches, show the user the relevant capture excerpt next
   to your reading of the prototype and get sign-off before starting the next
   element.

Never claim a match from memory or from having edited the code. Every "it
matches now" must come from a fresh capture taken after the last rebuild. If a
capture looks identical after a change, suspect a stale binary.

Some layout rules can be written as rendering assertions: view tests that
render into a fixed-size buffer and check rows and columns. Once the element
is settled, add or update one so the match survives future refactors. Iterate
visually first; test what you converged on.

## Composing with review-loop

When the user asks for an independent quality gate, "use review-loop" or
"score it against the prototype", run the *review-loop* skill on top of this
loop. The element decomposition above becomes the reviewer's criteria list,
and the running tmux session becomes the work product under review.

You stay the worker: edit code for the current element, rebuild, relaunch the
session, re-navigate, and do a quick self-capture check before spending a
reviewer cycle. The reviewer is a subagent that never sees your captures or
your reasoning. Give it the tmux session name, the navigation keys, and the
prototype path, and have it take its own captures and score fidelity. The
scratch data directory means the reviewer may drive the app freely. It must
not edit code, rebuild, kill the session, or resize the pane.

Reviewer prompt sketch:

```
You are a TUI fidelity reviewer. The app runs in tmux session `tui-design-skill`
at 200x50 against scratch data. Do not resize the pane. Drive the app
freely with `tmux send-keys -t tui-design-skill <keys>`.
Read the prototype at <path>. Element under review: <element>.

1. Capture with `tmux capture-pane -p -t tui-design-skill` for geometry and
   `-p -e` for colors and styles. Capture both focused and unfocused
   states.
2. Compare element by element. Count columns and rows to verify padding
   and centering claims. Cite row and column numbers and capture excerpts
   as evidence, and say whether the plain or -e capture backs each finding.
3. Score 1-10. Rank findings: Critical means the layout is structurally
   wrong; Important covers off-by-one padding, missing junctions, and
   wrong active states; Minor is style nits.

Prior-loop feedback already addressed: <list>
```

Adjustments to review-loop's defaults for this domain:

- Gate per element, not per screen, matching the one-element-at-a-time rule
  above.
- The reviewer must capture after your relaunch. State in the handoff that
  the session was rebuilt and relaunched at the current code.
- Replace review-loop's final polish step with this skill's exit ritual: show
  the user the passing capture for sign-off, then add a rendering assertion
  test for the converged layout.
- If the score plateaus on the same geometry finding for two loops, suspect a
  framework limitation. The layout library may not support that join or
  spacing. Tell the user instead of looping harder.

## Cleanup

Kill the session when the loop ends or is abandoned:

```bash
tmux kill-session -t tui-design-skill 2>/dev/null
```

Leave no orphaned sessions running the user's app.
