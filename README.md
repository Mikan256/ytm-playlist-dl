# ytm-playlist-dl

YouTube Music の公開プレイリストを連番付きでダウンロードし、
EBU R128 のアルバムモードで音量を統一する Go 製 CLI / Web UI ツールです。

## 主な機能

- 公開プレイリストの音声のみをダウンロード
- プレイリストだけでなく、単体動画・Shorts・youtu.be リンクにも対応
- 連番ファイル名（`01_曲名.flac`）とカスタム yt-dlp テンプレート
- FLAC / MP3 / 元の形式、MP3 ビットレート指定
- プレイリスト全体を解析する 2 パス方式の音量正規化
- ReplayGain タグ方式（再エンコードなし）
- 正規化済みファイルをマニフェストで記録し、再実行をスキップ
- `--start` / `--end` による部分ダウンロード
- `--keep-going` で失敗曲を飛ばして続行、部分失敗は終了コード `2`
- ネットワークエラー時のリトライ
- 並列トラックダウンロードと並列フラグメント取得
- `--zip` で出力フォルダを ZIP にまとめる
- メタデータ・カバーアートの埋め込み
- `--dry-run` / `--list-tracks`
- JSON 設定、JSON レポート、テキストログ
- ブラウザ UI（レトロな紙面風デザイン、静的ファイルをバイナリに埋め込み）
- yt-dlp / ffmpeg を同梱したポータブル配布が可能

## 必要環境

- Go 1.22+（ビルドする場合）
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) のスタンドアロンバイナリ
- ffmpeg / ffprobe（音量処理・メタデータ埋め込みを使う場合）

同梱バンドル版は yt-dlp / ffmpeg / ffprobe をすべて含むため、
これらを別途インストールする必要はありません。

Go 版は `yt-dlp` をサブプロセスとして実行します。Python ランタイムは不要です。

## ビルド

```bash
go build -o ytm-playlist-dl ./cmd/ytm-playlist-dl
go build -o ytm-playlist-dl-web ./cmd/ytm-playlist-dl-web
```

`go install` を使う場合:

```bash
go install github.com/Mikan256/ytm-playlist-dl/cmd/ytm-playlist-dl@latest
go install github.com/Mikan256/ytm-playlist-dl/cmd/ytm-playlist-dl-web@latest
```

## ポータブル配布（yt-dlp / ffmpeg 同梱）

`scripts/build-bundle.sh` を使うと、yt-dlp / ffmpeg / ffprobe を同梱した
配布用アーカイブを作成できます。受け取った側は展開して実行するだけで、
別途 yt-dlp や ffmpeg をインストールする必要はありません。

```bash
# 現在の OS / CPU 向け
scripts/build-bundle.sh

# Linux ARM64 向け
scripts/build-bundle.sh --os linux --arch arm64

# ローカルのバイナリを使う場合（オフライン環境など）
scripts/build-bundle.sh \
  --yt-dlp ./yt-dlp \
  --ffmpeg ./ffmpeg \
  --ffprobe ./ffprobe
```

生成物:

```text
dist/
├── ytm-playlist-dl-0.4.1-linux-amd64.tar.gz
├── ytm-playlist-dl-0.4.1-windows-amd64.zip
└── ...
```

展開後の構成:

```text
ytm-playlist-dl-linux-amd64/
├── ytm-playlist-dl
├── ytm-playlist-dl-web
├── bin/
│   ├── yt-dlp
│   ├── ffmpeg
│   └── ffprobe
├── LICENSE
├── LICENSES/
├── README.md
├── SPEC.md
└── THIRD_PARTY_NOTICES.md
```

対応ターゲット:

- Linux amd64 / arm64
- macOS amd64 / arm64
- Windows amd64

### バイナリの探索順

同梱バイナリは次の順で自動探索されます。

1. `--yt-dlp PATH` / `--ffmpeg PATH`
2. `YTDLP_PATH` / `FFMPEG_PATH` / `FFPROBE_PATH`
3. 実行ファイルと同じディレクトリ
4. 実行ファイル隣の `bin/` ディレクトリ
5. カレントディレクトリと `./bin/`
6. `PATH`

そのため、アーカイブ内の `ytm-playlist-dl` を直接実行すれば、
`bin/` の yt-dlp / ffmpeg が使われます。PATH を空にしても動作します。

### 注意

