# License files

This directory contains the licenses for the application and the third-party
software that can be bundled into a portable release.

| File | Component | License |
| --- | --- | --- |
| `../LICENSE` | ytm-playlist-dl (this project) | MIT |
| `yt-dlp-Unlicense.txt` | yt-dlp | Unlicense |
| `FFmpeg-GPLv3.txt` | FFmpeg GPL builds | GPL-3.0 |
| `FFmpeg-LGPLv2.1.txt` | FFmpeg LGPL builds | LGPL-2.1 |
| `Go-BSD-3-Clause.txt` | Go runtime / standard library | BSD-3-Clause |
| `Wails-MIT.txt` | Wails v3 | MIT |
| `purego-Apache-2.0.txt` | ebitengine/purego | Apache-2.0 |
| `godbus-BSD-2-Clause.txt` | godbus/dbus | BSD-2-Clause |
| `go-ole-MIT.txt` | go-ole | MIT |
| `coder-websocket-ISC.txt` | coder/websocket | ISC |
| `x-sys-BSD-3-Clause.txt` | golang.org/x/sys | BSD-3-Clause |
| `x-term-BSD-3-Clause.txt` | golang.org/x/term | BSD-3-Clause |
| `pflag-BSD-3-Clause.txt` | github.com/spf13/pflag | BSD-3-Clause |

## FFmpeg

FFmpeg is not part of this repository's source code. Portable releases may
include an FFmpeg binary downloaded from an external provider. The license of
that binary depends on how it was built:

- LGPL builds: comply with the LGPL, including notices and relinking rights.
- GPL builds: comply with the GPL. If you redistribute the binary, provide the
  corresponding source code or a valid written offer, plus the license text.

The release script downloads GPL builds by default. If you redistribute a
bundle, review the exact build's license and satisfy its source/notice
requirements. See `FFmpeg-SOURCE.md` and `../THIRD_PARTY_NOTICES.md`.

## yt-dlp

yt-dlp is distributed under the Unlicense. Replace `bin/yt-dlp` with a newer
official release when YouTube changes require it.

## This project

The MIT license covers this repository's own source code. It does not relicense
yt-dlp, FFmpeg, Go, or pflag. Those components remain under their respective
licenses.
