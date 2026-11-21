# Third-party notices

This project is licensed under the MIT License. See `LICENSE`.

Portable releases can include third-party executables and libraries. Their
licenses are listed below. Full license texts are in `LICENSES/`.

## Summary

| Component | License | License file |
| --- | --- | --- |
| ytm-playlist-dl | MIT | `LICENSE` |
| yt-dlp | Unlicense | `LICENSES/yt-dlp-Unlicense.txt` |
| FFmpeg / ffprobe | GPL-3.0 or LGPL-2.1 depending on build | `LICENSES/FFmpeg-GPLv3.txt`, `LICENSES/FFmpeg-LGPLv2.1.txt` |
| Go runtime / standard library | BSD-3-Clause | `LICENSES/Go-BSD-3-Clause.txt` |
| Wails v3 | MIT | `LICENSES/Wails-MIT.txt` |
| ebitengine/purego | Apache-2.0 | `LICENSES/purego-Apache-2.0.txt` |
| godbus/dbus | BSD-2-Clause | `LICENSES/godbus-BSD-2-Clause.txt` |
| go-ole | MIT | `LICENSES/go-ole-MIT.txt` |
| coder/websocket | ISC | `LICENSES/coder-websocket-ISC.txt` |
| golang.org/x/sys | BSD-3-Clause | `LICENSES/x-sys-BSD-3-Clause.txt` |
| golang.org/x/term | BSD-3-Clause | `LICENSES/x-term-BSD-3-Clause.txt` |
| golang.org/x/mod | BSD-3-Clause | `LICENSES/x-mod-BSD-3-Clause.txt` |
| github.com/spf13/pflag | BSD-3-Clause | `LICENSES/pflag-BSD-3-Clause.txt` |

## yt-dlp

- Project: https://github.com/yt-dlp/yt-dlp
- License: Unlicense (public domain dedication)
- Included file: `LICENSES/yt-dlp-Unlicense.txt`

yt-dlp is downloaded from the official release page. YouTube and yt-dlp change
frequently, so a bundled yt-dlp can become outdated. Use `--yt-dlp PATH` or
replace `bin/yt-dlp` to update it.

## FFmpeg / ffprobe

- Project: https://ffmpeg.org/
- License: depends on the build. The portable release script downloads GPL
  builds by default, but the actual binary may be GPL or LGPL.
- Included files:
  - `LICENSES/FFmpeg-GPLv3.txt`
  - `LICENSES/FFmpeg-LGPLv2.1.txt`
  - `LICENSES/FFmpeg-SOURCE.md`
- Legal information: https://ffmpeg.org/legal.html
- Source code: https://ffmpeg.org/download.html

If you redistribute a bundle that includes FFmpeg, comply with the license of
the specific build. GPL builds require the corresponding source code or a
valid written offer, together with the license notices. The release script
copies these notices, but it cannot determine or satisfy your obligations for
the exact binary you choose to distribute.

## Go runtime

The compiled Go binaries include the Go runtime and standard library.

- Project: https://go.dev/
- License: BSD-3-Clause
- Included file: `LICENSES/Go-BSD-3-Clause.txt`

## Wails v3 and desktop dependencies

The desktop application uses Wails v3 for the native window.

- Wails: https://github.com/wailsapp/wails — MIT
  (`LICENSES/Wails-MIT.txt`)
- ebitengine/purego: https://github.com/ebitengine/purego — Apache-2.0
  (`LICENSES/purego-Apache-2.0.txt`)
- godbus/dbus: https://github.com/godbus/dbus — BSD-2-Clause
  (`LICENSES/godbus-BSD-2-Clause.txt`)
- go-ole: https://github.com/go-ole/go-ole — MIT
  (`LICENSES/go-ole-MIT.txt`)
- coder/websocket: https://github.com/coder/websocket — ISC
  (`LICENSES/coder-websocket-ISC.txt`)
- golang.org/x/sys: BSD-3-Clause
  (`LICENSES/x-sys-BSD-3-Clause.txt`)
- golang.org/x/term: BSD-3-Clause
  (`LICENSES/x-term-BSD-3-Clause.txt`)
- golang.org/x/mod: BSD-3-Clause
  (`LICENSES/x-mod-BSD-3-Clause.txt`)

The desktop app uses the operating system's WebView (WebView2, WKWebView,
GTK/WebKitGTK). Those runtime libraries are not relicensed by this project.

## github.com/spf13/pflag

- Project: https://github.com/spf13/pflag
- License: BSD-3-Clause
- Included file: `LICENSES/pflag-BSD-3-Clause.txt`

## Redistribution checklist

When publishing a portable bundle:

- keep `LICENSE`, `THIRD_PARTY_NOTICES.md`, and `LICENSES/` in the archive;
- verify the license of the bundled FFmpeg build;
- provide the corresponding FFmpeg source or a valid written offer if required;
- do not imply that the MIT license covers third-party components.
