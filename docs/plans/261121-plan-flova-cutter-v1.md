# Plan: Flova Cutter v1 (one phase)

Status: approved decisions, ready for Codex
Date: 2026-11-21
Repo: `flova-cutter` (new, public later; nothing is committed or pushed in this phase)

## Standing rules for this plan

- No code comments anywhere (Go, HTML, CSS, JS, shell). Reasoning lives in this plan only.
- No unit tests, no test files. Verification = gofmt, go vet, three cross-builds, and the manual runs in "Verification".
- No guessing: every external fact below was checked on 2026-11-21 (sources listed). If something in the code needs a fact
  that is not in this plan, stop and report it instead of inventing it.
- Go standard library only. No third-party Go modules, no npm, no frameworks.

## Goal

A user double-clicks one file. Their browser opens a local page (Option 1 "Clean" design). They pick what the video is for,
paste a YouTube link, and the app downloads and cuts the video into files Flova accepts, saved in a folder on their computer.
Parts appear in a horizontal row; each part opens a preview dialog that plays it. The page shows the full folder path with
"Open folder" and "Copy path".

## Decisions

| # | Decision | Made by |
|---|---|---|
| D1 | UI = Option 1 "Clean" from the approved mock: white page, text `#0F172A`, accent `#2563EB`, borders `#E2E8F0`, Plus Jakarta Sans, centered column max 880 px, "Cut for" select inside the big link bar, parts row, result block, preview dialog. | Owner |
| D2 | Runs on the user's own computer: a local web server on `127.0.0.1`, UI in the default browser. | Owner |
| D3 | Start by double-clicking one file: macOS `Flova Cutter.app` (shipped zipped), Windows `FlovaCutter.exe`. No console window. | Owner |
| D4 | Quality switch "Best quality / Fast" shown only for the Flova upload target; remembered between runs. | Owner |
| D5 | Reference targets always use the precise (re-encode) path, because a stream-copy cut snaps to keyframes and can exceed the model's maximum length. | Claude |
| D6 | No visible app window: macOS bundle has `LSUIElement=true`; Windows exe built with `-H=windowsgui`. The server quits by itself 30 s after the last page ping when no job is running. Launching again while running only reopens the browser tab. | Claude |
| D7 | Video tools are downloaded on first run into the app's cache folder and checked by SHA-256 before use. Nothing is bundled. | Claude |
| D8 | Builds: macOS universal (arm64 + amd64 via `lipo`), Windows x64 only (the pinned ffmpeg source has no Windows arm64 file). | Claude |
| D9 | MIT license, `LICENSE` file with `Copyright (c) 2026 teinfai`. Added in the review-fix round. | Owner |
| D10 | After review and tests pass: public GitHub repo `teinfai/flova-cutter`, then GitHub Release `v1.0.0` with `FlovaCutter-mac.zip` and `FlovaCutter.exe`; README links to the release. Done by Claude, not Codex. | Owner |

## Verified facts (sources)

