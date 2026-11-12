# rummy-tui

Two humans can play seven- or thirteen-card Rummy in one terminal, or from
separate terminals over a laptop hotspot / local network. Both modes use
[Cardboard](https://github.com/DukeAvi/cardboard) for full-size Vintage card
artwork and the same tested rules engine.

## Set up Go in WSL (Ubuntu/Debian)

If `go version` already reports **1.24 or newer**, skip to Install and run.
Run these commands inside WSL, not Windows PowerShell:

```sh
sudo apt update
sudo apt install -y git curl ca-certificates tar build-essential
```

For a fresh installation, download the current stable Linux toolchain into
`~/.local/lib/go`. This uses the [official Go distribution](https://go.dev/doc/install):

```sh
RUMMY_GO_VERSION=$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n 1)
case "$(uname -m)" in
  x86_64) RUMMY_GO_ARCH=amd64 ;;
  aarch64|arm64) RUMMY_GO_ARCH=arm64 ;;
  *) echo 'Unsupported CPU: use the Linux download from https://go.dev/dl/' ;;
esac
RUMMY_GO_TEMP=$(mktemp -d)
curl -fL "https://go.dev/dl/${RUMMY_GO_VERSION}.linux-${RUMMY_GO_ARCH}.tar.gz" -o "$RUMMY_GO_TEMP/go.tar.gz"
mkdir -p "$HOME/.local/lib"
if test -e "$HOME/.local/lib/go"; then
  echo 'A Go installation already exists here. Do not extract over it; see the official upgrade instructions.'
else
  tar -C "$HOME/.local/lib" -xzf "$RUMMY_GO_TEMP/go.tar.gz"
fi
export PATH="$HOME/.local/lib/go/bin:$PATH"
go version
```

To keep Go available in new terminals, add this line to `~/.bashrc` (Bash)
or `~/.zshrc` (Zsh), then reopen the terminal:

```sh
export PATH="$HOME/.local/lib/go/bin:$PATH"
```

## Install and run

Use Go **1.24+**, Git, and a truecolor Unicode terminal in Linux or WSL.
For the supplied checkout, Cardboard lives at `../cardboard`; `go.mod` uses
that local source directly. If installing from a source copy on a new computer,
copy this project's source folder to `~/code/rummy` and get Cardboard beside it:

```sh
mkdir -p ~/code
# Skip this clone if ~/code/cardboard already exists.
git clone https://github.com/DukeAvi/cardboard.git ~/code/cardboard
```

Rummy has not been published to a remote repository; obtain its source folder
or compiled executable from the host. Build and launch the supplied source
checkout from WSL (on another account, use `cd ~/code/rummy`):

```sh
cd /home/duke/code/rummy
go mod download
go build -o rummy ./cmd/rummy
./rummy
```

Or launch from source with `go run ./cmd/rummy`. For a different Cardboard
location, run `go mod edit -replace=github.com/DukeAvi/cardboard=/absolute/path/to/cardboard`
and `go mod tidy`. With internet access, `go mod edit -dropreplace=github.com/DukeAvi/cardboard`
followed by `go mod tidy` downloads the pinned revision instead.

Your friend can build the same source, or receive a copy of the compiled
`rummy` executable for the same Linux CPU architecture. The executable includes
its dependencies: Cardboard and Go are not required on the receiving machine.
Run `chmod +x rummy` and `./rummy` in their Linux/WSL terminal. Build separately
for a different operating system or CPU architecture.

Use at least **88 columns × 36 rows**; **180×60** is comfortable. Check with
`stty size` (rows, then columns). When the window is too small, the game asks
you to zoom out or enlarge it. It never replaces cards with a text-only hand.
Resize at any point to resume the artwork. Each card uses 26×20 terminal cells.

## First round

1. Launch the program and choose **One PC** or **OTA / local network**.
2. One PC: choose 7/13 cards and the wild rank, then pass the keyboard at each
   handoff screen. Only Enter reveals the next player's hand.
3. Network host: choose **Host a game**, enter the laptop address reachable by
   your friend, then choose the mode and wild rank. The server listens on TCP
   **45873**. The lobby shows **FULL JOIN CODE**, plus separate **Host address** and
   **Join token** fields. Copy the whole code, like
   `192.168.137.1:45873/<token>`. It appears after selecting the mode and wild
   rank and successfully starting the host. A small window requests resizing
   rather than hiding part of the code.
4. Friend: connect to the laptop hotspot, launch the same game, choose network
   play → **Join a game**, and paste the **entire code**, including address,
   port, slash, and token. Press Enter. No separate address entry is needed.
5. Both players review the lobby and press Enter to ready. Player 1 (host)
   starts. The guest sees a waiting screen until their turn. All moves are
   checked by the host; each client receives only its own hand and groups.
6. Draw with `d` (closed) or `o` (open), arrange/group cards, then `x` discards
   the selected card or `v` declares using it as the final discard.

The initial code admits one guest, expires after ten minutes, and is consumed
when used. Esc cancels the host lobby; hosting again generates a new code.
Codes are credentials for a trusted local network; traffic is not encrypted.

## Hotspot and WSL connectivity

Turn on Windows Mobile hotspot and have your friend connect. Use `ipconfig`
in Windows to find the hotspot adapter's IPv4 address. Enter that address in
the Host screen; the automatically suggested WSL address may not be reachable
from the guest.

For automatic WSL2 NAT setup, turn on the hotspot, then open **Administrator
Windows PowerShell** and run the supplied script (replace `Ubuntu` if your
WSL distribution has a different name; `wsl --list --quiet` lists them):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "\\wsl.localhost\Ubuntu\home\duke\code\rummy\scripts\setup-hotspot.ps1" -Distro Ubuntu
```

The script detects an active Wi-Fi Direct hotspot adapter or asks which local
IPv4 address to use, forwards TCP 45873 to WSL, and adds a firewall rule scoped
to that address and the local subnet. It prints the Host address for the game.
Rerun it if WSL's IP changes. Check guest connectivity after setup.

Alternatively, configure WSL2 NAT manually instead of using the script. In
**Administrator PowerShell**, substitute your actual hotspot address:

```powershell
$RummyHotspotIP = "192.168.137.1"
$RummyWslIP = ((wsl hostname -I).Trim() -split '\s+')[0]
netsh interface portproxy add v4tov4 listenaddress=$RummyHotspotIP listenport=45873 connectaddress=$RummyWslIP connectport=45873
New-NetFirewallRule -DisplayName "Rummy hotspot" -Direction Inbound -Action Allow -Protocol TCP -LocalAddress $RummyHotspotIP -LocalPort 45873 -RemoteAddress LocalSubnet
```

Refresh the forwarding rule if WSL's IP changes. If already using mirrored
networking, omit the port proxy and allow TCP 45873 through the WSL Hyper-V
firewall as described in [Microsoft's WSL networking guide](https://learn.microsoft.com/en-us/windows/wsl/networking).
If a Linux firewall is active, allow the same TCP port there too.

Troubleshooting: confirm the host lobby is open, the hotspot address is right,
and both devices use the same game build. In guest Windows PowerShell,
`Test-NetConnection <hotspot-IP> -Port 45873` checks reachability. An incorrect
or used code requires a fresh lobby; a port-in-use error means another host
is running. Hotspots with client isolation may block peer traffic.

After use, optional cleanup in Administrator PowerShell (set the same hotspot
address you used during setup):

```powershell
$RummyHotspotIP = "192.168.137.1"
netsh interface portproxy delete v4tov4 listenaddress=$RummyHotspotIP listenport=45873
Get-NetFirewallRule -DisplayName "Rummy hotspot*" -ErrorAction SilentlyContinue | Remove-NetFirewallRule
```

## Another round, reconnecting, and leaving

On the winner screen, Enter / `p` lets the host choose a new mode and wild
rank; the guest waits for that proposal. Both must ready again. The same
session and authenticated seats are retained across rounds.

A transient connection failure pauses actions and retries automatically. The
guest lobby shows a **private rejoin code**: keep it if you may need to restart
the guest app, then paste it in Join. It expires after five minutes without
contact. It restores the same seat, hand, groups, and action sequence; the
initial public join code cannot reclaim that seat. A voluntary quit ends the
session and invalidates credentials. If the host process exits or crashes,
the guest cannot resume that session and should quit, then join a new host.

`q` / Ctrl+C opens quit confirmation (in address/code fields, use Ctrl+C).
Confirm with `y` / Enter, or cancel with `n` / Esc. The alternate terminal
screen is restored on exit. Ending a guest session notifies the host; after
a host process exits the guest may see connection retries and can press `q`.

## Playing

Choose a mode with arrows and Enter, then choose any wild rank **A–K** with
arrows and Enter. Rank shortcuts are A, 2–9, t/0 (ten), J, Q, K; use uppercase
letters for court-card shortcuts. All suits of the chosen rank and every
printed joker are wild for the entire round. Play again lets you change both
mode and rank.

At the handoff screen, pass the terminal and press Enter to reveal the active
hand. Draw once from the closed stock or open discard pile. Arrange the eight
or fourteen cards, then discard exactly one. Older discards are automatically
shuffled into an empty stock while its visible open top remains in place.

| Key | Action |
| --- | --- |
| Left/Right, h/l | Select a card; move it while carrying |
| Space / Enter | Pick up or place a card |
| Esc | Restore a carried card / close an overlay |
| m | Mark/unmark selected card |
| g | Make a draft group from marked cards |
| u | Remove selected card's entire group, ready to edit/rebuild |
| i | Inspect groups, full validity reasons, and joker substitutions |
| z | Clear all groups and marks |
| d / o | Draw from closed / open pile |
| x | Discard selected card; hide hands and pass control |
| v | Declare, using selected card as final discard |
| a | Preview automatic winning groups excluding selected final discard |
| c | Switch overlapping hand / paged grid |
| ? | Help and rules; Up/Down or j/k scroll |
| q / Ctrl+C | Ask to quit; y/Enter confirms, n/Esc cancels |
| Enter / p on winner screen | Start another round |

A gold Cardboard frame identifies the selected card; red means carried.
Marked positions appear above the artwork. Melds
show their cards and classification. Invalid drafts can be saved for editing;
physical cards cannot belong to two groups. Group membership follows card
IDs when cards move. Place or cancel a carried card before other game actions.

To declare, draw first and select your final discard. **With no draft groups,
`v` searches every legal partition** of the remaining hand. **With drafts,
every remaining card must be grouped exactly once.** Remove the final discard
from its draft group with `u`, or clear drafts with `z`. `a` finds and stores a
complete arrangement for the chosen discard; `v` commits it. Failed attempts
preserve all cards, groups, and turn state. The winner screen shows every meld
and joker substitution; Left/Right inspects group artwork. Small winner screens prompt you to resize.

## House rules

- **7-card mode:** one 52-card deck plus two printed jokers; seven cards each.
  All seven must form **exactly two groups of sizes 3 + 4**, including at least
  one pure sequence. The other group may be a sequence or a three/four-card set.
- **13-card mode:** two 52-card decks plus four printed jokers; thirteen cards
  each. Cover all thirteen with valid groups and **at least one pure sequence**.
  All other groups can be sets; a second sequence is **not compulsory**.
- A **sequence** has at least three consecutive cards of the same suit.
  Ace may be low (`AH 2H 3H`) or high (`QH KH AH`), never wrapping (`KH AH 2H`).
  Printed or rank-wild jokers can fill missing positions in an impure sequence.
- A **pure sequence** uses only original faces, without substitutions. With
  six wild, `4H 5H 6H 7H` is pure: 6H represents itself. `4H 5H 6S 7H` is
  impure: 6S represents 6H. Printed jokers cannot form a pure sequence.
- A **set** contains exactly three or four cards of one rank, with different
  natural suits. Jokers can replace missing suits. Two natural 9H cards from
  different decks cannot both be used in one set.
- Every physical card has a distinct Cardboard ID. Duplicate faces are separate
  cards; a physical card cannot be reused. Every declared card must be covered.

## Checks

```sh
gofmt -w cmd internal
go mod tidy
go test ./...
go test -race ./...
go vet ./...
go build -o rummy ./cmd/rummy
```

The race check requires a working Go race toolchain (on WSL amd64, CGO enabled
and a C compiler such as GCC). Tests use seeded deals and deterministic hands
for deck identities, joker interpretations, exhaustive declarations, turn
restrictions, stock recycling, conservation, privacy, and TUI transitions.

## Verification and limitations

The game, two-seat protocol, retry handling, replay, privacy, and TUI transitions
have automated tests, including complete winning hands in both modes and
repeated stock recycling. The transport is tested with framed connections.
Run the commands above for tests, race checks, vet, and a native build.
Agent setup and maintenance instructions are in [AGENTS.md](AGENTS.md).

Rounds are in memory: there is no save file, scoring, or computer opponent.
The host must stay running. This version supports one host and one guest,
using TCP 45873, with the full connection code rather than automatic discovery.
Hotspot connectivity and the Windows setup script still need verification on
the target devices. Local automated tests do not verify those network settings.
