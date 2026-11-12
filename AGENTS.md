# Agent instructions for Rummy

This file is the setup and maintenance runbook for coding agents. README.md
is the player-facing installation and gameplay guide. Read both before work.

## Scope and project layout

- Work in the project root containing `go.mod`, `cmd/rummy`, and `internal`.
  The original machine uses `/home/duke/code/rummy`; do not assume that path
  or the username on another installation.
- Keep code in this root, without creating another `rummy-tui` subfolder.
- Go module: `github.com/DukeAvi/rummy-tui`. CLI: `./cmd/rummy`.
- `internal/game`: terminal-independent rules and game state.
- `internal/lan`: authoritative two-seat session and TCP protocol.
- `internal/tui`: Bubble Tea menus, local/remote controls and Cardboard views.
- `scripts/setup-hotspot.ps1`: Windows administrator setup for WSL2 NAT.
- Use Cardboard as a dependency. Never copy, vendor or modify its source.
- `to_do.md` is private planning material and must remain ignored by Git.
- Preserve existing Git history, identity and unrelated changes. Do not create
  remotes, push, or rewrite commits as part of setup. The original delivery
  required exactly two commits; do not add commits without a new instruction.

## Setup workflow

If asked to install/configure the game, carry out the steps that the available
permissions and tools allow. Inspect existing installations and settings before
changing them. Do not stop at printing instructions when execution is possible.
Do not claim a Windows setting or network connection is configured just because
a script exists or a local test passed.

### 1. Inspect the environment

Run read-only checks from the project root:

```sh
pwd
git status --short --branch
uname -srm
command -v go
go version
command -v git
command -v curl
```

Determine whether this is native Linux or WSL and whether this machine is the
host, guest, or both for a local check. Identify the shell before editing its
startup file. Inspect any applicable ancestor AGENTS.md files too.

### 2. Provide the required tools

- Reuse Go 1.24+ if installed. Do not replace a suitable existing installation.
- For Ubuntu/Debian, install missing packages with the user's authorized sudo
  access: `git curl ca-certificates tar build-essential`.
- The README has the fresh Go installation commands. Use official Go downloads
  and match `uname -m` to Linux amd64 or arm64. Prefer the current stable
  release; verify current download metadata at https://go.dev/dl/.
- Do not extract a toolchain over an existing Go tree. Inspect it first; follow
  https://go.dev/doc/install for an upgrade if one is actually needed.
- Add the Go bin directory to the active shell's startup file only if missing,
  preserving its other contents. Also export PATH for the current process.
- Confirm the resolved `go version` is at least 1.24 before building.
- If a sandbox prevents writes to the default Go cache, use a writable
  task-specific GOCACHE, e.g. `/tmp/rummy-go-cache`, for that session. Do not
  change global Go settings to work around temporary sandbox restrictions.

### 3. Resolve Cardboard and install Rummy

Inspect `go.mod` and its `replace` directive. This checkout expects Cardboard
at `../cardboard`. If the directory exists, inspect its module path and reuse
it. Do not overwrite it or clone another copy over it.

If needed, clone https://github.com/DukeAvi/cardboard into that sibling path,
or update the replacement to an existing user-supplied checkout. If the user
prefers downloaded modules and network access is available, remove the local
replacement and run `go mod tidy` to use the pinned version. Do not invent a
Rummy download URL: this project may exist only as a local source copy.

Build from the project root:

```sh
go mod download
go build -o rummy ./cmd/rummy
```

The receiving player can use a copy of the executable built for the same OS
and CPU architecture, without installing Go or Cardboard. Give it execute
permission and launch from an actual interactive Linux/WSL terminal. Do not
silently overwrite a friend's existing executable or source directory.

### 4. Configure hotspot hosting when requested

The host listens on **TCP 45873**. The advertised address must be reachable
from the guest; a WSL private address or localhost may not be suitable.

1. Identify the actual Windows WSL distribution and hotspot IPv4 address.
   Use `wsl --list --quiet`, `Get-NetIPAddress -AddressFamily IPv4`, and
   `Get-NetAdapter -IncludeHidden` from Windows PowerShell where available.
   Turn on Windows Mobile hotspot if the authorized environment supports it;
   otherwise state the exact user action required.
