# Third-party notices

This release can optionally include third-party executables. Their licenses
and source availability are listed below. The release script copies the
relevant license files into `LICENSES/`.

## yt-dlp

- Project: https://github.com/yt-dlp/yt-dlp
- License: Unlicense (public domain dedication)
- License text: https://raw.githubusercontent.com/yt-dlp/yt-dlp/master/LICENSE

yt-dlp is downloaded from the official release page. YouTube and yt-dlp
change frequently, so a bundled yt-dlp can become outdated. Use
`--yt-dlp PATH` or replace `bin/yt-dlp` to update it.

## FFmpeg / ffprobe

- Project: https://ffmpeg.org/
- License: depends on the build. Static release builds commonly include GPL
  components and are distributed under the GPL.
- Legal information: https://ffmpeg.org/legal.html
- Source code: https://ffmpeg.org/download.html

When redistributing a bundle that includes ffmpeg, comply with the license of
the specific build you use. In particular, GPL builds require providing the
corresponding source and license notices. The release script downloads
`LICENSES/FFmpeg-GPLv3.txt` for convenience, but you must verify the actual
license of the binary you distribute.

## Windows builds

The Windows ffmpeg bundle is downloaded from BtbN's FFmpeg-Builds project:
https://github.com/BtbN/FFmpeg-Builds

## macOS builds

The macOS ffmpeg/ffprobe binaries are downloaded from the ffmpeg-static
project: https://github.com/eugeneware/ffmpeg-static

## This application

The application itself does not relicense yt-dlp or ffmpeg. If you publish a
bundled release, include this file and the files under `LICENSES/` alongside
the executables.
