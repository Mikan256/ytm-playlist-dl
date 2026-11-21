# ytm-playlist-dl 仕様書

## 概要
YouTube Music の公開プレイリストを連番付きでダウンロードし、音量を統一する
CLI / Web UI ツール。

## 依存関係
- Python 3.10+
- yt-dlp (pip)
- ffmpeg / ffprobe（音量処理・メタデータ埋め込みに使用。OSインストール）

Web UI は Python 標準ライブラリの HTTP サーバーで動作し、追加の Web
フレームワークを必要としない。

## ディレクトリ構成
```
ytm-playlist-dl/
├── main.py              # CLIエントリポイント（引数 / 対話 / Web起動）
├── service.py           # CLIとWebで共有する実行パイプライン
├── downloader.py        # yt-dlp を用いたダウンロード処理
├── normalizer.py        # EBU R128 / ReplayGain 音量処理
├── manifest.py          # 正規化済みファイルの記録
├── report.py            # JSONレポート・テキストログ
├── options.py           # 実行オプションの検証
├── models.py            # 共通結果オブジェクト
├── config.py            # 既定値・JSON設定ファイル
├── webui.py             # Web UI / JSON API サーバー
├── web/static/          # HTML / CSS / JavaScript
├── tests/               # unittest による自動テスト
├── pyproject.toml       # パッケージング・console script
├── requirements.txt     # 実行時依存
├── requirements-dev.txt # 開発用依存（ruff / mypy）
├── .github/workflows/   # CI
└── SPEC.md              # 本仕様書
```

## 機能一覧

### 1. プレイリストダウンロード
- `yt-dlp` を使用して YouTube Music の公開プレイリストをダウンロード
- 認証不要の公開プレイリストのみ対応
- 各トラックは音声のみ抽出（動画はダウンロードしない）
- `--start` / `--end` で取得する曲の範囲を指定可能

### 2. 連番ファイル名
- プレイリスト内の順序に従い `01_Title.flac` 形式で出力
- 命名テンプレート:
  - `{num}_{title}`（デフォルト）
  - `{num}_{artist}_{title}`
  - `--template-string` による任意の yt-dlp テンプレート
- 100曲以上でも自然順で扱う

### 3. 出力形式
- デフォルト: FLAC
- オプション: MP3, 元の形式のまま
- MP3 は `--audio-quality` でビットレート指定可能
- 拡張子は選択した形式に自動追従

### 4. 音量正規化（EBU R128 アルバムモード）
- アルバムモード: プレイリスト全体を解析し、統一的に補正
- 2パス方式:
  - 1パス目: 全ファイルのラウドネス計測
  - 2パス目: 計測値に基づき全曲に一括ゲイン適用
- `--loudness` でターゲットを -30〜-5 LUFS から指定（既定 -14）
- `--no-normalize` / `--normalize-mode off` でスキップ
- `original` 形式でも正規化する場合は、ゲイン適用のため音声コーデックへ再エンコードする

### 5. ReplayGainタグ方式
- `--replaygain` / `--normalize-mode replaygain`
- 音声を再エンコードせず、ffmpeg のストリームコピーでタグのみ付与
- `REPLAYGAIN_TRACK_GAIN` / `REPLAYGAIN_TRACK_PEAK` /
  `REPLAYGAIN_ALBUM_GAIN` / `REPLAYGAIN_ALBUM_PEAK` を書き込む

### 6. 正規化済みの再利用
- プレイリストディレクトリの `.ytm-playlist-dl.manifest.json` に記録
- サイズ・更新時刻・モード・ターゲットが一致するファイルはスキップ
- `--force-normalize` で強制再処理

### 7. 再開可能
- 出力先に同名ファイルが存在する場合は yt-dlp が再利用
- 途中で中断しても再実行で続きから再開
- 開始時に `.part` / `.part-FragN` / `.ytdl` をクリーンアップ

### 8. 失敗曲の継続
- `--keep-going` で個別の失敗を飛ばして続行
- 失敗メッセージを収集し、レポート・標準エラーに表示
- 部分失敗時の終了コードは `2`

### 9. 並列処理
- `--jobs N`: 複数トラックを並列ダウンロード
- `--concurrent-fragments N`: 1曲内のフラグメントを並列取得

### 10. メタデータ・カバーアート
- `--embed-metadata`: タイトル・アーティスト等を埋め込み
- `--embed-thumbnail`: サムネイルをダウンロードして埋め込み