- 同梱しても、ダウンロードにはネットワーク接続が必要です。
- YouTube 側の変更で yt-dlp の更新が必要になることがあります。
  その場合は `bin/yt-dlp` を新しいものに差し替えるか、`--yt-dlp PATH` を指定してください。
- ffmpeg のライセンスはビルドによって異なります。同梱・再配布時は
  `THIRD_PARTY_NOTICES.md`、`LICENSE`、`LICENSES/` を必ず同梱してください。
- GitHub Actions の `Build portable bundles` ワークフローでも
  Linux / macOS / Windows 向けバンドルを生成できます。

## ライセンス

このリポジトリの自作コードは **MIT License** です。全文は `LICENSE` を
参照してください。

同梱・利用する第三者ソフトウェアは、それぞれのライセンスに従います。

- yt-dlp: Unlicense
- FFmpeg / ffprobe: GPL-3.0 または LGPL-2.1（ビルドによる）
- Go runtime / standard library: BSD-3-Clause
- github.com/spf13/pflag: BSD-3-Clause

詳細と全文:

- `THIRD_PARTY_NOTICES.md`
- `LICENSES/README.md`
- `LICENSES/FFmpeg-SOURCE.md`

ポータブル配布物には `LICENSE`、`THIRD_PARTY_NOTICES.md`、`LICENSES/` が
自動的に同梱されます。FFmpeg の GPL ビルドを再配布する場合は、対応する
ソースコードの提供など GPL の条件を満たす必要があります。

## CLI

```bash
# 基本（プレイリスト）
./ytm-playlist-dl "https://music.youtube.com/playlist?list=PL..."

# 単体動画・Shorts・youtu.be
./ytm-playlist-dl "https://www.youtube.com/watch?v=VIDEO_ID"
./ytm-playlist-dl "https://youtube.com/shorts/VIDEO_ID"
./ytm-playlist-dl "https://youtu.be/VIDEO_ID"

# ZIPにまとめる
./ytm-playlist-dl "https://www.youtube.com/watch?v=VIDEO_ID" \
  --output ./downloads --zip

# MP3 + アーティスト名 + 320kbps
./ytm-playlist-dl "https://music.youtube.com/playlist?list=PL..." \
  --format mp3 --template num_artist_title --audio-quality 320

# 2曲目から10曲目まで、失敗曲は飛ばす
./ytm-playlist-dl "https://music.youtube.com/playlist?list=PL..." \
  --start 2 --end 10 --keep-going

# ReplayGain タグのみ付与
./ytm-playlist-dl "https://music.youtube.com/playlist?list=PL..." --replaygain

# 正規化済みでも強制再処理
./ytm-playlist-dl "https://music.youtube.com/playlist?list=PL..." --force-normalize

# 保存せずに確認
./ytm-playlist-dl "https://music.youtube.com/playlist?list=PL..." --list-tracks
./ytm-playlist-dl "https://music.youtube.com/playlist?list=PL..." --dry-run

# レポートとログ
./ytm-playlist-dl "https://music.youtube.com/playlist?list=PL..." \
  --report-file reports/run.json --log-file reports/run.log
```

URL を省略すると対話モードになります。

```bash
./ytm-playlist-dl
```

### 主なオプション

| オプション | 説明 | デフォルト |
| --- | --- | --- |
| `--format {flac,mp3,original}` | 出力形式 | `flac` |
| `--template {num_title,num_artist_title}` | ファイル名テンプレート | `num_title` |
| `--template-string TEMPLATE` | yt-dlp 形式のカスタムテンプレート | なし |
| `--loudness LUFS` | ターゲットラウドネス（-30〜-5） | `-14` |
| `--normalize-mode {album,replaygain,off}` | 音量処理 | `album` |
| `--no-normalize` | 音量処理をスキップ | - |
| `--replaygain` | ReplayGain タグを付与 | - |
| `--force-normalize` | 正規化済みでも再実行 | 無効 |
| `--start N` / `--end N` | 取得する曲の範囲 | 全体 |
| `--keep-going` | 失敗曲を飛ばして続行 | 無効 |
| `--audio-quality QUALITY` | MP3 ビットレート（例: 192, 320） | yt-dlp 任せ |
| `--embed-metadata` | メタデータを埋め込む | 無効 |
| `--embed-thumbnail` | カバーアートを埋め込む | 無効 |
| `--jobs N` | 並列トラック数 | `1` |
| `--normalize-jobs N` | 並列正規化数 | `1` |
| `--concurrent-fragments N` | 1曲内の並列フラグメント数 | `1` |
| `--dry-run` | 保存せず出力予定を表示 | 無効 |
| `--list-tracks` | 曲一覧のみ表示 | 無効 |
| `--report-file PATH` | JSON レポート出力先 | なし |
| `--log-file PATH` | テキストログ出力先 | なし |
| `--zip` | 出力フォルダを ZIP にまとめる | 無効 |
| `--json` | 結果を JSON で標準出力 | 無効 |
| `--output DIR` | 出力先ディレクトリ | `./downloads` |
| `--yt-dlp PATH` | yt-dlp バイナリのパス | PATH から探索 |
| `--ffmpeg PATH` | ffmpeg バイナリのパス | PATH から探索 |

