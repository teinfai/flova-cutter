# Flova Cutter

Flova Cutter downloads a YouTube video and cuts it into MP4 files for Flova upload or model references. It runs on your computer. You can preview each finished part in your browser while the rest of the video is being cut.

## Download and first open

Download the file for your computer from [Releases](https://github.com/teinfai/flova-cutter/releases).

- **Mac (Apple silicon or Intel)**. Download `FlovaCutter-mac.zip`, unzip it, and open **Flova Cutter.app**. If macOS blocks the app, open **System Settings → Privacy & Security → Open Anyway**.
- **Windows x64**. Download and open `FlovaCutter.exe`. If SmartScreen appears, choose **More info → Run anyway**.

The app opens a local page in your default browser. The first time you open it, it needs an internet connection to download and check the video tools. Start becomes available when they are ready.

## How to use

1. Choose **Cut for**. For Flova upload, choose **Best quality** or **Fast**. For a reference, choose the whole video or one clip and its start time. Use **Change** to pick a different save folder.
2. Paste a YouTube video link to start. You can also type the link and press **Enter** or **Start**. A timestamp in a pasted reference link fills in the start time.
3. Select a finished part to play it. Use **Open folder** to see the output or **Copy path** to copy its location. **Stop** keeps your finished files.

## Targets

| Cut for | Output |
| --- | --- |
| Flova upload | Parts up to 8 minutes |
| Kling 3.0 Omni reference | Up to 8 seconds |
| Gemini Omni Flash reference | Up to 10 seconds |
| Wan 3.0 reference | Up to 15 seconds |
| MiniMax H3 reference | Up to 15 seconds |
| Seedance 2.0 reference | Up to 16 seconds |
| Seedance 2.5 reference | Up to 30 seconds |
| Kling 4.0 reference | Up to 30 seconds |

Best quality makes H.264 video up to 1080p with AAC audio. Fast keeps the downloaded picture and sound and cuts at points where playback can start. If a Fast part runs longer than eight minutes, run it again with Best quality. Reference clips always use the precise cut.

## Where files go

By default, each video gets a folder named after its title inside **Downloads → Flova Cutter**. Cutting the same video again creates a numbered folder. The result shows the full path. The app remembers your target, upload quality, and save folder.

## Troubleshooting

- **YouTube blocked the download.** If a VPN is on, turn it off and try again.
- **Could not get the video tools.** Check your internet connection and reopen Flova Cutter.
- **Not enough free space.** Free the space shown in the message. The download and the finished parts both need room.
- **Something went wrong.** The last run’s log is at `~/Library/Application Support/FlovaCutter/log.txt` on Mac or `%AppData%\FlovaCutter\log.txt` on Windows.

## Quit

Close the tab. The app quits by itself about 10 minutes later, once no job is running. To quit during a cut, press **Stop** first. Opening the app again while it is running reopens the local page.
