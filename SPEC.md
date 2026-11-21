# ytm-playlist-dl 仕様書

## 概要
YouTube Music の公開プレイリストを連番付きでダウンロードし、音量を統一する
Go 製 CLI / Web UI ツール。

## 依存関係
- Go 1.22+（ビルド時）
- yt-dlp（スタンドアロンバイナリ）
- ffmpeg / ffprobe（音量処理・メタデータ埋め込みに使用）

同梱バンドル版には yt-dlp / ffmpeg / ffprobe を含めることができる。
Go 版は yt-dlp をサブプロセスとして実行する。Python ランタイムは不要。
Web UI は Go 標準ライブラリの HTTP サーバーで動作し、静的ファイルは
バイナリに埋め込まれる。

## ディレクトリ構成
```text
ytm-playlist-dl/
├── cmd/
│   ├── ytm-playlist-dl/       # CLI エントリポイント
│   └── ytm-playlist-dl-web/   # Web UI エントリポイント
├── internal/
│   ├── binmgr/                # yt-dlp / ffmpeg / ffprobe の探索
│   ├── cli/                   # 引数解析・対話モード
│   ├── config/                # JSON 設定
│   ├── downloader/            # yt-dlp 連携・ファイル収集・並列処理
│   ├── ytdlp/                 # yt-dlp 実行・JSON/進捗解析
│   ├── normalizer/            # EBU R128 / ReplayGain
│   ├── manifest/              # 正規化済み記録
│   ├── model/                 # オプション・結果・イベント
│   ├── report/                # JSON レポート・ログ
│   ├── service/               # CLI/Web 共通パイプライン
│   ├── testutil/              # テスト用ヘルパー
│   ├── urlutil/               # URL 正規化・ファイル名サニタイズ
│   └── webui/                 # HTTP API・ジョブ管理
├── scripts/
│   └── build-bundle.sh        # 同梱配布アーカイブの生成
├── web/
│   ├── embed.go               # 静的ファイルの埋め込み
│   └── static/                # HTML / CSS / JavaScript
├── LICENSE                    # このプロジェクトの MIT ライセンス
├── LICENSES/                  # 第三者ライセンス全文
├── THIRD_PARTY_NOTICES.md
├── go.mod
├── go.sum
├── .github/workflows/ci.yml
├── .github/workflows/release.yml
└── SPEC.md
```

## 機能一覧

### 1. プレイリストダウンロード
- yt-dlp で公開プレイリストを取得
- 認証不要の公開プレイリストのみ対応
- 音声のみを抽出
- `--start` / `--end` で範囲指定

### 2. 連番ファイル名
- `01_Title.flac` 形式
- `num_title` / `num_artist_title`
- `--template-string` による任意の yt-dlp テンプレート
- 100曲以上も自然順で扱う

### 3. 出力形式
- FLAC（デフォルト）/ MP3 / original
- `--audio-quality` で MP3 ビットレート指定
- 拡張子は形式に追従

### 4. 音量正規化（EBU R128 アルバムモード）
- プレイリスト全体を解析し、統一的に補正
- 1パス目: 全ファイルのラウドネス計測
- 2パス目: 計測値に基づく一括ゲイン適用
- `--loudness`（-30〜-5 LUFS、既定 -14）
- `--normalize-jobs` で解析・適用を並列化
- アルバムモードで original 形式を使う場合は再エンコード

### 5. ReplayGainタグ方式
- `--replaygain`
- 再エンコードせず `-c copy` でタグのみ付与
- トラック/アルバムのゲイン・ピークを書き込む

### 6. 正規化済みの再利用
- `.ytm-playlist-dl.manifest.json` に記録
- サイズ・更新時刻・モード・ターゲットが一致する場合はスキップ
- `--force-normalize` で強制再処理

### 7. 再開・部分ファイル
- 既存ファイルは yt-dlp が再利用
- `.part` / `.part-FragN` / `.ytdl` を開始時に削除

### 8. 失敗曲の継続
- `--keep-going`
- 失敗メッセージを収集してレポートに記録
- 部分失敗時の終了コードは `2`

### 9. 並列処理
- `--jobs N`: 複数トラックを並列ダウンロード
  - プレイリスト情報を1回取得し、各曲の直接URLをワーカーに渡す
- `--concurrent-fragments N`: 曲内のフラグメント並列数
- `--normalize-jobs N`: ffmpeg 解析・適用の並列数

### 10. メタデータ・カバーアート
- `--embed-metadata`
- `--embed-thumbnail`

### 11. 確認・レポート
- `--list-tracks`
- `--dry-run`
- `--report-file PATH`
- `--log-file PATH`
- `--json`
- レポートにはオプション、プレイリスト、成功/失敗、正規化結果を含む

### 12. 設定ファイル
- JSON 形式
- 既定: `~/.config/ytm-playlist-dl/config.json`
- `XDG_CONFIG_HOME` / `YTM_PLAYLIST_DL_CONFIG` / `--config`
- `--save-config`

### 13. Web UI
- `--web` または `ytm-playlist-dl-web`
- 既定 `http://127.0.0.1:8765/`
- プレイリスト情報、設定、ダウンロード、ドライラン、進捗、レポート
- 温かみのあるレトロな紙面風デザイン
- 静的ファイルは `embed` でバイナリに同梱

