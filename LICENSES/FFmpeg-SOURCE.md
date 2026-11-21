# FFmpeg source information

Portable releases built by `scripts/build-bundle.sh` download FFmpeg from one
of these providers:

- Linux: https://johnvansickle.com/ffmpeg/
- macOS: https://github.com/eugeneware/ffmpeg-static
- Windows: https://github.com/BtbN/FFmpeg-Builds

These builds commonly contain GPL components. If you distribute a bundle that
includes one of them, you are responsible for complying with the license of
that exact build. In particular, GPL redistribution requires the corresponding
source code (or a valid written offer) and the applicable license notices.

FFmpeg source and build information:

- https://ffmpeg.org/download.html
- https://ffmpeg.org/legal.html
- https://github.com/FFmpeg/FFmpeg

The `LICENSES/FFmpeg-GPLv3.txt` and `LICENSES/FFmpeg-LGPLv2.1.txt` files are
included as license texts, not as a statement that every build uses both
licenses. Check the build you actually redistribute.
