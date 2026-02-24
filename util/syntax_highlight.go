package util

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/rivo/tview"
)

// Diff background colors
const (
	AddedLineBg   = "#133813"
	DeletedLineBg = "#381313"
)

// syntaxStyle is the chroma style used for syntax highlighting
var syntaxStyle = styles.Get("catppuccin-frappe")

// defaultTextColor is the fallback text color
const defaultTextColor = "#f8f8f2"

// tokenCache caches tokenized results using a hash key for fast comparison
var tokenCache struct {
	sync.Mutex
	key    uint64
	tokens [][]chroma.Token
}

// hashCacheKey computes a fast hash for the cache key
func hashCacheKey(filePath string, codeLines []string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(filePath))
	h.Write([]byte{0})
	for _, line := range codeLines {
		h.Write([]byte(line))
		h.Write([]byte{'\n'})
	}
	return h.Sum64()
}

// TokenizeCode tokenizes code lines using chroma.
// It caches the result for repeated calls with the same filePath and code.
func TokenizeCode(filePath string, codeLines []string) [][]chroma.Token {
	cacheKey := hashCacheKey(filePath, codeLines)

	tokenCache.Lock()
	if tokenCache.key == cacheKey {
		result := tokenCache.tokens
		tokenCache.Unlock()
		return result
	}
	tokenCache.Unlock()

	codeText := strings.Join(codeLines, "\n")

	lexer := lexers.Match(filePath)
	if lexer == nil {
		lexer = lexers.Analyse(codeText)
	}
	if lexer == nil {
		return nil
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, codeText)
	if err != nil {
		return nil
	}

	// Build per-line token slices
	result := make([][]chroma.Token, len(codeLines))
	lineIdx := 0
	for _, tok := range iterator.Tokens() {
		if lineIdx >= len(codeLines) {
			break
		}
		// Split tokens that span multiple lines
		parts := strings.Split(tok.Value, "\n")
		for i, part := range parts {
			if i > 0 {
				lineIdx++
				if lineIdx >= len(codeLines) {
					break
				}
			}
			if part != "" {
				result[lineIdx] = append(result[lineIdx], chroma.Token{
					Type:  tok.Type,
					Value: part,
				})
			}
		}
	}

	tokenCache.Lock()
	tokenCache.key = cacheKey
	tokenCache.tokens = result
	tokenCache.Unlock()

	return result
}

// resolveTokenColor returns the hex color for a token type using the chroma style.
func resolveTokenColor(tt chroma.TokenType) string {
	if syntaxStyle == nil {
		return defaultTextColor
	}
	entry := syntaxStyle.Get(tt)
	if entry.Colour.IsSet() {
		return fmt.Sprintf("#%06x", int(entry.Colour)&0xFFFFFF)
	}
	return defaultTextColor
}

// RenderHighlightedLine renders tokens into a tview color-tagged string with the given background.
func RenderHighlightedLine(tokens []chroma.Token, bgColor string) string {
	if len(tokens) == 0 {
		if bgColor != "" {
			return "[:#" + bgColor[1:] + "] [-:-]"
		}
		return ""
	}

	var sb strings.Builder
	for _, tok := range tokens {
		fg := resolveTokenColor(tok.Type)
		escaped := tview.Escape(tok.Value)
		if bgColor != "" {
			sb.WriteString("[" + fg + ":" + bgColor + "]" + escaped + "[-:-]")
		} else {
			sb.WriteString("[" + fg + "]" + escaped + "[-]")
		}
	}
	return sb.String()
}

// ReplaceBackground replaces background color portions in a tview color-tagged string.
// It scans for [fg:bg] patterns and replaces bg with newBg.
// For tags like [fg] (no bg), it inserts the background.
func ReplaceBackground(line string, newBg string) string {
	var sb strings.Builder
	i := 0
	for i < len(line) {
		if line[i] == '[' {
			// Find closing bracket
			end := strings.IndexByte(line[i:], ']')
			if end == -1 {
				sb.WriteByte(line[i])
				i++
				continue
			}
			tag := line[i+1 : i+end]
			// Check if this looks like a color tag (not an escaped bracket like "[")
			if strings.Contains(tag, "[]") || len(tag) == 0 {
				// Escaped bracket or empty tag, pass through
				sb.WriteString(line[i : i+end+1])
				i += end + 1
				continue
			}

			// Check for region tags ["xxx"] - pass through
			if len(tag) > 0 && tag[0] == '"' {
				sb.WriteString(line[i : i+end+1])
				i += end + 1
				continue
			}

			parts := strings.SplitN(tag, ":", 2)
			if len(parts) == 2 {
				// [fg:bg] -> [fg:newBg]
				fg := parts[0]
				sb.WriteString("[" + fg + ":" + newBg + "]")
			} else {
				// [fg] -> [fg:newBg]
				sb.WriteString("[" + tag + ":" + newBg + "]")
			}
			i += end + 1
		} else {
			sb.WriteByte(line[i])
			i++
		}
	}
	return sb.String()
}
