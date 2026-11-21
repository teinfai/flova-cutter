# Flova Cutter

Local web app: paste a YouTube link, get mp4 parts and reference clips sized for Flova. One Go binary serves the
embedded page `web/index.html` on 127.0.0.1 and opens the browser; yt-dlp, ffmpeg, ffprobe and deno are fetched on
first run into the cache folder. The approved v1 plan, with every decision and its source, is
`docs/plans/261121-plan-flova-cutter-v1.md`; read it before changing behaviour.

## House rules

- Go standard library only; `go.mod` has no requires, so `scripts/build.sh` builds offline (`GOPROXY=off`).
- Source carries zero comments. The single allowed directive is `//go:embed web/index.html`. Reasoning lives in `docs/plans/`.
- Verification is builds plus real runs (below); the repo has no unit tests by design.
- New work starts as a plan in `docs/plans/YYMMDD-plan-<name>.md`: one phase, checklist, verified facts with sources, KISS/DRY verdict.
- Tools run through `exec` argument lists; the YouTube URL always goes after `--`.

## Why the numbers are what they are

Limits live in `targets.go`. Their sources are the owner's Flova notes: uploads must stay at 8 min or less (Flova
rejects 9-10 min), references have per-model maximums, and Kling 3.0 Omni needs at least 3 s.

| Path | Rule | Reason |
|---|---|---|
| Flova upload, Best | re-encode, segment 478 s, GOP 2 s, 4 Mbps video + AAC 192k | parts land at exactly 478 s and about 241 MB, under 300 MB |
| Flova upload, Fast | stream copy, segment 470 s, then every part checked `<= 480 s` | copy cuts snap to the next keyframe |
| Reference clips | always re-encode, length = model seconds - 0.1 | AAC padding made "30 s" clips 30.018 s; copy cuts would overshoot |
| Kling 3.0 whole video | a last clip under 3 s is dropped | model minimum |
| Best download | VP9 up to 2160p, else avc1; scale to at most 1080p | ffmpeg's native VP9 decoder is in every build; the pinned macOS build documents no AV1 decoder |
| Encoder | `ffmpeg -encoders` at start: `h264_videotoolbox`, else `libx264` | Mac has VideoToolbox; the Windows gyan build ships libx264 |

## Tools and pins (`tools.go`)

- ffmpeg/ffprobe come from `eugeneware/ffmpeg-static` `b6.1.1`, deno from `v2.9.8`, each checked against a pinned SHA-256.
  To bump a pin: download the new file, hash it with `shasum -a 256`, update the constant, record it in a plan.
- yt-dlp is always the latest release, verified against that release's `SHA2-256SUMS`, and self-updated with `yt-dlp -U`
  on every start, because YouTube changes break old yt-dlp.
- Every yt-dlp call passes `--js-runtimes deno:<path>` and `--ffmpeg-location <bin>`; YouTube needs the JS runtime.

## Security model

Keep all of it when adding routes: listen on `127.0.0.1` only (random port); a 32-byte token in the page URL; every
request checks `Host`, the token (`X-Token` header, or `t` query for `/files/` and `/api/ping`), and `Origin` on POST;
`/files/` and `/api/open` accept only names in the current job's file list.

## Lifecycle and packaging

- One instance: a file lock plus `instance.json` (`port`, `token`); a second launch reopens the browser and exits.
- The app quits 10 min after the last page ping when no job runs. Chrome slows hidden tabs to about one timer per
  minute, so a short limit kills the app while the user is in another tab.
- macOS: the `.app` is ad-hoc signed (`codesign --force -s -`) in `scripts/build.sh`; an unsealed bundle is refused
  after download. `LSUIElement` hides the Dock icon. Not notarized, so users click Open Anyway once.
- Windows: built with `-H=windowsgui`; child tools start hidden (`CREATE_NO_WINDOW`).

## YouTube behaviour seen in testing

- "Sign in to confirm you're not a bot" appears when the user is on a VPN; the app shows the VPN hint (`blocked`).
- A download can fail once with `HTTP Error 403` and succeed right after; the app retries the download up to 3 times.

## Where things are on a user's machine

- Data: `os.UserConfigDir()/FlovaCutter` holds `config.json`, `instance.json`, and `log.txt` (every command line and
  stderr of the last run: read it first when debugging).
- Cache: `os.UserCacheDir()/FlovaCutter` holds `bin/`, `work/`, `thumbs/`; `work/` and `thumbs/` are wiped at start.
- Output: `~/Downloads/Flova Cutter/<title>/` by default, changeable in the page.

## Verify a change

1. `scripts/build.sh` passes (gofmt, vet, two macOS builds joined with `lipo`, codesign check, Windows build).
2. Run `dist/Flova Cutter.app/Contents/MacOS/flova-cutter`, read `port` and `token` from `instance.json`, and drive
   `POST /api/start` + `GET /api/state` with the `X-Token` header.
3. Reference results on an M1 Pro Mac:

| Run | Link | Expected |
|---|---|---|
| Flova upload, Fast | `https://www.youtube.com/watch?v=MYPVQccHhAQ` (3:35:22) | 28 parts, longest <= 480 s, about 1.5 min |
| Seedance 2.5, one clip | same link + `&t=750` | one clip, 29.9 s, starts at 12:30, 1080p |
| Kling 3.0, whole video | `https://www.youtube.com/watch?v=jNQXAC9IVRw` (19 s) | clips of about 7.95 s plus one of about 3.1 s |

Windows has only been cross-built; a change touching `platform_windows.go` needs a run on a real Windows PC.

## Release

Bump `CFBundleShortVersionString` in `packaging/Info.plist`, run `scripts/build.sh`, tag `vX.Y.Z` on `main`, and attach
`dist/FlovaCutter-mac.zip` and `dist/FlovaCutter.exe` to the GitHub Release. Commits use the repo-local GitHub
noreply address already set in `.git/config`.
