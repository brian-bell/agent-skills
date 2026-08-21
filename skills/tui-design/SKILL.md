---
name: tui-design
description: Drive a terminal UI toward a visual prototype by running it in tmux, capturing its rendered output with capture-pane, and iterating until it matches. Use whenever the user wants a TUI to look like a prototype, screenshot, or mockup, asks to polish or pixel-nudge TUI layout (borders, padding, centering, headers, tabs), or reports that a full-screen terminal app "doesn't look right" — even if they don't mention tmux. Also use when a TUI needs a real TTY to render and you must see its actual output.
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

If you cannot find it, ask — guessing the target defeats the loop.

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
tmux kill-session -t tui-iter 2>/dev/null
tmux new-session -d -s tui-iter -x 200 -y 50 '<run command>'
sleep 1   # let the first frame render
```

- Pick `-x`/`-y` deliberately and keep them fixed across the whole loop so
  captures are comparable. Match the prototype's approximate aspect if known.
- Use the project's real run command (check its skill, Makefile, or README).
  Rebuild before relaunching when the app is compiled.
- Navigate to the screen under work with
  `tmux send-keys -t tui-iter <keys>` — compare the screen the prototype
  shows, not just the launch screen.

## Capture and compare

```bash
tmux capture-pane -p -t tui-iter          # plain text: layout, spacing, joins
tmux capture-pane -p -e -t tui-iter       # with escape codes: colors, styles
```

Read the capture and compare it to the prototype element by element. The plain
capture is the source of truth for geometry: count columns and rows to check
padding bands (for example pad-text-pad), confirm vertical centering by
comparing blank rows above and below text, and check that borders reach their
corners and junctions. The `-e` capture answers color and emphasis questions.

Common mismatches worth checking every pass, because they are easy to miss in
text form: borders that stop one cell short of a junction, asymmetric padding
that reads as "uncentered", missing left padding when a cursor or indicator is
absent, stray blank rows between stacked bands, and active-state styling that
only renders when a pane has focus (capture both focused and unfocused states
by tabbing between panes).

## The loop

1. Change the code for the current element only.
2. Rebuild, kill and relaunch the tmux session, re-navigate, re-capture.
3. Compare capture vs prototype. Fixed? Move to the next detail. Not fixed or
   regressed elsewhere? Adjust and repeat.
4. When the element matches, show the user the relevant capture excerpt next
   to your reading of the prototype and get sign-off before starting the next
   element.

Never claim a match from memory or from having edited the code — every "it
matches now" must come from a fresh capture taken after the last rebuild. If a
capture looks identical after a change, suspect a stale binary.

Where a layout rule is expressible as a rendering assertion (a view test that
renders into a fixed-size buffer and checks rows/columns), add or update one
once the element is settled, so the match survives future refactors. Iterate
visually first; test what you converged on.

## Cleanup

Kill the session when the loop ends or is abandoned:

```bash
tmux kill-session -t tui-iter 2>/dev/null
```

Leave no orphaned sessions running the user's app.
