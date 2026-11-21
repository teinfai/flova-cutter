package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type jobError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *jobError) Error() string { return e.Message }

var failed = &jobError{"failed", "Something went wrong. Try again."}
var notLink = &jobError{"not_link", "This isn't a YouTube video link."}
var blocked = &jobError{"blocked", "YouTube blocked this download. If a VPN is on, turn it off, then try again."}
var tooLong = &jobError{"part_too_long", "A part came out longer than 8 minutes. Use Best quality for this video."}
var clipTooLong = &jobError{"part_too_long", "A clip came out longer than the model allows. Try again."}

type partFile struct {
	Name    string  `json:"name"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Seconds float64 `json:"seconds"`
	Bytes   int64   `json:"bytes"`
}

type job struct {
	ID         string     `json:"id"`
	Target     string     `json:"target"`
	URL        string     `json:"url"`
	StartText  string     `json:"start"`
	Mode       string     `json:"mode"`
	Quality    string     `json:"quality"`
	Phase      string     `json:"phase"`
	Percent    int        `json:"percent"`
	ETASeconds int        `json:"etaSeconds"`
	PartsDone  int        `json:"partsDone"`
	PartsTotal int        `json:"partsTotal"`
	Title      string     `json:"title"`
	Duration   float64    `json:"duration"`
	Folder     string     `json:"folder"`
	Height     int        `json:"height"`
	Files      []partFile `json:"files"`
	Error      *jobError  `json:"error"`

	ctx       context.Context
	cancel    context.CancelFunc
	outputDir string
	work      string
	thumbs    string
	target    target
	start     float64
}

type startRequest struct{ URL, Target, Mode, Start, Quality string }

var clockPattern = regexp.MustCompile(`^\d+(?::[0-5]\d){1,2}$`)
var unitsPattern = regexp.MustCompile(`^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$`)
var secondsPattern = regexp.MustCompile(`^\d+$`)

func parseStart(text string, query bool) float64 {
	if query && secondsPattern.MatchString(text) {
		n, _ := strconv.ParseFloat(text, 64)
		return n
	}
	if query && text != "" {
		units := unitsPattern.FindStringSubmatch(text)
		if units == nil {
			return -1
		}
		var n float64
		for _, v := range units[1:] {
			val, _ := strconv.ParseFloat(v, 64)
			n = n*60 + val
		}
		return n
	}
	if !clockPattern.MatchString(text) {
		return -1
	}
	var n float64
	for v := range strings.SplitSeq(text, ":") {
		val, _ := strconv.ParseFloat(v, 64)
		n = n*60 + val
	}
	return n
}

func (a *app) start(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running() {
		reply(w, 409, failed)
		return
	}
	if a.closing || !a.tools.Ready {
		reply(w, 503, failed)
		return
	}
	var req startRequest
	if !decode(w, r, &req) {
		return
	}
	u, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		reply(w, 400, notLink)
		return
	}
	switch strings.ToLower(u.Host) {
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com", "youtu.be":
	default:
		reply(w, 400, notLink)
		return
	}
	t, ok := findTarget(req.Target)
	if !ok {
		reply(w, 400, failed)
		return
	}
	var start float64
	if t.Kind == "upload" {
		if !validQuality(req.Quality) || req.Mode != "" || req.Start != "" {
			reply(w, 400, failed)
			return
		}
	} else {
		if req.Quality != "" || (req.Mode != "whole" && req.Mode != "one") || (req.Mode != "one" && req.Start != "") {
			reply(w, 400, failed)
			return
		}
		if req.Mode == "one" {
			if q := u.Query(); q.Has("t") {
				start = parseStart(q.Get("t"), true)
			} else {
				start = parseStart(req.Start, false)
			}
		}
	}
	var id [16]byte
	rand.Read(id[:])
	j := &job{
		ID: hex.EncodeToString(id[:]), Target: t.ID, Mode: req.Mode, Quality: req.Quality,
		Phase: "probe", ETASeconds: -1, Files: []partFile{},
		target: t, URL: u.String(), StartText: req.Start, start: start, outputDir: a.config.OutputDir,
	}
	j.ctx, j.cancel = context.WithCancel(a.ctx)
	j.work = filepath.Join(a.cache, "work", j.ID)
	j.thumbs = filepath.Join(a.cache, "thumbs", j.ID)
	a.job = j
	a.background.Go(func() { a.execute(j) })
	reply(w, 202, j)
}

type writerFunc func([]byte) (int, error)

func (w writerFunc) Write(b []byte) (int, error) { return w(b) }

func lineOutput(line func(string)) io.Writer {
	var pending []byte
	return writerFunc(func(b []byte) (int, error) {
		count := len(b)
		for len(b) > 0 {
			i := bytes.IndexByte(b, '\n')
			if i < 0 {
				pending = append(pending, b...)
				break
			}
			pending = append(pending, b[:i]...)
			line(strings.TrimSpace(string(pending)))
			pending = pending[:0]
			b = b[i+1:]
		}
		return count, nil
	})
}

func matchStderr(marker string, found *bool) io.Writer {
	var tail string
	return writerFunc(func(b []byte) (int, error) {
		s := tail + string(b)
		*found = *found || strings.Contains(s, marker)
		tail = s[max(len(s)-len(marker), 0):]
		return len(b), nil
	})
}

func (a *app) runTool(ctx context.Context, name string, args []string, out io.Writer) error {
	return a.runIn(ctx, "", name, args, out, nil)
}

func (a *app) runIn(ctx context.Context, dir, name string, args []string, out, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, a.bin(name), args...)
	cmd.Dir = dir
	cmd.SysProcAttr = procAttr()
	cmd.Cancel = func() error { return killTree(cmd.Process.Pid) }
	cmd.WaitDelay = 2 * time.Second
	cmd.Stdout = out
	var denied bool
	cmd.Stderr = io.MultiWriter(log.Writer(), matchStderr("Sign in to confirm you", &denied))
	if stderr != nil {
		cmd.Stderr = io.MultiWriter(cmd.Stderr, stderr)
	}
	log.Printf("%q", cmd.Args)
	err := cmd.Run()
	if err != nil && denied {
		return blocked
	}
	return err
}

func (a *app) execute(j *job) {
	err := a.process(j)
	if cleanup := os.RemoveAll(j.work); cleanup != nil {
		log.Print(cleanup)
		if err == nil {
			err = cleanup
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case j.ctx.Err() != nil:
		j.Phase = "stopped"
	case err != nil:
		log.Print(err)
		j.Phase = "error"
		if !errors.As(err, &j.Error) {
			j.Error = failed
		}
	default:
		j.Phase, j.Percent, j.PartsTotal = "done", 100, j.PartsDone
	}
	j.ETASeconds = 0
	j.cancel()
	j.cancel = nil
}

type formatSize struct {
	Size   int64 `json:"filesize"`
	Approx int64 `json:"filesize_approx"`
}

func (f formatSize) bytes() int64 {
	if f.Size > 0 {
		return f.Size
	}
	return f.Approx
}

type videoInfo struct {
	formatSize
	Title    string       `json:"title"`
	Duration float64      `json:"duration"`
	Formats  []formatSize `json:"requested_formats"`
}

func (v videoInfo) downloadBytes() int64 {
	if n := v.bytes(); n > 0 {
		return n
	}
	var total int64
	for _, f := range v.Formats {
		if f.bytes() <= 0 {
			return 0
		}
		total += f.bytes()
	}
	return total
}

func createOutput(parent, title string) (string, error) {
	if err := os.MkdirAll(parent, 0755); err != nil {
		return "", err
	}
	name := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`\/:*?"<>|`, r) {
			return -1
		}
		return r
	}, title)
	runes := []rune(strings.TrimSpace(name))
	name = strings.TrimRight(strings.TrimSpace(string(runes[:min(80, len(runes))])), ". ")
	if name == "" {
		name = "Video"
	}
	if !filepath.IsLocal(name) {
		name = "_" + name
	}
	for n := 1; ; n++ {
		base := name
		if n > 1 {
			base = fmt.Sprintf("%s (%d)", name, n)
		}
		path := filepath.Join(parent, base)
		err := os.Mkdir(path, 0755)
		if !errors.Is(err, os.ErrExist) {
			return path, err
		}
	}
}

