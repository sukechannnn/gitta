# gitta

**gitta** is a terminal UI tool for interactively staging Git hunks and lines as minimal patches.  
It's like `git add -p`, but better — with a keyboard-driven interface and visual diff selection.

![demo](./docs/images/gitta_demo.gif)

## ✨ Features

- Visual Git diff viewer in the terminal (default unified view, `s` toggle split view mode)
- Interactive selection of lines/hunks using Vim-like keys (`j`, `k`, `V`, etc.)
- Apply selected changes as minimal patches (`git apply --cached`)
- Real-time file watching with `--watch` flag
- Fast, minimal, and works well with real Git repositories

---

## 🚀 Usage

```bash
cd path/to/git/repository
gitta
```

Use the arrow keys or `j` / `k` to move, `V` to start visual selection, and `U` to stage the selected diff.

### Key Bindings

Navigation:
- `j` / `k` or arrow keys — move cursor up/down
- `g` + `g` — go to top
- `G` — go to bottom
- `enter` — switch between file list and diff view
- `q` — quit

Actions:
- `V` — start/stop line selection mode
- `a` — stage selected lines
- `A` — stage/unstage entire file
- `s` — toggle split view (side-by-side diff)

### Command Line Options

Enable auto-refresh mode to watch for file changes:
```bash
gitta --watch
```

With `--watch` enabled, gitta will automatically refresh the file list and diff view, allowing you to see changes in near real-time as you edit files in your editor.

Enable debug mode and output logs to `tmp/`:
```bash
gitta --debug
```

---

## 📦 Installation

```bash
go install github.com/sukechannnn/gitta@latest

echo 'export PATH="$PATH:$HOME/go/bin"' >> ~/.bashrc
source ~/.bashrc
```

Or clone and build:

```bash
git clone https://github.com/sukechannnn/gitta.git
cd gitta
go build -o gitta

mv gitta /usr/local/bin  # or any directory in your $PATH
```

---

## 💡 Why?

Sometimes you want to commit just part of a change — a few lines, not the whole file.  
`gitta` gives you full control over what gets staged, with a cleaner, more intuitive UI than `git add -p`.

---

## 📋 License

MIT License
