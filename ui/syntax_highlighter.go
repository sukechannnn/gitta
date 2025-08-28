package ui

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/rivo/tview"
)

var (
	// キャッシュされたフォーマッターとスタイル
	cachedFormatter chroma.Formatter
	cachedStyle     *chroma.Style

	// 差分のキャッシュ（ファイルパス -> 処理済み差分）
	diffCache = make(map[string]string)
	// 差分の内容ハッシュ（変更検知用）
	diffHashCache = make(map[string]string)
)

func init() {
	// 起動時にフォーマッターとスタイルをキャッシュ
	cachedFormatter = formatters.Get("terminal256")
	if cachedFormatter == nil {
		cachedFormatter = formatters.Fallback
	}

	cachedStyle = styles.Get("vulcan")
	if cachedStyle == nil {
		cachedStyle = styles.Fallback
	}
}

// 簡易ハッシュ関数（差分の変更検知用）
func hashDiff(diff string) string {
	// 簡単のため、最初の100文字と長さを使用
	if len(diff) < 100 {
		return diff
	}
	return diff[:100] + fmt.Sprint(len(diff))
}

// ApplySyntaxHighlight applies syntax highlighting to code using ANSI output
func ApplySyntaxHighlight(code string, filePath string) string {
	// 空文字列の場合はそのまま返す
	if code == "" {
		return code
	}

	// ファイル拡張子から言語を推定（Analyseは遅いので使わない）
	lexer := lexers.Match(filePath)
	if lexer == nil {
		// Analyseをスキップしてfallbackを使用
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// トークナイズ
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return tview.Escape(code)
	}

	// フォーマット（キャッシュされたフォーマッターとスタイルを使用）
	var buf bytes.Buffer
	err = cachedFormatter.Format(&buf, cachedStyle, iterator)
	if err != nil {
		return tview.Escape(code)
	}

	// ANSIコードをtviewのカラータグに変換
	return tview.TranslateANSI(buf.String())
}

// PreloadDiffCache preloads diff syntax highlighting into cache
func PreloadDiffCache(diff string, filePath string) {
	// キャッシュキーを生成
	cacheKey := filePath + ":" + hashDiff(diff)

	// すでにキャッシュにある場合はスキップ
	if _, ok := diffCache[cacheKey]; ok {
		return
	}

	// 通常の処理を実行してキャッシュに保存
	_ = ColorizeDiffWithSyntax(diff, filePath)
}

// ColorizeDiffWithSyntax applies syntax highlighting to a diff with ANSI output
func ColorizeDiffWithSyntax(diff string, filePath string) string {
	// キャッシュキーを生成
	cacheKey := filePath + ":" + hashDiff(diff)

	// キャッシュをチェック
	if cached, ok := diffCache[cacheKey]; ok {
		return cached
	}

	lines := strings.Split(diff, "\n")
	var result []string

	for _, line := range lines {
		// ヘッダー行をスキップ（元のColorizeDiffの動作に合わせる）
		if isUnifiedHeaderLine(line) {
			continue
		}

		if len(line) == 0 {
			result = append(result, "")
			continue
		}

		// 差分のプレフィックスを判定して色付け
		switch line[0] {
		case '+':
			// 追加行
			content := ""
			if len(line) > 1 {
				content = line[1:]
			}
			// シンタックスハイライトを適用
			highlighted := ApplySyntaxHighlight(content, filePath)
			// プレフィックスを緑色に
			result = append(result, "[green]+[-]"+highlighted)
		case '-':
			// 削除行
			content := ""
			if len(line) > 1 {
				content = line[1:]
			}
			// シンタックスハイライトを適用
			highlighted := ApplySyntaxHighlight(content, filePath)
			// プレフィックスを赤色に
			result = append(result, "[red]-[-]"+highlighted)
		case ' ':
			// コンテキスト行（先頭のスペースを削除して、スペースでインデントを合わせる）
			content := ""
			if len(line) > 1 {
				content = line[1:]
			}
			// シンタックスハイライトを適用
			highlighted := ApplySyntaxHighlight(content, filePath)
			// 先頭にスペースを追加してインデントを合わせる
			result = append(result, " "+highlighted)
		default:
			// その他の行（通常はないはず）
			result = append(result, tview.Escape(line))
		}
	}

	resultStr := strings.Join(result, "\n") + "\n"

	// キャッシュに保存（最大50ファイルまで）
	if len(diffCache) > 50 {
		// 古いエントリを10個削除（一度に複数削除して頻繁な削除を避ける）
		deleteCount := 0
		for k := range diffCache {
			delete(diffCache, k)
			delete(diffHashCache, k)
			deleteCount++
			if deleteCount >= 10 {
				break
			}
		}
	}
	diffCache[cacheKey] = resultStr

	return resultStr
}
