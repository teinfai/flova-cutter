package main

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type toolState struct {
	Ready   bool   `json:"ready"`
	Percent int    `json:"percent"`
	Error   string `json:"error"`
}

type distribution struct {
	ffArch, denoArch, ffHash, probeHash, denoHash string
}

var distributions = map[string]distribution{
	"darwin/arm64":  {"darwin-arm64", "aarch64-apple-darwin", "8923876afa8db5585022d7860ec7e589af192f441c56793971276d450ed3bbfa", "d986a8ec7b030899fe66a8a288ed809a3543338705a3ce178cfb85869c5d80be", "9f0bf660393da2eaff044cdc7444cb08c73f235849b61d224d825a194ce7c98e"},
	"darwin/amd64":  {"darwin-x64", "x86_64-apple-darwin", "929b375c1182d956c51f7ac25e0b2b0411fb01f6f407aa15c9758efeb4242106", "d4da574d6e2e197bd259b47d69cf262df9e312af24ad960444f6d806d3d4c186", "4b2f5edd2639b7470f034238b321d7002d8dfb4726d8558f52c17afac370eb2c"},
	"windows/amd64": {"win32-x64", "x86_64-pc-windows-msvc", "8883a3dffbd0a16cf4ef95206ea05283f78908dbfb118f73c83f4951dcc06d77", "f309e6223ad89d2fe54bccd420a7709b66fd27540674e92309578ed491a43c8d", "cba84b26e0df5ae7f74fc66b86a95b866872a86292ebd84fa1e959927edd93bf"},
}

func (a *app) bin(name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(a.binDir, name)
}

func (a *app) ytArgs(args ...string) []string {
	return append([]string{"--js-runtimes", "deno:" + a.bin("deno"), "--ffmpeg-location", a.binDir}, args...)
}

func (a *app) prepareTools() {
	message := "Could not get the video tools. Check your internet and reopen Flova Cutter."
	if err := a.installTools(); err != nil {
		log.Print(err)
	} else {
		var listing strings.Builder
		err := a.runTool(a.ctx, "ffmpeg", []string{"-hide_banner", "-encoders"}, &listing)
		encoder := ""
		encoders := make(map[string]bool)
		for line := range strings.SplitSeq(listing.String(), "\n") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				encoders[f[1]] = true
			}
		}
		if encoders["h264_videotoolbox"] {
			encoder = "h264_videotoolbox"
		} else if encoders["libx264"] {
			encoder = "libx264"
		}
		if err == nil && encoder != "" {
			a.mu.Lock()
			a.encoder = encoder
			a.tools = toolState{Ready: true, Percent: 100}
			a.mu.Unlock()
			_ = a.runTool(a.ctx, "yt-dlp", a.ytArgs("-U"), io.Discard)
			return
		}
		message = "This computer's video tool cannot make H.264 files."
	}
	a.mu.Lock()
	a.tools.Error = message
	a.mu.Unlock()
}

type asset struct{ name, url, digest string }

func fetch(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := (&http.Client{Timeout: 30 * time.Minute}).Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return resp, nil
}

func ytRelease(ctx context.Context, name string) (asset, error) {
	const base = "https://github.com/yt-dlp/yt-dlp/releases/"
	resp, err := fetch(ctx, base+"latest/download/SHA2-256SUMS")
	if err != nil {
		return asset{}, err
	}
	defer resp.Body.Close()
	binaryURL := base + "latest/download/" + name
	for req := resp.Request; req != nil; {
		u := req.URL.String()
		if strings.HasPrefix(u, base+"download/") && strings.HasSuffix(u, "/SHA2-256SUMS") {
			binaryURL = strings.TrimSuffix(u, "SHA2-256SUMS") + name
			break
		}
		if req.Response == nil {
			break
		}
		req = req.Response.Request
	}
	s := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		b, err := hex.DecodeString(fields[0])
		if err == nil && len(b) == sha256.Size {
			return asset{"yt-dlp", binaryURL, strings.ToLower(fields[0])}, nil
		}
	}
	return asset{}, errors.New("missing yt-dlp checksum")
}

func (a *app) installTools() error {
	d, ok := distributions[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return errors.New("unsupported platform")
	}
	const ff = "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/"
	const deno = "https://github.com/denoland/deno/releases/download/v2.9.8/deno-"
	assets := []asset{
		{"yt-dlp", "", ""},
		{"ffmpeg", ff + "ffmpeg-" + d.ffArch + ".gz", d.ffHash},
		{"ffprobe", ff + "ffprobe-" + d.ffArch + ".gz", d.probeHash},
		{"deno", deno + d.denoArch + ".zip", d.denoHash},
	}
	var pending []asset
	var bodies []io.ReadCloser
	var total, received int64
	known := true
	for _, item := range assets {
		info, err := os.Stat(a.bin(item.name))
		if err == nil && info.Mode().IsRegular() {
			continue
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if item.name == "yt-dlp" {
			name := "yt-dlp_macos"
			if runtime.GOOS == "windows" {
				name = "yt-dlp.exe"
			}
			item, err = ytRelease(a.ctx, name)
			if err != nil {
				return err
			}
		}
		resp, err := fetch(a.ctx, item.url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		pending = append(pending, item)
		bodies = append(bodies, resp.Body)
		known = known && resp.ContentLength > 0
		total += resp.ContentLength
	}
	progress := writerFunc(func(data []byte) (int, error) {
		received += int64(len(data))
		if known && total > 0 {
			a.mu.Lock()
			a.tools.Percent = int(min(received*100/total, 100))
			a.mu.Unlock()
		}
		return len(data), nil
	})
	for i, item := range pending {
		if err := a.install(item, io.TeeReader(bodies[i], progress)); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) install(item asset, src io.Reader) error {
	f, err := os.CreateTemp(a.binDir, ".download-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, hash), src)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if hex.EncodeToString(hash.Sum(nil)) != item.digest {
		return fmt.Errorf("%s checksum mismatch", item.name)
	}
	path := f.Name()
	if item.name != "yt-dlp" {
		out, err := os.CreateTemp(a.binDir, ".unpack-*")
		if err != nil {
			return err
		}
		defer os.Remove(out.Name())
		err = unpackTool(path, filepath.Base(a.bin(item.name)), item.name == "deno", out)
		closeErr = out.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		path = out.Name()
	}
	if runtime.GOOS == "darwin" {
		if err := os.Chmod(path, 0700); err != nil {
			return err
		}
	}
	return os.Rename(path, a.bin(item.name))
}

func unpackTool(path, name string, zipped bool, out io.Writer) error {
	if zipped {
		z, err := zip.OpenReader(path)
		if err != nil {
			return err
		}
		defer z.Close()
		var entry *zip.File
		for _, f := range z.File {
			if filepath.Base(f.Name) == name && f.Mode().IsRegular() {
				if entry != nil {
					return errors.New("duplicate deno executable")
				}
				entry = f
			}
		}
		if entry == nil {
			return errors.New("missing deno executable")
		}
		r, err := entry.Open()
		if err != nil {
			return err
		}
		defer r.Close()
		_, err = io.Copy(out, r)
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	g, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer g.Close()
	_, err = io.Copy(out, g)
	return err
}