func (a *app) progress(j *job, done, total float64, since time.Time) {
	if total <= 0 || done < 0 {
		return
	}
	pct := int(min(100.0, 100*done/total))
	a.mu.Lock()
	defer a.mu.Unlock()
	j.Percent = max(pct, j.Percent)
	if done > 0 {
		j.ETASeconds = int(math.Ceil(max(0.0, total-done) * time.Since(since).Seconds() / done))
	}
}

func (a *app) process(j *job) error {
	fast := j.Quality == "fast"
	format := "bv*[vcodec~='^vp0?9'][height<=2160]+ba/bv*[vcodec^=avc1]+ba/b"
	container := "mkv"
	if fast {
		format = "bv*[vcodec^=avc1][height<=1080]+ba[ext=m4a]/b[ext=mp4]"
		container = "mp4"
	}
	base := a.ytArgs("--ignore-config", "--no-playlist", "-f", format, "--merge-output-format", container)
	var buf bytes.Buffer
	args := append(append([]string{}, base...), "-J", "--", j.URL)
	if err := a.runTool(j.ctx, "yt-dlp", args, &buf); err != nil {
		return err
	}
	var info videoInfo
	if err := json.Unmarshal(buf.Bytes(), &info); err != nil {
		return err
	}
	if info.Duration <= 0 {
		return errors.New("missing video duration")
	}
	clipLength := float64(j.target.Seconds)
	if j.target.Kind == "ref" {
		clipLength -= 0.1
	} else if fast {
		clipLength = 470
	}
	single := j.Mode == "one"
	a.mu.Lock()
	j.Title, j.Duration = info.Title, info.Duration
	a.mu.Unlock()
	if single && (j.start < 0 || j.start+clipLength > info.Duration) {
		return &jobError{"start_past_end", "The video is only " + clockTime(info.Duration) + " long."}
	}
	if err := j.ctx.Err(); err != nil {
		return err
	}
	length := info.Duration
	if single {
		length = clipLength
	}
	total := int(math.Ceil(length / clipLength))
	downloadSize := info.downloadBytes()
	from, to := 0.0, info.Duration
	if single {
		from = max(0.0, j.start-2)
		to = min(info.Duration, j.start+clipLength+2)
	}
	if err := os.MkdirAll(j.work, 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(j.outputDir, 0755); err != nil {
		return err
	}
	if downloadSize > 0 {
		workingSize := uint64(downloadSize)
		if single {
			workingSize = uint64(math.Ceil(float64(downloadSize) * (to - from) / info.Duration))
		}
		outSize := uint64(math.Ceil(4_200_000 * length / 8))
		if fast {
			outSize = workingSize
		}
		needed := workingSize + outSize
		free, err := freeBytes(j.outputDir)
		if err != nil {
			return err
		}
		cacheFree, err := freeBytes(j.work)
		if err != nil {
			return err
		}
		if free < needed || cacheFree < workingSize {
			return &jobError{"no_space", fmt.Sprintf("Not enough free space. This video needs about %.1f GB.", math.Ceil(float64(needed)/1e8)/10)}
		}
	}
	folder, err := createOutput(j.outputDir, info.Title)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(j.thumbs, 0700); err != nil {
		return err
	}
	a.mu.Lock()
	j.Folder, j.Phase, j.PartsTotal = folder, "download", total
	a.mu.Unlock()
	args = append(append([]string{}, base...), "--newline", "--progress", "--progress-template", "download:%(progress.downloaded_bytes)s/%(progress.total_bytes_estimate)s", "--print", "after_move:filepath", "--no-simulate", "-o", filepath.Join(j.work, "source.%(ext)s"))
	if single {
		args = append(args, "--download-sections", "*"+decimal(from)+"-"+decimal(to))
	}
	args = append(args, "--", j.URL)
	source, err := a.download(j, args, downloadSize)
	if err != nil {
		return err
	}
	meta, err := a.probeFile(j.ctx, source)
	if err != nil {
		return err
	}
	a.mu.Lock()
	j.Phase, j.Percent, j.ETASeconds = "cut", 0, -1
	a.mu.Unlock()
	return a.cut(j, source, meta, clipLength, length, j.start-from)
}

func (a *app) download(j *job, args []string, downloadSize int64) (string, error) {
	for attempt := 1; ; attempt++ {
		started := time.Now()
		var previous, finished int64
		var source string
		var streamMu sync.Mutex
		onLine := func(line string) {
			streamMu.Lock()
			defer streamMu.Unlock()
			if filepath.Dir(line) == j.work && strings.HasPrefix(filepath.Base(line), "source.") {
				source = line
				return
			}
			s, estimate, ok := strings.Cut(line, "/")
			if !ok {
				return
			}
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil || n < 0 {
				return
			}
			if n < previous {
				finished += previous
			}
			previous = n
			size := downloadSize
			if est, err := strconv.ParseInt(estimate, 10, 64); err == nil {
				size = max(size, finished+est)
			}
			a.progress(j, float64(finished+n), float64(size), started)
		}
		var forbidden bool
		stderr := io.MultiWriter(lineOutput(onLine), matchStderr("HTTP Error 403", &forbidden))
		err := a.runIn(j.ctx, "", "yt-dlp", args, lineOutput(onLine), stderr)
		if err == nil {
			if source == "" {
				return "", errors.New("missing downloaded file path")
			}
			return source, nil
		}
		if attempt == 3 || !forbidden || j.ctx.Err() != nil {
			return "", err
		}
		if err := os.RemoveAll(j.work); err != nil {
			return "", err
		}
		if err := os.MkdirAll(j.work, 0700); err != nil {
			return "", err
		}
		log.Printf("retrying download after HTTP Error 403 (attempt %d of 3)", attempt+1)
		a.mu.Lock()
		j.Percent, j.ETASeconds = 0, -1
		a.mu.Unlock()
	}
}

func decimal(n float64) string { return strconv.FormatFloat(n, 'f', -1, 64) }

type mediaInfo struct {
	duration, fps float64
	height        int
}

func (a *app) probeFile(ctx context.Context, path string) (mediaInfo, error) {
	var output bytes.Buffer
	err := a.runTool(ctx, "ffprobe", []string{"-v", "error", "-select_streams", "v:0", "-show_entries", "format=duration:stream=height,avg_frame_rate,r_frame_rate", "-of", "json", path}, &output)
	if err != nil {
		return mediaInfo{}, err
	}
	var probe struct {
		Format  struct{ Duration string }
		Streams []struct {
			Height  int
			Average string `json:"avg_frame_rate"`
			Rate    string `json:"r_frame_rate"`
		}
	}
	if err := json.Unmarshal(output.Bytes(), &probe); err != nil {
		return mediaInfo{}, err
	}
	duration, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil || duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) || len(probe.Streams) == 0 {
		return mediaInfo{}, errors.New("missing video stream or duration")
	}
	stream := probe.Streams[0]
	var fps float64
	for _, rate := range []string{stream.Average, stream.Rate} {
		n, d, _ := strings.Cut(rate, "/")
		num, _ := strconv.ParseFloat(n, 64)
		den, _ := strconv.ParseFloat(d, 64)
		if den > 0 && num > 0 {
			fps = num / den
			break
		}
	}
	return mediaInfo{duration, fps, stream.Height}, nil
}

