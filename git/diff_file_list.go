package git

import (
	"github.com/go-git/go-git/v5"
)

func GetModifiedFiles(repoPath string) ([]string, []string, error) {
	// Git リポジトリを開く
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, nil, err
	}

	// 作業ツリーを取得
	wt, err := repo.Worktree()
	if err != nil {
		return nil, nil, err
	}

	// 差分のあるファイルを取得
	status, err := wt.Status()
	if err != nil {
		return nil, nil, err
	}

	var modifiedFiles []string
	var untrackedFiles []string

	for file, fileStatus := range status {
		switch {
		case fileStatus.Worktree == git.Untracked: // 新規追加ファイル
			untrackedFiles = append(untrackedFiles, file)
		case fileStatus.Worktree != git.Unmodified: // 差分がある既存ファイル
			modifiedFiles = append(modifiedFiles, file)
		}
	}

	return modifiedFiles, untrackedFiles, nil
}
