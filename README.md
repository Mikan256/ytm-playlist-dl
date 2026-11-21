# ytm-playlist-dl

YouTube Music の公開プレイリストを連番付きでダウンロードし、
EBU R128 のアルバムモードで音量を統一する Go 製 CLI / Web UI ツールです。

## 主な機能

- 公開プレイリストの音声のみをダウンロード
- 連番ファイル名（`01_曲名.flac`）とカスタム yt-dlp テンプレート
- FLAC / MP3 / 元の形式、MP3 ビットレート指定
- プレイリスト全体を解析する 2 パス方式の音量正規化
- ReplayGain タグ方式（再エンコードなし）
- 正規化済みファイルをマニフェストで記録し、再実行をスキップ
- `--start` / `--end` による部分ダウンロード
- `--keep-going` で失敗曲を飛ばして続行、部分失敗は終了コード `2`
- ネットワークエラー時のリトライ
- 並列トラックダウンロードと並列フラグメント取得
- メタデータ・カバーアートの埋め込み
- `--dry-run` / `--list-tracks`
- JSON 設定、JSON レポート、テキストログ
- ブラウザ UI（レトロな紙面風デザイン、静的ファイルをバイナリに埋め込み）
- 単一バイナリで配布可能

## 必要環境

- Go 1.22+
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) のスタンドアロンバイナリ
- ffmpeg / ffprobe（音量処理・メタデータ埋め込みを使う場合）

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

## CLI

```bash
# 基本
./ytm-playlist-dl "https://music.youtube.com/playlist?list=PL..."

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

- プレイリスト情報・曲一覧の取得
- 出力形式・テンプレート・音量処理・範囲・並列数の設定
- ダウンロード開始とリアルタイム進捗
- ドライラン
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
| `GET` | `/health` | ヘルスチェック |

## 設定ファイル

設定は JSON で保存されます。

- 既定: `~/.config/ytm-playlist-dl/config.json`
- `XDG_CONFIG_HOME` がある場合はその配下
- `--config PATH` または環境変数 `YTM_PLAYLIST_DL_CONFIG` で変更可能

```bash
./ytm-playlist-dl --format mp3 --jobs 2 --save-config
```

## 出力とマニフェスト

出力先は `OUTPUT/プレイリスト名/` です。

- 正規化の記録: `.ytm-playlist-dl.manifest.json`
- ダウンロード中の部分ファイル: `.part` / `.part-FragN`
- 音量処理をやり直す場合: `--force-normalize`

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
