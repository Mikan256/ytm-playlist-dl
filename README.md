# ytm-playlist-dl

YouTube Music の公開プレイリストを連番付きでダウンロードし、
EBU R128 のアルバムモードで音量を統一する CLI ツールです。

## 必要環境

- Python 3.10+
- [yt-dlp](https://github.com/yt-dlp/yt-dlp)（`requirements.txt` でインストール）
- ffmpeg / ffprobe（OS にインストールし、PATH を通してください）

## セットアップ

```bash
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
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

## 使い方

URL を指定すると、確認なしで引数モードで実行します。

```bash
python main.py "https://music.youtube.com/playlist?list=PL..."
```

```bash
# MP3 で保存し、ファイル名を「番号_アーティスト_曲名」にする
python main.py "https://music.youtube.com/playlist?list=PL..." \
  --format mp3 --template num_artist_title

# 出力先とターゲットラウドネスを指定
python main.py "https://music.youtube.com/playlist?list=PL..." \
  --output ~/Music --loudness -16

# 音量正規化をスキップ
python main.py "https://music.youtube.com/playlist?list=PL..." --no-normalize
```

URL を省略すると対話モードになります。

```bash
python main.py
```

主なオプション:

| オプション | 説明 | デフォルト |
| --- | --- | --- |
| `--format {flac,mp3,original}` | 出力形式 | `flac` |
| `--template {num_title,num_artist_title}` | ファイル名テンプレート | `num_title` |
| `--loudness LUFS` | ターゲットラウドネス（-30〜-5） | `-14` |
| `--output DIR` | 出力先ディレクトリ | `./downloads` |
| `--no-normalize` | 音量正規化をスキップ | 無効 |

出力先は `OUTPUT/プレイリスト名/` です。ファイル名は
`01_曲名.flac` のようにプレイリスト順の連番になります。

## 機能

- 公開プレイリストの音声のみをダウンロード
- FLAC / MP3 / 元の形式を選択可能
- プレイリスト全体を解析する 2 パス方式の音量正規化
- 既存ファイルを利用した再開（`.part` は開始時にクリーンアップ）
- ネットワークエラー時のリトライ（最大 3 回）
- 曲ごとの進捗表示（`[1/15] Downloaded: ...`）

## テスト

追加のテスト依存は不要です（標準ライブラリの `unittest` を使用）。

```bash
python -m unittest discover -s tests -v
```

ネットワーク、yt-dlp、ffmpeg に依存しないよう外部処理はモックし、
URL 正規化、ファイル収集、ラウドネス計算、ゲイン適用、CLI 解析などを
検証しています。

## 注意

- 著作権で保護されたコンテンツは個人視聴目的でのみ使用してください。
- YouTube Music の仕様変更により動作しなくなる可能性があります。
- FLAC 出力でも元音源以上の音質にはなりません。
- `original` 形式で音量正規化する場合は、ゲイン適用のために音声を
  再エンコードします。