2. Inspect whether WSL uses NAT or mirrored networking. Inspect existing port
   proxies and firewall rules before changing them. Do not assume
   `192.168.137.1` is the hotspot address; it is only a common example.
3. For **WSL2 NAT**, run `scripts/setup-hotspot.ps1` in Administrator Windows
   PowerShell, supplying `-Distro` and `-HotspotIP` explicitly for unattended
   execution. Derive the script's Windows/UNC path from the real project path,
   for example using `wslpath -w scripts/setup-hotspot.ps1` in WSL.
4. The script forwards the hotspot address's port 45873 to that distro's WSL
   IPv4 address and creates/updates the named `RummyHotspot45873` firewall
   rule, scoped to the hotspot address and local subnet. Rerun if the WSL IP
   changes. Do not disable the firewall or open unrelated ports.
5. For **mirrored networking**, do not apply the NAT script blindly. Follow
   https://learn.microsoft.com/en-us/windows/wsl/networking and allow TCP
   45873 through the applicable Hyper-V/Windows firewall rules.
6. Inspect Linux firewall rules if one is enabled; permit the game's port as
   needed for the intended interface/subnet. Preserve unrelated rules.
7. If Windows interop, elevation or socket creation is blocked, report the
   command and actual error. Continue independent setup work. Provide the
   single concrete Administrator PowerShell command the user needs; do not
   repeatedly request permissions that the environment cannot grant.

### 5. Verify installation and first play

For implementation or setup changes, run the appropriate checks:

```sh
go test ./...
go test -race ./...
go vet ./...
go build -o rummy ./cmd/rummy
git diff --check
```

Race checks need a supported Go race toolchain and C compiler. Explain a
missing prerequisite instead of presenting a skipped check as passed. Do not
run or change Cardboard's own test suite merely to verify this application.

Use a real terminal or PTY to check:

- First menu offers **One PC** and **OTA / local network**.
- Local play selects mode and joker rank and hides hands at each handoff.
- A window below 88 columns or 36 rows displays a zoom/resize prompt. Restoring
  the size restores Cardboard artwork. Never use a text-only hand fallback.
- Network Host selects the reachable address, mode and wild rank, and displays
  the full `address:port/token` code. Join accepts that whole code.
- Both players must ready; only the active seat can act. No client receives
  the opponent's hand/groups or the closed stock order.
- Both modes permit draws, reordering, groups, discards and valid declarations.
- After a win, the host proposes the new mode/rank and both ready again.
- Transient failures pause and retry; a guest's private rejoin code restores
  their seat within five minutes. Voluntary exit ends the session.
- Confirmed quit restores the terminal; cancelling quit resumes the game.

Keep the host lobby listening while checking from the guest device:

```powershell
Test-NetConnection <actual-hotspot-IPv4> -Port 45873
```

A successful TCP probe alone is not a gameplay test. Join from the second
device, take turns in both directions, verify privacy, and exercise replay
and exit before claiming the complete hotspot workflow is verified. Do not
mark private TODO hardware checks complete based on in-memory protocol tests.

## Rules and UI constraints

Preserve the chosen house rules: seven cards need exactly two groups of 3+4;
thirteen cards need complete coverage; both need at least one pure sequence.
Neither mode requires a second sequence. Rank-wild cards can be natural faces
in a pure sequence; printed jokers cannot. Track physical cards by Cardboard
ID. Validate before committing a declaration or other destructive action.

Use Cardboard Vintage art for all playing cards, with overlap/grid navigation.
Below the minimum terminal size, prompt to zoom out or enlarge the window.
Keep private hands hidden during local handoffs and on all unauthorized views.

## Documentation and handoff

Keep README.md focused on installing, connecting and playing. Put agent-only
procedures and maintenance details here. When finishing setup, report:

- Installed/reused Go version, project path and build/launch command.
- Host address, port and which forwarding/firewall changes were actually made.
- Checks passed, failed, or not run, separating local tests from guest-device
  connectivity and complete two-device gameplay.
- Any remaining action the user must take and why the agent could not do it.

The previous development environment blocked TCP sockets and Windows interop
(`Operation not permitted` / `UtilBindVsockAnyPort`), so physical hotspot
connectivity and the PowerShell script were not verified there. Recheck the
current environment rather than assuming that limitation still applies.
