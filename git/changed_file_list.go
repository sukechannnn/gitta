package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileInfo represents a file with its status
type FileInfo struct {
	Path         string
	ChangeStatus string // "added", "deleted", "modified", "untracked"
}

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

func GetChangedFiles(repoPath string) ([]FileInfo, []FileInfo, []FileInfo, error) {
	// Git リポジトリのルートを検索
	gitRoot, err := FindGitRoot(repoPath)
	if err != nil {
		return nil, nil, nil, err
	}
	// Git status --porcelain を実行してstaged/unstagedの両方のファイルを取得
	// -c core.quotepath=false でマルチバイトファイル名をエスケープしないようにする
	cmd := exec.Command("git", "-c", "core.quotepath=false", "status", "--porcelain")
	cmd.Dir = gitRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, nil, err
	}

	var stagedFiles []FileInfo
	var modifiedFiles []FileInfo
	var untrackedFiles []FileInfo

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

		// リネームの場合の処理 (R  old -> new)
		if indexStatus == 'R' {
			// " -> " で分割して古いファイル名と新しいファイル名を取得
			parts := strings.Split(filename, " -> ")
			oldFile := parts[0]
			newFile := parts[1]
			// 古いファイルは削除として、新しいファイルは追加として扱う
			stagedFiles = append(stagedFiles, FileInfo{Path: oldFile, ChangeStatus: "deleted"})
			stagedFiles = append(stagedFiles, FileInfo{Path: newFile, ChangeStatus: "added"})
		} else if worktreeStatus == 'R' {
			// unstaged のリネーム
			parts := strings.Split(filename, " -> ")
			oldFile := parts[0]
			newFile := parts[1]
			modifiedFiles = append(modifiedFiles, FileInfo{Path: oldFile, ChangeStatus: "deleted"})
			modifiedFiles = append(modifiedFiles, FileInfo{Path: newFile, ChangeStatus: "added"})
		} else if indexStatus == '?' && worktreeStatus == '?' {
			// untrackedファイル
			untrackedFiles = append(untrackedFiles, FileInfo{Path: filename, ChangeStatus: "untracked"})
		} else {
			// ステータスに応じて適切な情報を追加
			if indexStatus == 'A' {
				stagedFiles = append(stagedFiles, FileInfo{Path: filename, ChangeStatus: "added"})
			} else if indexStatus == 'D' {
				stagedFiles = append(stagedFiles, FileInfo{Path: filename, ChangeStatus: "deleted"})
			} else if indexStatus == 'M' {
				stagedFiles = append(stagedFiles, FileInfo{Path: filename, ChangeStatus: "modified"})
			} else if indexStatus != ' ' && indexStatus != '?' {
				stagedFiles = append(stagedFiles, FileInfo{Path: filename, ChangeStatus: "modified"})
			}

			// unstagedの変更
			if worktreeStatus == 'D' {
				modifiedFiles = append(modifiedFiles, FileInfo{Path: filename, ChangeStatus: "deleted"})
			} else if worktreeStatus == 'M' {
				modifiedFiles = append(modifiedFiles, FileInfo{Path: filename, ChangeStatus: "modified"})
			} else if worktreeStatus != ' ' && worktreeStatus != '?' {
				modifiedFiles = append(modifiedFiles, FileInfo{Path: filename, ChangeStatus: "modified"})
			}
		}
	}

	return stagedFiles, modifiedFiles, untrackedFiles, nil
}