| Fact | Source |
|---|---|
| yt-dlp official executables: `yt-dlp_macos` (universal, macOS 10.15+), `yt-dlp.exe` (Windows x64). URL pattern `https://github.com/yt-dlp/yt-dlp/releases/latest/download/<name>`; checksums in `SHA2-256SUMS` of the same release. | yt-dlp README lines 98-99; release 2026.10.26 assets |
| yt-dlp official executables need no remote components for YouTube; a JS runtime is required; `deno` is enabled by default and can be given with `--js-runtimes deno:<path>`. | yt-dlp README "--js-runtimes", "--remote-components", "yt-dlp-ejs" |
| yt-dlp format filter supports `~=` (regex) and `^=` (starts with). | yt-dlp README line 1582 |
| deno v2.9.8 files and SHA-256: `deno-aarch64-apple-darwin.zip` `9f0bf660393da2eaff044cdc7444cb08c73f235849b61d224d825a194ce7c98e`; `deno-x86_64-apple-darwin.zip` `4b2f5edd2639b7470f034238b321d7002d8dfb4726d8558f52c17afac370eb2c`; `deno-x86_64-pc-windows-msvc.zip` `cba84b26e0df5ae7f74fc66b86a95b866872a86292ebd84fa1e959927edd93bf`. URL `https://github.com/denoland/deno/releases/download/v2.9.8/<file>`. | release `.sha256sum` files |
| ffmpeg/ffprobe from `eugeneware/ffmpeg-static` release `b6.1.1`, gzip files, URL `https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/<file>`. SHA-256 of the `.gz` files: `ffmpeg-darwin-arm64.gz` `8923876afa8db5585022d7860ec7e589af192f441c56793971276d450ed3bbfa`; `ffprobe-darwin-arm64.gz` `d986a8ec7b030899fe66a8a288ed809a3543338705a3ce178cfb85869c5d80be`; `ffmpeg-darwin-x64.gz` `929b375c1182d956c51f7ac25e0b2b0411fb01f6f407aa15c9758efeb4242106`; `ffprobe-darwin-x64.gz` `d4da574d6e2e197bd259b47d69cf262df9e312af24ad960444f6d806d3d4c186`; `ffmpeg-win32-x64.gz` `8883a3dffbd0a16cf4ef95206ea05283f78908dbfb118f73c83f4951dcc06d77`; `ffprobe-win32-x64.gz` `f309e6223ad89d2fe54bccd420a7709b66fd27540674e92309578ed491a43c8d`. | hashed after download on 2026-11-21 |
| The Windows ffmpeg build is gyan.dev 6.1.1 essentials and includes `libx264`. The macOS builds' encoder list is not documented, so the encoder is detected at runtime (see Encoding). | `win32-x64.README`, `darwin-x64.README` |
| Proven on this Mac today: 4K source -> 1080p H.264 4 Mbps + AAC 192k, segment 478 s with GOP 2 s -> every part 478.0 s and 241 MB. YouTube 1080p H.264 + m4a stream-copied with segment 470 s -> longest part 476 s. | manual runs 2026-11-21 |
| Flova limits (owner's notes): upload length max 9-10 min, keep parts <= 8 min; reference max: Kling 3.0 Omni 8 s, Gemini Omni Flash 10 s, Wan 3.0 15 s, MiniMax H3 15 s, Seedance 2.0 16 s, Seedance 2.5 30 s, Kling 4.0 30 s; mp4 or mov; 50-300 MB depending on model. | owner message |

## Files (fewest possible)

```
flova-cutter/
  go.mod                      module github.com/teinfai/flova-cutter, go 1.27
  main.go                     start-up, single instance, HTTP server, routes, auto-quit, open browser
  job.go                      one job at a time: probe, download, cut, thumbnails, progress, stop, errors
  tools.go                    first-run download + SHA-256 check + unpack, yt-dlp self-update
  targets.go                  the 8 targets table
  platform_darwin.go          open URL/folder, reveal file, folder picker, free disk space, kill process group
  platform_windows.go         same five functions for Windows
  web/index.html              the whole UI (HTML + CSS + JS in one file), embedded with `embed`
  packaging/Info.plist        macOS bundle plist
  scripts/build.sh            builds dist/Flova Cutter.app (+ zip) and dist/FlovaCutter.exe
  README.md                   what it does, download, first open on macOS/Windows, use, limits, troubleshooting
  .gitignore                  dist/, .cache/
  docs/plans/                 this plan
```

## Checklist

### A. Project and start-up (main.go)

- [ ] `go.mod` with module `github.com/teinfai/flova-cutter`, `go 1.27`, no requires.
- [ ] Data folder = `os.UserConfigDir()/FlovaCutter` (config + lock), cache folder = `os.UserCacheDir()/FlovaCutter` (tools + thumbnails).
- [ ] Single instance: lock file `instance.json` holds `{port, token}`. On launch, if it exists and `GET http://127.0.0.1:<port>/api/ping?t=<token>` answers 200 within 1 s, open the browser to that URL and exit. Otherwise start a new server and rewrite the file.
- [ ] Listen on `127.0.0.1:0` (random free port). Never listen on any other interface.
- [ ] Token: 32 random bytes from `crypto/rand`, hex. Page URL `http://127.0.0.1:<port>/?t=<token>`.
- [ ] Every request except `GET /` must carry the token (`X-Token` header, or `t` query for `/files/` and `/api/ping`). Every request must have `Host` equal to `127.0.0.1:<port>`; otherwise 403. `POST` requests with an `Origin` header that is not `http://127.0.0.1:<port>` get 403.
- [ ] `GET /` without the right `t` returns 403 with a one-line plain page "Open Flova Cutter from its app icon."
- [ ] Open the browser after the server listens: macOS `open <url>`, Windows `rundll32 url.dll,FileProtocolHandler <url>`.
- [ ] Auto-quit: the page calls `POST /api/ping` every 5 s; a ticker stops the server and removes `instance.json` when the last ping is older than 30 s and no job is running.
- [ ] Config `config.json` in the data folder: `target`, `quality`, `outputDir`. Default `outputDir` = `<home>/Downloads/Flova Cutter`. Written atomically (temp file + rename).

### B. Routes

| Method + path | Does |
|---|---|
| `GET /` | serves embedded `web/index.html` |
| `GET /api/state` | `{tools:{ready,percent,error}, settings:{target,quality,outputDir}, os:"mac"|"windows", job:<job or null>, targets:[...]}` |
| `POST /api/start` | body `{url, target, mode:"whole"|"one", start:"mm:ss"|"h:mm:ss", quality:"best"|"fast"}`; 409 if a job runs; validates (section C) then starts the job |
| `POST /api/stop` | stops the running job |
| `POST /api/settings` | saves `target`, `quality` |
| `POST /api/choose-folder` | opens the native folder picker; on a pick saves `outputDir` and returns it; on cancel returns the old one |
| `POST /api/open` | body `{file?}`; no file = open the job folder; with file = reveal that file; file must be a direct child of the job folder |
| `GET /files/<name>` | serves a part or its thumbnail from the job folder / thumbnail folder with `http.ServeContent` (Range support for video seeking); name must be a plain file name with no separators or `..` |
| `POST /api/ping`, `GET /api/ping` | 204 / 200, updates last-ping time |

### C. Input checks (trust boundary)

- [ ] URL: parse with `net/url`; scheme `http` or `https`; host one of `youtube.com`, `www.youtube.com`, `m.youtube.com`, `music.youtube.com`, `youtu.be`. Otherwise error code `not_link`.
- [ ] Start time from the `t` query param when present (`750`, `750s`, `12m30s`, `1h2m3s`) or from the body; must be `>= 0` and `start + clip length <= duration`, else error `start_past_end` with the video length.
- [ ] Target must be one of the 8 ids; mode only for reference targets; quality only for `flova`.
- [ ] All tools run with `exec.Command` and an argument list, never a shell. The URL is passed after `--`.

### D. Targets (targets.go)

| id | Label in UI | Kind | Seconds |
|---|---|---|---|
| `flova` | Flova upload · parts up to 8 min | upload | 478 |
| `kling3` | Kling 3.0 Omni reference · up to 8 s | ref | 8 |
| `gemini` | Gemini Omni Flash reference · up to 10 s | ref | 10 |
| `wan3` | Wan 3.0 reference · up to 15 s | ref | 15 |
| `minimax` | MiniMax H3 reference · up to 15 s | ref | 15 |
| `seedance2` | Seedance 2.0 reference · up to 16 s | ref | 16 |
| `seedance25` | Seedance 2.5 reference · up to 30 s | ref | 30 |
| `kling4` | Kling 4.0 reference · up to 30 s | ref | 30 |

### E. Tools (tools.go)

- [ ] Tool folder `<cache>/bin`. Files: `yt-dlp`, `ffmpeg`, `ffprobe`, `deno` (`.exe` on Windows). Mark executable on macOS.
- [ ] macOS arch from `runtime.GOARCH`: arm64 -> `darwin-arm64` / `aarch64-apple-darwin`; amd64 -> `darwin-x64` / `x86_64-apple-darwin`. Windows -> `win32-x64` / `x86_64-pc-windows-msvc` / `yt-dlp.exe`.
- [ ] Download to a temp file in the same folder, check SHA-256 (pinned values above for deno and ffmpeg/ffprobe `.gz`; for yt-dlp read the matching line of `SHA2-256SUMS` from the same release), then gunzip (ffmpeg/ffprobe) or unzip (deno, take the single `deno`/`deno.exe` entry), then rename into place. Any mismatch: delete the temp file, `tools.error` = "Could not get the video tools. Check your internet and reopen Flova Cutter."
- [ ] Skip a tool that already exists. Report overall percent by bytes across all missing files.
- [ ] After tools are ready, run `yt-dlp -U` once in the background per app start; ignore its result.
- [ ] Every yt-dlp call adds `--js-runtimes deno:<path to deno>` and `--ffmpeg-location <bin folder>`.
- [ ] Encoder detection once after tools are ready: run `ffmpeg -hide_banner -encoders`; use `h264_videotoolbox` if listed, else `libx264`, else `tools.error` = "This computer's video tool cannot make H.264 files."

### F. Job (job.go)

- [ ] Probe: `yt-dlp -J --no-playlist -- <url>` -> title, duration, and format sizes (`filesize` or `filesize_approx`) of the chosen formats.
- [ ] Output folder: `<outputDir>/<safe title>/`; safe title = title with `\/:*?"<>|` and control chars removed, trimmed, max 80 chars; if the folder exists add ` (2)`, ` (3)`...
- [ ] Free space check before downloading: needed = download size + output size (output = 4.2 Mbit/s x duration for re-encode paths; = download size for Fast); if free space is smaller, error `no_space` with "about X GB".
- [ ] Download working file into `<cache>/work/<job id>/` and delete that folder when the job ends (done, stopped or failed).
- [ ] Paths:

| Case | Download format (`-f`) | Then |
|---|---|---|
| `flova` + `fast` | `bv*[vcodec^=avc1][height<=1080]+ba[ext=m4a]/b[ext=mp4]`, `--merge-output-format mp4` | `ffmpeg -i src -map 0:v:0 -map 0:a:0 -c copy -f segment -segment_time 470 -reset_timestamps 1 -segment_format_options movflags=+faststart part-%02d.mp4`; then ffprobe each part; any part > 480 s -> error `part_too_long` "A part came out longer than 8 minutes. Use Best quality for this video." |
| `flova` + `best` | `bv*[vcodec~='^vp0?9'][height<=2160]+ba/bv*[vcodec^=avc1]+ba/b`, `--merge-output-format mkv` | `ffmpeg -i src -map 0:v:0 -map 0:a:0 -vf scale=-2:'min(1080,ih)' -pix_fmt yuv420p <enc> -b:v 4M -maxrate 5M -bufsize 8M -g <2 x fps> -c:a aac -b:a 192k -f segment -segment_time 478 -reset_timestamps 1 -segment_format_options movflags=+faststart part-%02d.mp4` |
| ref + `whole` | same as `best` | same encode, plus `-force_key_frames expr:gte(t,n_forced*N)` and `-segment_time N`, files `clip-%03d.mp4` |
| ref + `one` | same format plus `--download-sections *<start-2>-<start+N+2>` (clamped to 0 and duration) | `ffmpeg -ss <offset> -i src -t N` + same encode, one file `clip-001.mp4` |

  `<enc>` = `-c:v h264_videotoolbox -profile:v high` or `-c:v libx264 -preset veryfast -profile:v high`. Use 3-digit numbers when a run can make 100+ files, else 2 digits. After cutting, rename segment files to 1-based (`part-01.mp4`...).
- [ ] Progress: yt-dlp `--newline --progress-template "download:%(progress.downloaded_bytes)s/%(progress.total_bytes_estimate)s"`; ffmpeg `-progress pipe:1 -nostats` (`out_time_us`). Job state: `phase` (`probe`,`download`,`cut`,`done`,`stopped`,`error`), `percent`, `etaSeconds`, `partsDone`, `partsTotal`, `title`, `duration`, `folder`, `files[]` (`name, start, end, seconds, bytes`), `error {code, message}`. Parts appear in `files[]` as soon as the next segment file starts (the previous one is complete).
- [ ] Thumbnails: after each part is complete, `ffmpeg -ss <mid> -i part -frames:v 1 -vf scale=320:-2 -q:v 5 <cache>/thumbs/<job id>/<name>.jpg`.
- [ ] Stop: macOS start every tool with `SysProcAttr.Setpgid=true` and kill the group; Windows `taskkill /T /F /PID <pid>`. Delete the unfinished last file, keep finished ones, phase `stopped`.
- [ ] Error codes and exact UI text:

| code | Message |
|---|---|
| `not_link` | This isn't a YouTube video link. |
| `blocked` (stderr contains `Sign in to confirm you`) | YouTube blocked this download. If a VPN is on, turn it off, then try again. |
| `no_space` | Not enough free space. This video needs about X GB. |
| `start_past_end` | The video is only H:MM:SS long. |
| `part_too_long` | A part came out longer than 8 minutes. Use Best quality for this video. |
| `failed` (anything else; last stderr line goes to the log only) | Something went wrong. Try again. |

- [ ] Log file `<data>/log.txt` (last run only, overwritten at start) with every command line and stderr, so failures can be diagnosed.

### G. UI (web/index.html)

- [ ] Port Option 1 "Clean" from the approved mock exactly (layout, type, colors, parts row with scroll-snap and arrows, result block with path pill, preview dialog). Remove all mock-only parts: Mock controls panel, fake timers, sample-link chip, Option 2/3 styles, OS switch.
- [ ] Data comes from `/api/state` polled every 500 ms while a job runs, every 5 s otherwise.
- [ ] Quality switch (D4): a two-button segmented control "Best quality" / "Fast" with one muted line under it ("Sharper picture, takes longer." / "Quicker, softer picture."), in the row under the link bar on the left of "Saves to…", visible only for `flova`.
- [ ] Paste of a valid link starts the job; Enter or Start also starts. Not-a-link and start-past-end errors show under their field; other errors show in the status line with "Try again".
- [ ] Parts row cards: thumbnail, `Part N`, time range, `length · size`; play icon; click/Enter opens the preview dialog. Preview `<video>` src `/files/<name>?t=<token>`.
- [ ] Result block: "<N> parts ready for Flova upload" or "1 clip ready for <model> · <N> s from <mm:ss> · <size>", facts line (`1080p · mp4 · each under 8 min · <total>`; for Fast say the real height from ffprobe), full path pill (middle-truncated, full path in `title`), "Open folder", "Copy path" (`navigator.clipboard.writeText`, toast "Path copied").
- [ ] Dialog: native `<dialog>`, Esc/X/backdrop close, Prev/Next + arrow keys, "Show in folder" (`/api/open {file}`), "Copy path" for that file, pause and reset video on close.
- [ ] First run: slim line "Getting video tools ready. This happens once." with its bar from `tools.percent`; Start disabled until ready.
- [ ] "Change" calls `/api/choose-folder`.
- [ ] UX checks: contrast >= 4.5:1, visible focus, keyboard use, buttons >= 44 px, `aria-live` for phase changes, SVG icons only, `prefers-reduced-motion`, no page sideways scroll at 390/1280/1440 px.

### H. Platform files

- [ ] `platform_darwin.go`: `openURL` (`open`), `openFolder` (`open <dir>`), `revealFile` (`open -R <file>`), `chooseFolder` (`osascript -e 'POSIX path of (choose folder with prompt "Choose where to save")'`, cancel = exit code 1), `freeBytes` (`syscall.Statfs`: `Bavail * Bsize`), `killTree` (`syscall.Kill(-pid, SIGKILL)`), `procAttr` (Setpgid).
- [ ] `platform_windows.go`: `openURL` (`rundll32 url.dll,FileProtocolHandler`), `openFolder` (`explorer <dir>`), `revealFile` (`explorer /select,<file>`), `chooseFolder` (`powershell -NoProfile -STA -Command` with `System.Windows.Forms.FolderBrowserDialog`, print `SelectedPath` only on OK), `freeBytes` (`kernel32.GetDiskFreeSpaceExW` via `syscall.NewLazyDLL`), `killTree` (`taskkill /T /F /PID`), `procAttr` (`HideWindow: true`, `CreationFlags: 0x08000000` so no console windows flash).

### I. Packaging (scripts/build.sh, packaging/Info.plist)

- [ ] `Info.plist`: `CFBundleName` "Flova Cutter", `CFBundleIdentifier` `app.flovacutter`, `CFBundleExecutable` `flova-cutter`, `CFBundlePackageType` `APPL`, `CFBundleShortVersionString` `1.0.0`, `LSUIElement` true, `LSMinimumSystemVersion` `11.0`.
- [ ] `build.sh` (bash, `set -euo pipefail`): `gofmt -l .` must print nothing; `go vet ./...`; build darwin/arm64 and darwin/amd64 with `-trimpath -ldflags "-s -w"`, `lipo -create` into `dist/Flova Cutter.app/Contents/MacOS/flova-cutter`, copy Info.plist, `ditto -c -k --keepParent` to `dist/FlovaCutter-mac.zip`; build windows/amd64 with `-ldflags "-s -w -H=windowsgui"` to `dist/FlovaCutter.exe`. Use `GOCACHE=$PWD/.cache/go-build` so it works in sandboxes.

### J. README.md

- [ ] What it does (one paragraph), download (Releases: `FlovaCutter-mac.zip`, `FlovaCutter.exe`), first open on macOS (unsigned: System Settings > Privacy & Security > Open Anyway) and Windows (SmartScreen: More info > Run anyway), how to use (3 steps), the targets table, where files go, troubleshooting (YouTube blocked -> turn VPN off; first run needs internet; log file location), how to quit (close the tab; it quits by itself).

## Verification (no unit tests)

Codex (offline, in its sandbox):
- [ ] `gofmt -l .` prints nothing; `go vet ./...` clean; `scripts/build.sh` produces all three outputs.
- [ ] `grep` shows no `//`, `/*`, `<!--` comments in Go/HTML/JS/CSS/shell. Allowed exceptions, because they are compiler/OS directives and not comments: URLs, the `#!/usr/bin/env bash` line, and exactly one `//go:embed web/index.html` line in `main.go`.

Claude (on the owner's Mac, after Codex):
- [ ] Review every changed file line by line against this plan (security checks in A and C first).
- [ ] Start the app from `dist/Flova Cutter.app`: first-run tool download, checksums pass, encoder detected.
- [ ] Fast + Flova upload on `https://www.youtube.com/watch?v=MYPVQccHhAQ`: 28 parts, each <= 480 s.
- [ ] Best + Flova upload on a short public video: parts H.264 1080p (or source height if smaller), mp4.
- [ ] Seedance 2.5 one clip with `&t=750`: one file, 30.0 s, starts at 12:30.
- [ ] Kling 3.0 whole-video on a short video: every clip 8.0 s except the last.
- [ ] Stop mid-cut: finished parts stay, unfinished one removed, work folder removed.
- [ ] Not-a-link, start-past-end, preview dialog plays and seeks, Open folder, Show in folder, Copy path, Change folder.
- [ ] Security: request without token -> 403; request with `Host: localhost:<port>` -> 403; `/files/../x` -> 403.
- [ ] Close the tab, app quits within ~35 s; double-click while running reopens the tab.
- [ ] Windows: cross-build only here. Real Windows run must be done by the owner or a tester (open risk R1).

## Risks

| # | Risk | Handling |
|---|---|---|
| R1 | Windows build is not run on a real Windows PC in this phase. | Owner/tester runs the exe; Claude fixes what they report. |
| R2 | macOS ffmpeg-static encoder list is unknown until run. | Runtime detection (E); verified on this Mac during testing; Intel Mac untested. |
| R3 | YouTube changes break yt-dlp. | `yt-dlp -U` on each start; latest yt-dlp on first run. |
| R4 | Unsigned app warnings scare users. | README steps; signing is a later decision. |
| R5 | Best quality on long 4K videos is slow (about 70 min for 3.5 h on an M1 Pro; slower on PCs with `libx264`). | Owner chose a per-run switch; ETA is shown. |

## Review round 1 (Claude, 2026-11-21) — fixes for Codex

Evidence comes from Claude's code review and real runs on the owner's M1 Pro Mac with `dist/Flova Cutter.app`.

- [ ] R1-1 macOS signing. `codesign -dv` shows the bundle is not sealed and `spctl` says "no usable signature"; quarantined downloads of such bundles are refused. In `scripts/build.sh`, after copying `Info.plist` and before zipping, run `codesign --force -s - "$app"` then `codesign --verify --strict "$app"`. (Checked: an ad-hoc signed copy is "valid on disk" and gets the normal Gatekeeper path.)
- [ ] R1-2 Reference length over the limit. Measured: Seedance 2.5 clip 30.018 s, Kling 3.0 clips 8.022 s / 8.007 s (AAC padding). For reference targets use `clip = seconds - 0.1` everywhere the clip length is used (`-t`, `-segment_time`, `-force_key_frames`). After each part is probed, fail with the existing `part_too_long`-style error if `duration > 480` (upload) or `duration > seconds` (reference); one shared check replaces the Fast-only one. Reference error text: "A clip came out longer than the model allows. Try again."
- [ ] R1-3 Kling 3.0 minimum. The owner's notes say Kling 3.0 Omni takes 3 to 8 s. Add `min` to the target table (`kling3` = 3, all others 0). In whole-video mode, a last clip shorter than `min` is deleted and not listed.
- [ ] R1-4 Random YouTube 403. Seen once: the Fast download failed with `ERROR: unable to download video data: HTTP Error 403: Forbidden`; the same command by hand right after succeeded. When the download step fails, the job is not cancelled, and stderr contains `HTTP Error 403`, empty the work folder and run the download again, at most 3 tries in total, phase stays `download`. Log each retry.
- [ ] R1-5 Background tabs. Chrome slows timers in tabs hidden for over 5 minutes to about once a minute, so a 30 s ping limit can close the app while the user is in another tab. Change the idle limit to 10 minutes. The page also pings at once on load and when `visibilitychange` makes it visible. When a request fails because the app cannot be reached (network error, not an HTTP status), the status line says "Flova Cutter has closed. Open it again from its app icon." README Quit section: "Close the tab. The app quits by itself about 10 minutes later, once no job is running."
- [ ] R1-6 Free-space estimate. One-clip mode currently requires space for the whole video. Use `download size x (to - from) / duration` for one-clip mode. If the download size is unknown (`<= 0`), skip the space check instead of failing the job.
- [ ] R1-7 Bad settings file. An invalid `config.json` now stops the app from starting. Log it and use the defaults instead.
- [ ] R1-8 Leftovers. At start-up, after the instance lock is taken, delete `<cache>/work` and `<cache>/thumbs` (left by crashes or earlier runs). Add `-update 1` to the thumbnail ffmpeg command (ffmpeg warns without it).
- [ ] R1-9 Add `LICENSE` with the standard MIT text and the line `Copyright (c) 2026 teinfai`.

Verification for this round: the Codex part of "Verification" again (gofmt, go vet, `scripts/build.sh`, comment grep), plus `codesign --verify --strict "dist/Flova Cutter.app"` passes.

## KISS / DRY verdict

- Simplest working shape: one Go binary with the page embedded, standard library only, external tools downloaded once. No database, no framework, no background service, no tray icon.
- Reused, not rebuilt: yt-dlp does all YouTube work; ffmpeg's segment muxer does all cutting in one pass; the approved mock's HTML/CSS is ported instead of redesigned; `http.ServeContent` gives video seeking for free.
- One encode function builds the ffmpeg argument list for all three re-encode cases (best parts, whole-video clips, one clip); only segment length, keyframe forcing and `-ss/-t` differ.
- Rejected as over-built: SSE/websockets (polling is enough for one local user), a desktop window toolkit, bundling tools inside the app (bigger download, licensing), automatic hardware-encoder probing beyond the one `-encoders` check, a per-part re-cut fallback for Fast mode (a clear error is enough).
- Accepted duplication: `platform_darwin.go` and `platform_windows.go` have the same five functions on purpose; that is Go's normal per-OS file pattern.
