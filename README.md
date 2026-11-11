# rummy-tui

A complete local, two-human hot-seat Rummy game for a Linux/WSL terminal.
Choose seven or thirteen cards and the wild rank before each round. Draw,
carry/reorder cards, build melds, discard, and declare. Each turn ends behind a
privacy screen that requires the next player to press Enter.

[Cardboard](https://github.com/DukeAvi/cardboard) supplies all card identities,
decks, piles, transfers, carrying, and Vintage pixel artwork. Rummy rules live
in `internal/game`; Bubble Tea and Lip Gloss provide `internal/tui`.

## WSL setup and launch

Requires **Go 1.24+**, Git, and an interactive WSL Linux terminal. Windows
Terminal with Unicode and truecolor support is recommended. No Windows binary,
GUI, browser, server, or C library is required for normal play.

For **large Cardboard cards**, maximize the terminal and aim for **180 columns
× 60 rows**. The artwork appears automatically at **88 columns × 36 rows or
larger**; each card is 26×20 terminal cells. Check your current size with
`stty size`, which prints **rows, then columns**. You can increase the terminal
font size for physically larger cards, provided enough rows and columns remain.

Launch this checkout directly from WSL:

```sh
cd /home/duke/code/rummy
go run -buildvcs=false ./cmd/rummy
```

Choose 7-card or 13-card mode, choose the wild rank, then confirm the first
player's handoff. The hand starts in overlapping artwork view; press `c` for
the grid. Left/Right moves through the hand and reveals cards outside the
current view. Stock and open discard artwork appear beside your hand.

The workspace reserves the top-level `.git` directory. The existing Git
history is stored in `rummy-tui/.git` with the project root as its working
tree. Run Git commands as `git -C rummy-tui status` (or replace `status` with
the desired Git command). The build commands disable VCS stamping so Go can
build with this workspace layout.

This checkout uses the existing Cardboard source without modifying or copying
it. Its `go.mod` replacement expects this directory layout:

```text
code/
├── cardboard/             # existing github.com/DukeAvi/cardboard checkout
└── rummy/                 # this project
```

For a fresh installation, download dependencies once from the repository
directory (this needs network access unless they are already cached):

```sh
go mod download
```

To build and launch a native Linux executable:

```sh
go build -buildvcs=false -o rummy ./cmd/rummy
./rummy
```

If your Cardboard checkout is elsewhere, point the module replacement at it:

```sh
go mod edit -replace=github.com/DukeAvi/cardboard=/absolute/path/to/cardboard
go mod tidy
```

Alternatively, with network access, remove the local replacement to download
the pinned Cardboard revision from GitHub through Go's module proxy:

```sh
go mod edit -dropreplace=github.com/DukeAvi/cardboard
go mod tidy
```

The development environment used a local replacement because downloads were
blocked. All other dependencies are recorded in `go.mod` and `go.sum`.

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
Marked positions appear above the artwork, or as `*` in compact mode. Melds
show their cards and classification. Invalid drafts can be saved for editing;
physical cards cannot belong to two groups. Group membership follows card
IDs when cards move. Place or cancel a carried card before other game actions.

To declare, draw first and select your final discard. **With no draft groups,
`v` searches every legal partition** of the remaining hand. **With drafts,
every remaining card must be grouped exactly once.** Remove the final discard
from its draft group with `u`, or clear drafts with `z`. `a` finds and stores a
complete arrangement for the chosen discard; `v` commits it. Failed attempts
preserve all cards, groups, and turn state. The winner screen shows every meld
and joker substitution; Left/Right inspects group artwork. Compact winner
screens scroll with j/k.

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
go build -buildvcs=false -o rummy ./cmd/rummy
```

The race check requires a working Go race toolchain (on WSL amd64, CGO enabled
and a C compiler such as GCC). Tests use seeded deals and deterministic hands
for deck identities, joker interpretations, exhaustive declarations, turn
restrictions, stock recycling, conservation, privacy, and TUI transitions.

## Display and limitations

Use **180 columns × 60 rows** for comfortable large-card play; **88 columns ×
36 rows** is the artwork threshold. Cardboard's pixel cards stay at their full
26×20-cell size, with stock and discard artwork beside the hand. Overlap and grid views follow
the selected card, so not every card must fit at once. Smaller terminals use
readable ranks/suits; `i` inspects full groups and reasons with scrolling. Very
short terminals can show only part of the controls. Resize at any time.

Rounds are local and in memory: there is no save/resume, scoring, networking,
or computer opponent. Privacy hides the inactive hand from the interface; it
cannot prevent another person from watching the screen after confirmation.
The local module replacement must be adjusted if the directory layout changes.
