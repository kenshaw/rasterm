# Contributing to rasterm

Read three documents before you change anything.

[`AGENTS.md`](AGENTS.md) holds the rules: the standing rules, the layout, the
Go conventions and the commands to run. It is written for an AI coding agent,
and each rule in it applies to a person too. `CLAUDE.md` holds one line that
imports it, for Claude Code.

[`docs/PLAN.md`](docs/PLAN.md) holds the plan and each decision that this
project made. The index at the top lists each decision with its status. The
open questions for Ken are at the end.

[`docs/BACKLOG.md`](docs/BACKLOG.md) holds the work that is known and not
done.

Do not decide an open question on your own. Ask Ken.

## Before you send a change

Run these commands in the repository root. Each must pass, and `gofmt -l .`
must print nothing:

```sh
gofmt -l .
go vet ./...
go fix -diff ./...
TERM_GRAPHICS=sixel go test -count=1 ./...
```

CI runs the tests on Linux, macOS and Windows.

`TERM_GRAPHICS=sixel` makes the Sixel encoder available, as CI does. Without
a terminal that shows graphics, `TestEncode` fails.

If your change can make an encoder faster or slower, run the benchmarks
before and after it, and put the `benchstat` output in the pull request. The
Tests, benchmarks and lint section of `AGENTS.md` has the commands.

## Agent skills

The repository carries two agent skills. A skill is a set of instructions
that a coding agent loads for a task. `simple-english` sets how prose is
written, and `go-pedantry` sets how Go is written.

`skills-lock.json` names the source of each skill. The `skills` command from
npm writes that file, and version 1.7.0 is the version that was used. It
writes each skill into two folders. Codex and the other agents read
`.agents/skills/<name>`, and Claude Code reads `.claude/skills/<name>`.

To install or update a skill, run its command in the repository root:

```sh
npx skills@1.7.0 add AminBlg/SimpleEnglish --skill simple-english --agent codex claude-code --copy -y
npx skills@1.7.0 add oborchers/fractional-cto --skill go-pedantry --agent codex claude-code --copy -y
```

Keep `--copy`. Without it, the command writes `.claude/skills/<name>` as a
symbolic link. A Windows checkout writes a symbolic link as a text file, and
Claude Code then loads no skill and reports nothing. `TestSkillsAreCopies`
fails on a link, and it fails when the two folders differ. See D8.

`.claude/settings.local.json` holds the Claude Code permissions of one
person. The root `.gitignore` ignores it.
