# gitta: Go/tview → Rust/ratatui 移行プロジェクト

## 概要

gitta を Go + tview から Rust + ratatui に書き直す移行計画。

### 現状
- **言語**: Go
- **TUI ライブラリ**: tview + tcell
- **コード規模**: 約 6,200 行
- **主要機能**: Git インタラクティブステージングツール

### 移行後
- **言語**: Rust
- **TUI ライブラリ**: ratatui + crossterm
- **目標**: 同等機能 + より柔軟な UI

---

## フェーズ 1: プロジェクトセットアップ（1-2日）

### タスク
- [ ] 新規 Rust プロジェクト作成（`gitta-rs` または別リポジトリ）
- [ ] 依存関係の設定
  ```toml
  [dependencies]
  ratatui = "0.29"
  crossterm = "0.28"
  tokio = { version = "1", features = ["full"] }
  git2 = "0.19"  # libgit2 バインディング（または std::process で git コマンド実行）
  anyhow = "1.0"
  ```
- [ ] 基本的なアプリケーション構造の作成
- [ ] ターミナル初期化・終了処理
- [ ] イベントループの実装

### 成果物
- 空のウィンドウが表示される最小限のアプリ

---

## フェーズ 2: データ層（2-3日）

### タスク
- [ ] Git 操作モジュールの実装
  - [ ] `get_changed_files()`: staged/unstaged/untracked ファイル取得
  - [ ] `get_file_diff()`: ファイルの差分取得
  - [ ] `get_staged_diff()`: staged な差分取得
- [ ] データ構造の定義
  ```rust
  struct FileEntry {
      path: String,
      stage_status: StageStatus,  // Staged, Unstaged, Untracked
  }

  struct DiffLine {
      content: String,
      line_type: LineType,  // Added, Removed, Context, Header
  }
  ```

### 成果物
- Git 操作が CLI で動作確認できる状態

---

## フェーズ 3: 基本 UI レイアウト（3-4日）

### タスク
- [ ] メインレイアウトの実装
  ```
  ┌─────────────────────────────────────┐
  │ ステータスバー                       │
  ├──────────────┬──────────────────────┤
  │ ファイル      │ 差分ビュー           │
  │ リスト        │                      │
  └──────────────┴──────────────────────┘
  ```
- [ ] ファイルリストコンポーネント
  - [ ] セクション表示（Staged/Unstaged/Untracked）
  - [ ] 選択ハイライト
  - [ ] スクロール
- [ ] 差分ビューコンポーネント（Unified view のみ）
  - [ ] 差分の色付け表示
  - [ ] スクロール
- [ ] ステータスバー
  - [ ] メッセージ表示
  - [ ] キーバインドヘルプ

### 成果物
- ファイル一覧と差分が表示される状態

---

## フェーズ 4: ナビゲーション（2-3日）

### タスク
- [ ] キーバインドシステムの実装
- [ ] ファイルリストのナビゲーション
  - [ ] `j/k` または `↑/↓`: 上下移動
  - [ ] `gg/G`: 先頭/末尾
  - [ ] `Enter`: 差分ビューへフォーカス移動
- [ ] 差分ビューのナビゲーション
  - [ ] `j/k`: カーソル移動
  - [ ] `gg/G`: 先頭/末尾
  - [ ] `Ctrl+E/Ctrl+Y`: スクロール
  - [ ] `Enter`: ファイルリストへ戻る
- [ ] フォーカス管理（左右ペイン間の移動）

### 成果物
- キーボードで自由にナビゲートできる状態

---

## フェーズ 5: 行選択とステージング（4-5日）

### タスク
- [ ] 行選択モードの実装
  - [ ] `V`: 選択モード開始
  - [ ] 選択範囲のハイライト
  - [ ] `Esc`: 選択解除
- [ ] ステージング機能
  - [ ] `a`: 選択行をステージ
  - [ ] `a`（ファイルリスト）: ファイル全体をステージ/アンステージ
  - [ ] `Ctrl+A`: 全ファイルをステージ
- [ ] パッチ生成ロジック
  - [ ] 選択行のみのパッチを生成
  - [ ] `git apply --cached` で適用

### 成果物
- 行単位でステージングができる状態（コア機能完成）

---

## フェーズ 6: コミット機能（2-3日）

### タスク
- [ ] コミットメッセージ入力エリア
  - [ ] `Ctrl+K`: コミットモード開始
  - [ ] マルチライン入力
  - [ ] `Alt+Enter`: コミット実行
  - [ ] `Esc`: キャンセル
- [ ] アメンド機能
  - [ ] `Ctrl+J`: アメンドモード開始
  - [ ] 前回のコミットメッセージを取得して表示
- [ ] コミット実行
  - [ ] `git commit -m "message"`
  - [ ] 成功/失敗のフィードバック

### 成果物
- コミットまでの一連の流れが完成

---

## フェーズ 7: Split View（3-4日）

