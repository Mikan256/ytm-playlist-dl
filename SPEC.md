# ytm-playlist-dl 仕様書

## 概要
YouTube Music の公開プレイリストを連番付きでダウンロードし、音量を統一するCLIツール。

## 依存関係
- Python 3.10+
- yt-dlp (pip)
- ffmpeg (OSインストール必須)

## ディレクトリ構成
```
ytm-playlist-dl/
├── main.py              # CLIエントリポイント
├── downloader.py        # yt-dlp を用いたダウンロード処理
├── normalizer.py        # EBU R128 ラウドネス正規化
├── config.py            # 設定値・デフォルト値の管理
├── requirements.txt     # Python依存パッケージ
└── SPEC.md              # 本仕様書
```

## 機能一覧

### 1. プレイリストダウンロード
- `yt-dlp` を使用して YouTube Music の公開プレイリストをダウンロード
- 認証不要の公開プレイリストのみ対応
- 各トラックは音声のみ抽出（動画はダウンロードしない）

### 2. 連番ファイル名
- プレイリスト内の順序に従い `01_Title.flac` 形式で出力
- 命名テンプレートはユーザーが選択可能
  - `{num}_{title}`（デフォルト）
  - `{num}_{artist}_{title}`
  - 等

### 3. 出力形式
- デフォルト: FLAC
- オプション: MP3, 元の形式のまま
- 拡張子は選択した形式に自動追従

### 4. 音量正規化（EBU R128 アルバムモード）
- アルバムモード: プレイリスト全体を解析し、統一的に -14 LUFS に補正
- 2パス方式:
  - 1パス目: 全ファイルのラウドネス計測
  - 2パス目: 計測値に基づき全曲に一括ゲイン適用
- オプションでスキップ可能

### 5. 再開可能
- 出力先に同名ファイルが存在する場合はスキップ
- 途中で中断しても再実行で続きから再開

### 6. 進捗表示
- `[1/15] Downloading: Song Title...`
- `[2/15] Normalizing: Song Title...`
- 全体の進行状況を常に表示

## CLIインターフェース

```
usage: python main.py [-h] [--format {flac,mp3,original}]
                      [--template {num_title,num_artist_title}]
                      [--loudness LOUDNESS] [--output OUTPUT]
                      [--no-normalize]
                      url

positional arguments:
  url                   YouTube Music プレイリストURL

options:
  -h, --help            ヘルプ表示
  --format {flac,mp3,original}
                        出力形式 (default: flac)
  --template {num_title,num_artist_title}
                        ファイル名テンプレート
                        (default: num_title)
  --loudness LOUDNESS   ターゲットラウドネス [LUFS]
                        (default: -14)
  --output OUTPUT       出力先ディレクトリ
                        (default: ./downloads)
  --no-normalize        音量正規化をスキップ
```

## 処理フロー

```
main.py
  │
  ├── 1. 引数パース (argparse)
  │
  ├── 2. downloader.py
  │     ├── プレイリスト情報取得（タイトル、トラック一覧、総数）
  │     ├── 出力先作成 (output/PlaylistName/)
  │     └── 全曲ダウンロード
  │           └── 各完了後に [i/N] Downloading: Title 表示
  │
  ├── 3. normalizer.py (--no-normalize 時はスキップ)
  │     ├── 1パス目: 全ファイルのラウドネス解析
  │     ├── アルバム統合ラウドネス計算
  │     └── 2パス目: 全曲にゲイン適用、出力先に書き出し
  │
  └── 4. 完了メッセージ表示
```

## エラーハンドリング
- 無効なURL → エラーメッセージ表示して終了
- ネットワークエラー → リトライ（最大3回）
- ダウンロード中の部分ファイルは `.part` として管理、再開時にクリーンアップ
- ffmpeg未インストール → インストールを促すメッセージ

## 注意事項
- 著作権で保護されたコンテンツは個人視聴目的でのみ使用すること
- YouTube Music の仕様変更により動作しなくなる可能性がある
- FLAC出力には元の音源品質以上の音質は望めない