### 11. 確認・レポート
- `--list-tracks`: 曲一覧のみ表示
- `--dry-run`: 保存せず出力予定を表示
- `--report-file PATH`: JSON レポートを保存
- `--log-file PATH`: 実行ログを追記
- `--json`: 結果を JSON で標準出力
- レポートにはオプション、プレイリスト情報、成功/失敗、正規化結果を含む

### 12. 設定ファイル
- JSON 形式。既定は `~/.config/ytm-playlist-dl/config.json`
- `XDG_CONFIG_HOME` / `YTM_PLAYLIST_DL_CONFIG` / `--config` で変更可能
- `--save-config` で現在の設定を保存
- 形式・テンプレート・ラウドネス・出力先・正規化モード・並列数などを保持

### 13. Web UI
- `python main.py --web` または `ytm-playlist-dl-web` で起動
- 既定 `http://127.0.0.1:8765/`
- プレイリスト情報取得、設定、ダウンロード、ドライラン、進捗表示、
  レポート表示、設定保存に対応
- バックグラウンドジョブとポーリングで進捗を表示

### 14. パッケージング・CI
- `pyproject.toml` により `ytm-playlist-dl` / `ytm-playlist-dl-web`
  の console script を提供
- GitHub Actions で Python 3.10 / 3.12 のテスト、ruff、mypy（情報提供）を実行

## CLIインターフェース

```
usage: python main.py [-h] [--format {flac,mp3,original}]
                      [--template {num_title,num_artist_title}]
                      [--template-string TEMPLATE] [--loudness LUFS]
                      [--output OUTPUT]
                      [--normalize-mode {album,replaygain,off} |
                       --no-normalize | --replaygain]
                      [--force-normalize] [--start N] [--end N]
                      [--keep-going] [--audio-quality QUALITY]
                      [--embed-metadata] [--embed-thumbnail]
                      [--jobs N] [--concurrent-fragments N]
                      [--dry-run] [--list-tracks]
                      [--report-file PATH] [--log-file PATH] [--json]
                      [--config PATH] [--save-config]
                      [--web] [--host HOST] [--port PORT] [--open-browser]
                      [url]
```

URLを指定した場合は引数モードで動作し、確認プロンプトなしで実行する。
URLを省略した場合は対話モードとなる。`--web` 指定時は Web UI を起動する。

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
| GET | `/api/jobs/<id>/report` | JSONレポート |
| GET | `/health` | ヘルスチェック |

## 処理フロー

```
main.py / webui.py
  │
  ├── 1. 引数 / APIリクエストの検証（options.py）
  │
  ├── 2. service.py
  │     ├── プレイリスト情報取得（タイトル、曲一覧、総数）
  │     ├── list_tracks / dry_run の場合はここで終了
  │     ├── downloader.py
  │     │     ├── 範囲指定・並列処理・リトライ
  │     │     ├── メタデータ / サムネイル / 音質オプション
  │     │     └── 進捗イベント通知
  │     ├── normalizer.py（off 以外）
  │     │     ├── マニフェスト確認・スキップ
  │     │     ├── アルバムモード: 計測→一括ゲイン
  │     │     └── ReplayGain: 計測→タグ書き込み
  │     └── report.py（JSONレポート / ログ）
  │
  └── 3. 結果表示（CLI / Web UI）
```

## エラーハンドリング・終了コード
- 無効なURL（プレイリストIDなし・非対応ホスト） → エラー表示
- ネットワークエラー → リトライ（最大3回）
- 部分ファイルは `.part` として管理、再開時にクリーンアップ
- yt-dlp未インストール → `pip install -r requirements.txt` を促す
- ffmpeg/ffprobe未インストール → インストールを促す
- 1曲も保存できず失敗した場合 → エラー
- 終了コード:
  - `0`: 成功
  - `1`: 致命的エラー
  - `2`: 一部の曲が失敗（`--keep-going` 時など）

## テスト

標準ライブラリの `unittest` を使用する。

```
python -m unittest discover -s tests -v
```

- ネットワーク・yt-dlp・ffmpeg はモック
- URL正規化、ファイル収集、並列処理、ラウドネス計算、ReplayGain、
  マニフェスト、設定、レポート、CLI、Web API を検証
- 偽の ffmpeg / ffprobe を使ったコマンドフローの統合テストを含む

## 注意事項
- 著作権で保護されたコンテンツは個人視聴目的でのみ使用すること
- YouTube Music の仕様変更により動作しなくなる可能性がある
- FLAC出力には元の音源品質以上の音質は望めない
- `original` 形式でアルバムモード正規化を行う場合は再エンコードされる
