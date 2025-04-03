# gitta

**gitta** is a terminal UI tool for interactively staging Git hunks and lines as minimal patches.  
It's like `git add -p`, but better — with a keyboard-driven interface and visual diff selection.

![screenshot](screenshot.png) <!-- optional: replace with actual screenshot -->

---

## ✨ Features

- Visual Git diff viewer in the terminal
- Interactive selection of lines/hunks using Vim-like keys (`j`, `k`, `V`, etc.)
- Apply selected changes as minimal patches (`git apply --cached`)
- Toggleable debug mode (`--debug`) to see patch output and apply results
- Fast, minimal, and works well with real Git repositories

---

## 🚀 Usage

```bash
cd path/to/your/file
gitta
```

Use the arrow keys or `j` / `k` to move, `V` to start visual selection, and `U` to stage the selected diff.

Press:
- `V` — start/stop selecting
- `U` — apply selected patch (`git add` equivalent)
- `w` — go back to list page
- `q` — quit

Enable debug mode:

```bash
gitta --debug
```

---

## 📦 Installation

```bash
go install github.com/yourname/gitta@latest

echo 'export PATH="$PATH:$HOME/go/bin"' >> ~/.bashrc
source ~/.bashrc
```

Or clone and build:

```bash
git clone https://github.com/yourname/gitta.git
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