## Web UI

```bash
./ytm-playlist-dl --web
# または
./ytm-playlist-dl-web

# 起動と同時にブラウザを開く
./ytm-playlist-dl --web --open-browser
```

既定では `http://127.0.0.1:8765/` で待ち受けます。
`--host` / `--port` で変更できます。

Web UI では以下が可能です。

- プレイリスト情報・曲一覧の取得（単体動画にも対応）
- 出力形式・テンプレート・音量処理・範囲・並列数の設定
- ダウンロード開始とリアルタイム進捗
- ドライラン
- ZIP まとめ出力
- 設定保存
- JSON レポート表示

### Web API

| メソッド | パス | 内容 |
| --- | --- | --- |
| `GET` | `/api/config` | 既定値と設定ファイル |
| `POST` | `/api/config` | 設定保存 |
| `POST` | `/api/info` | プレイリスト情報取得 |
| `POST` | `/api/jobs` | ダウンロードジョブ開始 |
| `GET` | `/api/jobs` | ジョブ一覧 |
| `GET` | `/api/jobs/<id>` | ジョブ状態・進捗 |
| `GET` | `/api/jobs/<id>/report` | JSON レポート |
| `GET` | `/api/jobs/<id>/archive` | ZIPアーカイブ |
| `GET` | `/health` | ヘルスチェック |

## 設定ファイル

設定は JSON で保存されます。

- 既定: `~/.config/ytm-playlist-dl/config.json`
- `XDG_CONFIG_HOME` がある場合はその配下
- `--config PATH` または環境変数 `YTM_PLAYLIST_DL_CONFIG` で変更可能

```bash
./ytm-playlist-dl --format mp3 --jobs 2 --zip --save-config
```

## 出力とマニフェスト

出力先は `OUTPUT/タイトル/` です（プレイリスト名または動画タイトル）。

- 正規化の記録: `.ytm-playlist-dl.manifest.json`
- ダウンロード中の部分ファイル: `.part` / `.part-FragN`
- 音量処理をやり直す場合: `--force-normalize`
- ZIP 出力: `--zip` で `OUTPUT/タイトル.zip` を作成

## テスト

```bash
go test ./...
go test -race ./...
go vet ./...
```

ネットワーク・yt-dlp・ffmpeg に依存しないよう、テストでは偽の
`yt-dlp` / `ffmpeg` / `ffprobe` スクリプトを使用しています。

GitHub Actions で Go 1.22 / 1.23 のテスト、race test、vet、gofmt、
ビルドを実行します。

## 構成

```text
cmd/                      CLI / Web のエントリポイント
internal/cli/             引数解析・対話モード
internal/config/          JSON 設定
internal/downloader/      yt-dlp 連携・並列ダウンロード
internal/ytdlp/           yt-dlp サブプロセス・進捗解析
internal/normalizer/      EBU R128 / ReplayGain
internal/manifest/        正規化済み記録
internal/report/          JSON レポート・ログ
internal/service/         CLI / Web 共通処理
internal/webui/           HTTP API・ジョブ管理
web/static/               ブラウザ UI
```

## 注意

- 著作権で保護されたコンテンツは個人視聴目的でのみ使用してください。
- YouTube Music の仕様変更により動作しなくなる可能性があります。
- FLAC 出力でも元音源以上の音質にはなりません。
- `original` 形式でアルバムモードの正規化をする場合は、ゲイン適用のため
  音声を再エンコードします。音質を保ちたい場合は `--replaygain` を使用してください。
- yt-dlp / ffmpeg は引き続き外部依存です。
