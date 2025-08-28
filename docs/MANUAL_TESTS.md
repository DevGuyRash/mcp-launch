# Manual Test Matrix — TUI Preflight & UX

Scenarios to validate acceptance criteria (R1–R6, NF):

1) Three configs: one OK, one ERR, one with >300-char descriptions
- Expect all three servers listed with badges; ERR shows details (d/enter)
- Verify tool counts and long-desc warnings in Results view; -v shows details

2) Cancel from list
- From list view press q or Esc → program exits with “Cancelled — no servers launched”
- No processes (mcpo/cloudflared) started

3) Edit/trim/clear description
- From menu → 3) Descriptions → select tool
- + trims to ≤300; - clears override; e opens editor; enter saves, esc cancels
- Diff view shows -/+ markers

4) Launch picker recall
- Choose raw, confirm; re-run with TUI → Launch picker preselects last choice

5) Quick tunnel failure path
- Start with named/quick tunnel misconfigured → stack still starts without public URL; Results view indicates local URLs

6) Navigation consistency
- Up/Down or j/k move; Enter selects/confirm; Space toggles; Esc/b go back; ? toggles help overlay

7) No-color fallback
- Run with NO_COLOR=1 or TERM=dumb → no color styles; content remains readable