### 14. ビルド・CI
- 単一バイナリ（CGO 不要）
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- gofmt チェック
- GitHub Actions（Go 1.22 / 1.23）

### 15. ポータブルバンドル
- `scripts/build-bundle.sh` で yt-dlp / ffmpeg / ffprobe を同梱
- Linux amd64/arm64、macOS amd64/arm64、Windows amd64
- 実行ファイル隣の `bin/` を自動探索
- 探索順:
  1. 明示パス
  2. 環境変数
  3. 実行ファイルと同じディレクトリ
  4. 実行ファイル隣の `bin/`
  5. カレントディレクトリ / `./bin/`
  6. `PATH`
- GitHub Actions の `Build portable bundles` ワークフローで生成可能
- 同梱する ffmpeg のライセンスに応じて `LICENSES/` と
  `THIRD_PARTY_NOTICES.md` を同梱する

## CLI

```text
ytm-playlist-dl [options] [url]

--format {flac,mp3,original}
--template {num_title,num_artist_title}
--template-string TEMPLATE
--loudness LUFS
--output DIR
--normalize-mode {album,replaygain,off}
--no-normalize / --replaygain
--force-normalize
--start N / --end N
--keep-going
--audio-quality QUALITY
--embed-metadata / --embed-thumbnail
--jobs N / --normalize-jobs N / --concurrent-fragments N
--dry-run / --list-tracks
--report-file PATH / --log-file PATH / --json
--config PATH / --save-config
--web / --host HOST / --port PORT / --open-browser
--yt-dlp PATH / --ffmpeg PATH
--version
```

URL 省略時は対話モード。`--web` 指定時は Web UI を起動する。

## Web API

| メソッド | パス | 内容 |
| --- | --- | --- |
| GET | `/` | Web UI |
| GET | `/api/config` | 既定値・設定ファイル |
| POST | `/api/config` | 設定保存 |
| POST | `/api/info` | プレイリスト情報取得 |
| POST | `/api/jobs` | ジョブ開始 |
| GET | `/api/jobs` | ジョブ一覧 |
| GET | `/api/jobs/<id>` | ジョブ状態・進捗イベント |
| GET | `/api/jobs/<id>/report` | JSON レポート |
| GET | `/health` | ヘルスチェック |

## 処理フロー

```text
cli / webui
  │
  ├── 1. オプション検証（model.Options.Validate）
  │
  ├── 2. service.Service.Execute
  │     ├── yt-dlp でプレイリスト情報取得
  │     ├── list_tracks / dry_run はここで終了
  │     ├── downloader
  │     │     ├── 範囲指定・並列・リトライ
  │     │     ├── メタデータ / サムネイル / 音質
  │     │     └── 進捗イベント
  │     ├── normalizer（off 以外）
  │     │     ├── マニフェスト確認・スキップ
  │     │     ├── アルバムモード
  │     │     └── ReplayGain
  │     └── report
  │
  └── 3. 結果表示（CLI / Web UI）
```

## エラーハンドリング・終了コード
- 無効なURL（プレイリストIDなし・非対応ホスト） → エラー
- ネットワークエラー → yt-dlp のリトライ + 外側のリトライ
- 部分ファイル → `.part` / `.part-FragN` / `.ytdl` を開始時に削除
- yt-dlp 未検出 → `--yt-dlp` または PATH を案内
- ffmpeg/ffprobe 未検出 → インストールを案内
- 終了コード:
  - `0`: 成功
  - `1`: 致命的エラー
  - `2`: 一部の曲が失敗

## テスト
- 標準 `testing` パッケージ
- 偽の `yt-dlp` / `ffmpeg` / `ffprobe` スクリプトによる統合テスト
- URL正規化、設定、オプション、マニフェスト、レポート
- ダウンロード（直列・並列・keep-going）
- アルバム正規化・ReplayGain・マニフェストスキップ
- CLI 引数解析
- Web API / ジョブ管理

```bash
go test ./...
go test -race ./...
go vet ./...
```

## 注意事項
- 著作権で保護されたコンテンツは個人視聴目的でのみ使用すること
- YouTube Music の仕様変更により動作しなくなる可能性がある
- FLAC 出力でも元音源以上の音質にはならない
- original 形式のアルバム正規化では再エンコードされる
- yt-dlp / ffmpeg は外部依存

## ライセンス
- このリポジトリの自作コード: MIT License（`LICENSE`）
- yt-dlp: Unlicense（`LICENSES/yt-dlp-Unlicense.txt`）
- FFmpeg / ffprobe: GPL-3.0 または LGPL-2.1（ビルドによる。
  `LICENSES/FFmpeg-GPLv3.txt`、`LICENSES/FFmpeg-LGPLv2.1.txt`）
- Go runtime: BSD-3-Clause（`LICENSES/Go-BSD-3-Clause.txt`）
- github.com/spf13/pflag: BSD-3-Clause
  （`LICENSES/pflag-BSD-3-Clause.txt`）

ポータブル配布物には `LICENSE`、`THIRD_PARTY_NOTICES.md`、`LICENSES/` を
同梱する。FFmpeg の GPL ビルドを再配布する場合は、対応するソースコードの
提供など GPL の条件を満たすこと。