### タスク
- [ ] Split view レイアウト
  - [ ] Before/After の並列表示
  - [ ] 行番号表示
- [ ] 同期スクロール
- [ ] ビュー切り替え
  - [ ] `s`: Unified ↔ Split トグル

### 成果物
- 2種類のビューモードで差分確認ができる状態

---

## フェーズ 8: 追加機能（3-4日）

### タスク
- [ ] Fold 機能（コンテキスト行の折りたたみ）
  - [ ] `e`: 展開/折りたたみトグル
- [ ] Whitespace 無視
  - [ ] `w`: トグル
  - [ ] `git diff -w` で差分取得
- [ ] コピー機能
  - [ ] `y`: 選択行/ファイル名をコピー
  - [ ] `Ctrl+Y`: ファイルパスをコピー
- [ ] 変更破棄
  - [ ] `d`: 変更破棄（確認ダイアログ付き）
  - [ ] untracked ファイルの削除

### 成果物
- Go 版とほぼ同等の機能

---

## フェーズ 9: Git Log ビュー（2-3日）

### タスク
- [ ] Git log 表示
  - [ ] `t`: Git log ビューを開く
  - [ ] コミット一覧表示
  - [ ] `Enter`: コミット詳細表示
- [ ] コミット詳細ビュー
  - [ ] diff 表示
  - [ ] ナビゲーション

### 成果物
- Git log の閲覧機能

---

## フェーズ 10: 仕上げ（2-3日）

### タスク
- [ ] 自動リフレッシュ機能（`--watch`）
- [ ] エラーハンドリングの改善
- [ ] パフォーマンス最適化
- [ ] テストの追加
- [ ] ドキュメント作成
- [ ] CI/CD セットアップ

### 成果物
- 本番利用可能な状態

---

## 技術的な考慮事項

### アーキテクチャ

```
src/
├── main.rs           # エントリポイント
├── app.rs            # アプリケーション状態管理
├── event.rs          # イベントハンドリング
├── ui/
│   ├── mod.rs
│   ├── file_list.rs  # ファイルリストコンポーネント
│   ├── diff_view.rs  # 差分ビューコンポーネント
│   ├── commit.rs     # コミット入力エリア
│   └── git_log.rs    # Git log ビュー
├── git/
│   ├── mod.rs
│   ├── status.rs     # ファイル状態取得
│   ├── diff.rs       # 差分取得
│   ├── stage.rs      # ステージング操作
│   └── commit.rs     # コミット操作
└── util/
    ├── mod.rs
    └── clipboard.rs  # クリップボード操作
```

### Git 操作の実装方針

2つの選択肢：

1. **`git2` (libgit2)**: ライブラリとして Git 操作
   - メリット: 高速、git コマンド不要
   - デメリット: API が複雑、一部機能が制限される

2. **`std::process::Command`**: git コマンドを実行
   - メリット: シンプル、Go 版と同じ方式
   - デメリット: git コマンドが必要、パース処理が必要

**推奨**: 最初は `std::process::Command` で実装し、必要に応じて `git2` に移行

### 状態管理

```rust
struct App {
    // ファイル関連
    file_list: Vec<FileEntry>,
    current_selection: usize,

    // 差分関連
    diff_lines: Vec<DiffLine>,
    cursor_y: usize,
    select_start: Option<usize>,
    select_end: Option<usize>,

    // ビュー状態
    focus: Focus,  // FileList, DiffView, CommitInput
    view_mode: ViewMode,  // Unified, Split
    is_selecting: bool,
    ignore_whitespace: bool,

    // コミット
    commit_mode: Option<CommitMode>,  // Normal, Amend
    commit_message: String,
}
```

---

## 参考リソース

- [ratatui ドキュメント](https://ratatui.rs/)
- [ratatui examples](https://github.com/ratatui/ratatui/tree/main/examples)
- [tmuxcc](https://github.com/nyanko3141592/tmuxcc) - ratatui を使った参考実装
- [gitui](https://github.com/extrawurst/gitui) - Rust 製の Git TUI（参考になる）

---

## 見積もり

| フェーズ | 期間 |
|---------|------|
| 1. セットアップ | 1-2日 |
| 2. データ層 | 2-3日 |
| 3. 基本 UI | 3-4日 |
| 4. ナビゲーション | 2-3日 |
| 5. 行選択・ステージング | 4-5日 |
| 6. コミット | 2-3日 |
| 7. Split View | 3-4日 |
| 8. 追加機能 | 3-4日 |
| 9. Git Log | 2-3日 |
| 10. 仕上げ | 2-3日 |
| **合計** | **24-34日** |

※ 実際の進捗に応じて調整

---

## マイルストーン

1. **MVP (フェーズ 1-6)**: 基本的なステージング・コミットができる状態
2. **Feature Complete (フェーズ 7-9)**: Go 版と同等機能
3. **Production Ready (フェーズ 10)**: 本番利用可能
