# rasterm

`rasterm` encodes a Go `image.Image` as terminal graphics, with the Kitty,
iTerm and Sixel protocols, or as text with half blocks (D12). It is one Go
package, `github.com/kenshaw/rasterm`, with no command. `Detect` asks the
terminal which protocols it supports (D26, D27), and `Terminal.Encoder`
returns the encoder for it. Its main user is `github.com/kenshaw/iv`, a
command-line image viewer, which draws each image and each frame of an
animation with `rasterm`.

## Standing rules

These rules hold in every `xo` repository, for every coding agent (dbmeta
D110, adopted here as D8).

1. Stage changes for review. Commit and push only when a maintainer says so.
2. Load the `simple-english` skill before you write any text that a person
   reads: project documentation, a code comment, an error message or a
   commit message. Follow it for that text.
3. Load the `go-pedantry` skill before you write or review Go code. Follow it
   where it does not conflict with a rule in this file. A rule in this file
   wins where the two conflict.

`CLAUDE.md` holds one line that imports this file, so that Claude Code and
every other agent read the same rules. Edit this file, not that one.

## Which document to read

| If you are | Read |
| --- | --- |
| asking why something is the way it is | the decision index in [docs/PLAN.md](docs/PLAN.md) |
| about to decide something that is not written down | the open questions at the end of [docs/PLAN.md](docs/PLAN.md). Ask a maintainer |
| looking for work that is known and not done | [docs/BACKLOG.md](docs/BACKLOG.md) |
| changing how a protocol is detected | `detect.go`, `term_unix.go` and `tmux.go`, then D3, D21, D23, D24, D26, D27 and D28 |
| changing how an encoder writes its output | `encode.go`, then D9. Run the benchmarks below before and after |
| changing the PNG that Kitty or iTerm sends | `png.go`, then D10 and D11 |
| changing what an encoder sends as is | `png.go`, `jpeg.go` and `rgb.go`, then D13 and D14 |
| changing the half-block encoder | `halfblock.go`, then D12. It does not scale and does not add a dependency |
| changing how the output goes through tmux | `tmux.go`, then D20 and D21 |
| changing the animation of Kitty | `animate.go` and `stream.go`, then D25, D28 and D29, and open questions 6 to 8 |
| changing how Kitty draws an image in tmux | `placeholder.go`, then D22. Never edit `placeholder_table.go` by hand |
| adding or changing a test image | [testdata/README.md](testdata/README.md). Record its source, license and SHA-256, and what path of the code it reaches |
| asking why detection or animation has its design | [docs/DETECTION.md](docs/DETECTION.md) and [docs/KITTY_ANIMATION.md](docs/KITTY_ANIMATION.md), the design records of D26 and D25 |
| adding an encoder | `encode.go`, `rasterm.go`, D4, D12 and D26 |
| writing a document, a code comment, an error message or a commit message | the `simple-english` skill. Load it first |
| writing or reviewing Go code | the `go-pedantry` skill. Load it first, then the Go conventions below |
| adding or updating an agent skill | the Agent skills section of [CONTRIBUTING.md](CONTRIBUTING.md), and D8 |

`CONTRIBUTING.md` is the same guide for a person, and shorter. `README.md` is
for a person who uses the package.

A document that is not in this table does not exist. If you cannot find where
something is written down, it is not written down. Ask a maintainer. Do not decide it
yourself, and do not write it as though it were settled.

A reference to a decision of another repository names that repository, as in
dbmeta D110. A bare number, as in D3, names a decision of this repository, in
`docs/PLAN.md`.

## Layout

- `rasterm.go` holds `Protocol`, `Terminal`, `Terminal.Encoder` and its
  options, and the error constants (D6, D26, D27).
- `detect.go` holds `Detect` and its options, the queries to the terminal,
  the scan of the answers, and the names and variables that tell a protocol
  (D3, D26, D27, D28).
- `encode.go` holds the `Encoder` interface, and the Kitty, iTerm and Sixel
  encoders. An encoder reads no package state: `Terminal.Encoder` sets its
  fields. Each encoder writes each image in one call (D9).
- `animate.go` holds `KittyAnimation`, which streams the frames of an
  animation to a Kitty terminal that plays them (D25, D28). `stream.go`
  holds `KittyStream`, which shows pictures as one Kitty image with two
  frames, at the times that the program chooses (D29).
- `halfblock.go` holds `HalfblockEncoder`, which draws an image as text
  (D12).
- `jpeg.go` holds `JPEGImage` and `JPEG`, and the scan of the JPEG markers
  (D13). `rgb.go` holds `RGBImage`, `RGBAImage`, `RGB` and `RGBA` (D14).
  `zlib.go` holds `ZlibMode`, which compresses raw pixels for Kitty (D18).
- `tmux.go` holds the DCS passthrough of tmux, which Kitty and iTerm use
  in a tmux pane (D20), and the query to tmux that finds the terminal (D21).
  It is the only code that starts a process.
- `placeholder.go` draws a Kitty image in tmux with Unicode placeholder
  cells (D22). `placeholder_table.go` holds the 297 diacritics of kitty,
  copied from kitty v0.49.2.
- `png.go` holds `PNGImage` and `PNG` (D10), the PNG encoding that Kitty and
  iTerm share (D11), and the pool of output buffers.