func (a *app) encodeArgs(fps float64) []string {
	args := []string{"-vf", "scale=-2:'min(1080,ih)'", "-pix_fmt", "yuv420p", "-c:v", a.encoder}
	if a.encoder == "libx264" {
		args = append(args, "-preset", "veryfast")
	}
	return append(args, "-profile:v", "high", "-b:v", "4M", "-maxrate", "5M", "-bufsize", "8M", "-g", strconv.Itoa(max(1, int(math.Round(2*fps)))), "-c:a", "aac", "-b:a", "192k")
}

func (a *app) cut(j *job, source string, meta mediaInfo, clipLength, length, offset float64) error {
	single := j.Mode == "one"
	prefix := "part"
	maxDuration := 480.0
	durationError := tooLong
	if j.target.Kind == "ref" {
		prefix = "clip"
		maxDuration = float64(j.target.Seconds)
		durationError = clipTooLong
	}
	width := 2
	if j.PartsTotal >= 100 || single {
		width = 3
	}
	pattern := fmt.Sprintf("%s-%%0%dd.mp4", prefix, width)
	stage := func(index int) string { return filepath.Join(j.Folder, ".cut-"+fmt.Sprintf(pattern, index)) }
	args := []string{"-hide_banner", "-nostdin", "-progress", "pipe:1", "-nostats"}
	if single {
		args = append(args, "-ss", decimal(offset))
	}
	args = append(args, "-i", source)
	if single {
		args = append(args, "-t", decimal(clipLength))
	}
	args = append(args, "-map", "0:v:0", "-map", "0:a:0")
	if j.Quality == "fast" {
		args = append(args, "-c", "copy")
	} else {
		if meta.fps <= 0 {
			return errors.New("missing frame rate")
		}
		args = append(args, a.encodeArgs(meta.fps)...)
	}
	if single {
		args = append(args, "-movflags", "+faststart", filepath.Base(stage(0)))
	} else {
		if j.target.Kind == "ref" {
			args = append(args, "-force_key_frames", "expr:gte(t,n_forced*"+decimal(clipLength)+")")
		}
		args = append(args, "-f", "segment", "-segment_time", decimal(clipLength), "-reset_timestamps", "1", "-segment_format_options", "movflags=+faststart", ".cut-"+pattern)
	}
	ctx, cancel := context.WithCancel(j.ctx)
	defer cancel()
	done := make(chan error, 1)
	started := time.Now()
	go func() {
		done <- a.runIn(ctx, j.Folder, "ffmpeg", args, lineOutput(func(line string) {
			if raw, ok := strings.CutPrefix(line, "out_time_us="); ok {
				n, err := strconv.ParseInt(raw, 10, 64)
				if err == nil {
					a.progress(j, float64(n)/1e6, length, started)
				}
			}
		}), nil)
	}()
	index := 0
	timeline := j.start
	publish := func(last bool) error {
		path := stage(index)
		part, err := a.probeFile(a.ctx, path)
		if err != nil {
			return err
		}
		if part.duration > maxDuration {
			return durationError
		}
		if last && j.Mode == "whole" && part.duration < float64(j.target.Min) {
			return os.Remove(path)
		}
		stat, err := os.Stat(path)
		if err != nil {
			return err
		}
		name := fmt.Sprintf(pattern, index+1)
		thumb := filepath.Join(j.thumbs, name+".jpg")
		err = a.runTool(a.ctx, "ffmpeg", []string{"-hide_banner", "-nostdin", "-ss", decimal(part.duration / 2), "-i", path, "-frames:v", "1", "-vf", "scale=320:-2", "-q:v", "5", "-update", "1", thumb}, io.Discard)
		if err != nil {
			return err
		}
		if err := os.Rename(path, filepath.Join(j.Folder, name)); err != nil {
			return err
		}
		end := timeline + part.duration
		a.mu.Lock()
		j.Files = append(j.Files, partFile{name, timeline, end, part.duration, stat.Size()})
		j.PartsDone = len(j.Files)
		j.PartsTotal = max(j.PartsDone, j.PartsTotal)
		j.Height = part.height
		a.mu.Unlock()
		index++
		timeline = end
		return nil
	}
	collect := func() error {
		for {
			_, err := os.Stat(stage(index + 1))
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			if err := publish(false); err != nil {
				return err
			}
		}
	}
	defer func() {
		for n := index; ; n++ {
			err := os.Remove(stage(n))
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			if err != nil {
				log.Print(err)
				break
			}
		}
	}()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			if collectErr := collect(); collectErr != nil {
				return collectErr
			}
			if err != nil {
				return err
			}
			return publish(true)
		case <-ticker.C:
			if err := collect(); err != nil {
				cancel()
				<-done
				return err
			}
		}
	}
}
