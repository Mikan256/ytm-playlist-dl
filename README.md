# ytm-playlist-dl

YouTube Music の公開プレイリストを連番付きでダウンロードし、
EBU R128 のアルバムモードで音量を統一する CLI / Web UI ツールです。

## 主な機能

- 公開プレイリストの音声のみをダウンロード
- 連番ファイル名（`01_曲名.flac`）とカスタム yt-dlp テンプレート
- FLAC / MP3 / 元の形式を選択、MP3 はビットレート指定可能
- プレイリスト全体を解析する 2 パス方式の音量正規化
- ReplayGain タグ方式（再エンコードなし）にも対応
- 正規化済みファイルをマニフェストで記録し、再実行をスキップ
- `--start` / `--end` による部分ダウンロード
- `--keep-going` で失敗曲を飛ばして続行、部分失敗は終了コード 2
- ネットワークエラー時のリトライ（最大 3 回）
- 並列トラックダウンロード（`--jobs`）と並列フラグメント
- メタデータ・カバーアートの埋め込み
- `--dry-run` / `--list-tracks`
- JSON 設定ファイル、JSON レポート、テキストログ
- ブラウザで使えるローカル Web UI（追加の Web フレームワーク不要）
- GitHub Actions によるテスト・Lint

## 必要環境

- Python 3.10+
- [yt-dlp](https://github.com/yt-dlp/yt-dlp)
- ffmpeg / ffprobe（音量処理を使う場合。PATH を通してください）

## セットアップ

```bash
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

# コマンドとしてインストールする場合
pip install -e .
```

ffmpeg は例えば次のようにインストールします。

```bash
# Fedora
sudo dnf install ffmpeg
# Debian / Ubuntu
sudo apt install ffmpeg
# macOS (Homebrew)
brew install ffmpeg
```

## CLI

```bash
# 基本
python main.py "https://music.youtube.com/playlist?list=PL..."

# MP3 + アーティスト名 + 320kbps
python main.py "https://music.youtube.com/playlist?list=PL..." \
  --format mp3 --template num_artist_title --audio-quality 320

# 2曲目から10曲目まで、失敗曲は飛ばす
python main.py "https://music.youtube.com/playlist?list=PL..." \
  --start 2 --end 10 --keep-going

# ReplayGain タグのみ付与（音声は再エンコードしない）
python main.py "https://music.youtube.com/playlist?list=PL..." --replaygain

# 正規化済みでも強制的に再処理
python main.py "https://music.youtube.com/playlist?list=PL..." --force-normalize

# 保存せずに曲一覧・出力予定を確認
python main.py "https://music.youtube.com/playlist?list=PL..." --list-tracks
python main.py "https://music.youtube.com/playlist?list=PL..." --dry-run

# レポートとログを保存
python main.py "https://music.youtube.com/playlist?list=PL..." \
  --report-file reports/run.json --log-file reports/run.log
```

URL を省略すると対話モードになります。

```bash
python main.py
```

インストール後は `ytm-playlist-dl` コマンドでも実行できます。

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
| `--audio-quality QUALITY` | MP3 ビットレート（例: 192, 320） | ffmpeg 任せ |
| `--embed-metadata` | メタデータを埋め込む | 無効 |
| `--embed-thumbnail` | カバーアートを埋め込む | 無効 |
| `--jobs N` | 並列トラック数 | `1` |
| `--concurrent-fragments N` | 1曲内の並列フラグメント数 | `1` |
| `--dry-run` | 保存せず出力予定を表示 | 無効 |
| `--list-tracks` | 曲一覧のみ表示 | 無効 |
| `--report-file PATH` | JSON レポート出力先 | なし |
| `--log-file PATH` | テキストログ出力先 | なし |
| `--json` | 結果を JSON で標準出力 | 無効 |
| `--output DIR` | 出力先ディレクトリ | `./downloads` |

## Web UI

ローカル Web サーバーを起動し、ブラウザから同じ機能を利用できます。

```bash
python main.py --web
# または
python webui.py
# インストール後
ytm-playlist-dl-web

# 起動と同時にブラウザを開く
python main.py --web --open-browser
```

既定では `http://127.0.0.1:8765/` で待ち受けます。
`--host` / `--port` で変更できます。

Web UI では次の操作ができます。

- プレイリスト情報と曲一覧の取得
- 出力形式・テンプレート・音量処理・範囲・並列数の設定
- ダウンロードの開始とリアルタイム進捗表示
- ドライラン
- 設定の保存
- JSON レポートの表示

API は標準ライブラリの HTTP サーバーで実装しています。

| メソッド | パス | 内容 |
| --- | --- | --- |
| `GET` | `/api/config` | 既定値と設定ファイルの内容 |
| `POST` | `/api/config` | 設定の保存 |
| `POST` | `/api/info` | プレイリスト情報の取得 |
| `POST` | `/api/jobs` | ダウンロードジョブの開始 |
| `GET` | `/api/jobs` | ジョブ一覧 |
| `GET` | `/api/jobs/<id>` | ジョブの状態・進捗 |
| `GET` | `/api/jobs/<id>/report` | JSON レポート |

## 設定ファイル

設定は JSON で保存されます。

- 既定: `~/.config/ytm-playlist-dl/config.json`
- `XDG_CONFIG_HOME` がある場合はその配下
- `--config PATH` または環境変数 `YTM_PLAYLIST_DL_CONFIG` で変更可能

```bash
# 現在の設定を保存
python main.py --format mp3 --jobs 2 --save-config
```

## 出力とマニフェスト

出力先は `OUTPUT/プレイリスト名/` です。

- 正規化の記録: `.ytm-playlist-dl.manifest.json`
- ダウンロード中の部分ファイル: `.part` / `.part-FragN`（開始時に削除）
- 音量処理をやり直す場合: `--force-normalize`

## テスト

追加のテスト依存なしで実行できます（標準ライブラリの `unittest`）。

```bash
python -m unittest discover -s tests -v
```

ネットワーク・yt-dlp・ffmpeg に依存しないよう外部処理はモックしています。
また、偽の `ffmpeg` / `ffprobe` を使ったコマンドフローの統合テストも含みます。

開発用の Lint / 型チェック:

```bash
pip install -r requirements-dev.txt
ruff check .
mypy
```

## 注意

- 著作権で保護されたコンテンツは個人視聴目的でのみ使用してください。
- YouTube Music の仕様変更により動作しなくなる可能性があります。
- FLAC 出力でも元音源以上の音質にはなりません。
- `original` 形式でアルバムモードの正規化をする場合は、ゲイン適用のため
  音声を再エンコードします。音質を保ちたい場合は `--replaygain` を使用してください。
