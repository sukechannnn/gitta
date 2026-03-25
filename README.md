# giff

**giff** is a terminal UI tool for interactively staging Git hunks and lines as minimal patches.
It's like `git add -p`, but better — with a keyboard-driven interface and visual diff selection.

![demo](./docs/images/giff_demo.gif)

## Features

- Visual Git diff viewer with syntax highlighting and inline diff highlighting
- Unified view (default) and split view (side-by-side) modes
- Interactive line/hunk selection with Vim-like keys
- Apply selected changes as minimal patches (`git apply --cached`)
- File tree with directory folding and collapsing
- Full-text search within diffs
- Code folding for unchanged regions
- Git log viewer with shared diff/file list components
- Real-time file watching with `--watch` flag
- Whitespace-change hiding toggle

---

## Usage

```bash
cd path/to/git/repository
giff
```

### Key Bindings — File List

| Key | Action |
|-----|--------|
| `j` / `k` | Move cursor up/down |
| `H` / `L` | Navigate directories (collapse/expand) |
| `Enter` | Switch to diff view |
| `a` | Stage/unstage selected file |
| `A` | Stage/unstage entire file |
| `d` | Discard changes (or delete untracked file) |
| `Ctrl+A` | Stage all files |
| `Ctrl+K` | Open commit message input |
| `Ctrl+J` | Amend last commit |
| `s` | Toggle split view |
| `w` | Toggle whitespace hiding |
| `v` | Open file in vim |
| `t` | Open git log viewer |
| `Y` | Copy file path to clipboard |
| `Ctrl+E` / `Ctrl+Y` | Scroll diff view down/up |
| `q` | Quit |

### Key Bindings — Diff View

| Key | Action |
|-----|--------|
| `j` / `k` | Move cursor up/down |
| `g` + `g` / `G` | Go to top / bottom |
| `V` | Start/stop line selection mode |
| `a` | Stage selected lines |
| `A` | Stage/unstage entire file |
| `/` | Search in diff |
| `n` / `N` | Next/previous search match |
| `e` | Expand/collapse fold |
| `s` | Toggle split view |
| `w` | Toggle whitespace hiding |
| `y` | Yank (copy) current/selected lines |
| `Y` | Copy file path to clipboard |
| `Ctrl+L` | Copy file reference (path:line) to clipboard |
| `Ctrl+E` / `Ctrl+Y` | Scroll down/up |
| `Enter` / `Esc` | Return to file list |
| `q` | Quit |

### Command Line Options

```bash
# Enable auto-refresh mode (watch for file changes)
giff --watch

# Enable debug mode (output logs to tmp/)
giff --debug
```

With `--watch` enabled, giff will automatically refresh the file list and diff view as you edit files.

---

## Installation

```bash
go install github.com/sukechannnn/giff@latest

echo 'export PATH="$PATH:$HOME/go/bin"' >> ~/.bashrc
source ~/.bashrc
```

Or clone and build:

```bash
git clone https://github.com/sukechannnn/giff.git
cd giff
go build -o giff

mv giff /usr/local/bin  # or any directory in your $PATH
```

---

## Why?

Sometimes you want to commit just part of a change — a few lines, not the whole file.
`giff` gives you full control over what gets staged, with a cleaner, more intuitive UI than `git add -p`.

---

## License

MIT License
