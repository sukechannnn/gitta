package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FindGitRoot は現在のディレクトリから上位階層へ遡って .git ディレクトリを探します
func FindGitRoot(startPath string) (string, error) {
	dir, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	for {
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// ルートディレクトリに達した
			break
		}
		dir = parent
	}

	return "", os.ErrNotExist
}

func GetChangedFiles(repoPath string) ([]string, []string, []string, error) {
	// Git リポジトリのルートを検索
	gitRoot, err := FindGitRoot(repoPath)
	if err != nil {
		return nil, nil, nil, err
	}
	// Git status --porcelain を実行してstaged/unstagedの両方のファイルを取得
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = gitRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, nil, err
	}

	var stagedFiles []string
	var modifiedFiles []string
	var untrackedFiles []string

	// 出力を解析 (TrimSpaceは使わず、空行のみ除去)
	lines := strings.Split(string(output), "\n")
	// 空行を除去
	var filteredLines []string
	for _, line := range lines {
		if len(line) > 0 {
			filteredLines = append(filteredLines, line)
		}
	}
	lines = filteredLines
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		// git status --porcelain の形式: XY filename
		// X = index status, Y = worktree status
		indexStatus := line[0]
		worktreeStatus := line[1]
		filename := strings.TrimSpace(line[2:]) // 2文字目の後からファイル名

		// untrackedファイル
		if indexStatus == '?' && worktreeStatus == '?' {
			untrackedFiles = append(untrackedFiles, filename)
		} else {
			// stagedの変更があるファイル (indexStatus が空白以外)
			if indexStatus != ' ' && indexStatus != '?' {
				stagedFiles = append(stagedFiles, filename)
			}
			// unstagedの変更があるファイル (worktreeStatus が空白以外)
			if worktreeStatus != ' ' && worktreeStatus != '?' {
				modifiedFiles = append(modifiedFiles, filename)
			}
		}
	}

	return stagedFiles, modifiedFiles, untrackedFiles, nil
}