- `term_unix.go` sends the queries of `detect.go` and reads the answers,
  one byte at a time, with a deadline and a context, and reads nothing after
  the answer to DA1 (D23, D27). It sets aside the keys that it reads, and
  `Detect` returns them (D24, D26). `term_other.go` asks nothing on other
  platforms, so `Detect` reads the environment there.
- `testdata/` holds the test images. Git does not convert them.
  `testdata/README.md` records the source, the license and the checksum of
  each one, and `TestEachTestImageHasAProvenance` checks it.
- `rasterm_test.go` and `example_test.go` hold the tests of `Protocol` and
  `Terminal`, and the example. `encode_test.go` holds the tests and the
  benchmarks of D9, D10 and D11, `raw_test.go` holds those of D13 and D14,
  `zlib_test.go` holds those of D18, `tmux_test.go` holds those of D20 and
  D21, `placeholder_test.go` holds those of D22, and `halfblock_test.go`
  holds those of D12. `detect_test.go` holds the tests of `Detect` that need
  no terminal, `term_linux_test.go` holds those that need one, in a
  pseudo-terminal (D23, D24, D26, D27), `animate_test.go` holds those of
  D25, and `stream_test.go` holds those of D29. `docs_test.go` holds the checks of D8, and of `testdata/README.md`.

## Go conventions

These are the conventions that the code follows now. Match them.

- A sentinel error is a constant of `type Error string`, with the `Err`
  prefix (D6). This rule wins over `go-pedantry`, which wants `errors.New`.
- An error string starts with a lowercase letter.
- An exported type that satisfies an interface says so in its doc comment,
  as in "Encode satisfies the [Encoder] interface."
- A package-level default that a caller can change is an exported variable
  with the `Default` prefix, as `DefaultJPEGQuality` and
  `DefaultPNGCompression` are.
- An encoder builds its output in a buffer from `getBuffer` and calls
  `Write` once (D9). Do not write to the writer of the caller in pieces.
- An option of a function or a method is a functional option, as
  `WithTimeout` and `WithNoNewline` are (D27).
- A function that does I/O and can wait takes a `context.Context` as its
  first argument, named `ctx` (D27).
- No code reads the environment or asks the terminal outside `Detect`. An
  encoder gets what it needs from its fields (D26).
- A comment inside a function is short and starts with a lowercase letter.
  A doc comment is a full sentence that starts with the name.
- A test is in package `rasterm`, not in a `_test` package, except the
  example. A failure message has the form "expected X, got: Y".
- The module requires `go-sixel`, `golang.org/x/sys` and `golang.org/x/term`.
  Ask a maintainer before you add another dependency. Keep the `golang.org/x`
  modules at their newest release, and the go directive at the release that
  they need (D15).
- Use the idioms of the Go release that `go.mod` declares. `go fix -diff
  ./...` must print nothing.

## Tests, benchmarks and lint

Run these in the repository root before you stage a change. Each must pass,
and `gofmt -l .` must print nothing:

```sh
gofmt -l .
go vet ./...
go fix -diff ./...
TERM_GRAPHICS=sixel go test -count=1 ./...
```

CI runs the build and the tests on Linux, macOS and Windows, on the Go
release of `go.mod` and on the latest (D16). `term_other.go` builds on
Windows only, so run `GOOS=windows go vet ./...` after you change it.

`TestEncode` fails with "term graphics not available" when no protocol is
detected, so set `TERM_GRAPHICS` as CI does. `Detect` then asks the
terminal nothing. The test writes each test image to the terminal that runs
the test. Open question 4 in `docs/PLAN.md` asks the maintainers
about this.

A change that can make an encoder faster or slower must show its effect.
Run the benchmarks before and after the change, with the same dependencies,
and compare the two with `benchstat`:

```sh
go test -run '^$' -bench . -benchtime 10x -count 6 . > new.txt
go run golang.org/x/perf/cmd/benchstat@latest old.txt new.txt
```

`BenchmarkEncode` measures each encoder on each test image.
`BenchmarkEncodePNG` measures D10, and `BenchmarkPNGCompression` measures
D11. Each reports `writes/op`, which is the calls to `Write`, and
`bytes/op`, which is the size of the output. Write the result in the
decision that the change makes.

`.golangci.yml` is out of date, and CI does not run `golangci-lint`. Open
question 2 in `docs/PLAN.md` asks the maintainers about it.

## Writing documentation

A new document goes in `docs/`. Only `README.md`, `AGENTS.md`, `CLAUDE.md`
and `CONTRIBUTING.md` belong in the root, and `TestTheRootHoldsFourDocuments`
fails on any other. `testdata/README.md` is the one document outside them,
because it describes the files next to it. Add a new document to the table
above.

A decision goes in `docs/PLAN.md` and nowhere else (dbmeta D111). Give it the
next number and a heading such as `### D<n>. Title. Decided.` Use `Proposed`
if no maintainer chose it. If it changes an earlier decision, write
`Amends D5` in its heading and `Amended by D<n>` in the heading of the
earlier one. Add its row to the index. `TestTheDecisionIndexIsComplete`
prints the row when it is missing, and `TestAnAmendmentPointsBothWays` fails
unless both headings name each other.

Work that is known and not done goes in `docs/BACKLOG.md`, with where it came
from. When an item is done, delete it, and record anything decided in
`docs/PLAN.md`.
