package media

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alexandr/audio-text-manager-desktop/internal/sidecar"
)

var AudioExt = map[string]struct{}{
	".wav": {}, ".mp3": {}, ".m4a": {}, ".flac": {}, ".ogg": {}, ".mp4": {},
}

func Ext(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

func IsAllowed(path string) bool {
	_, ok := AudioExt[Ext(path)]
	return ok
}

func resolve(bin, fallback string) (string, error) {
	if strings.TrimSpace(bin) != "" {
		if p, err := exec.LookPath(bin); err == nil {
			return p, nil
		}
		if _, err := os.Stat(bin); err == nil {
			return bin, nil
		}
	}
	if p, err := exec.LookPath(fallback); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%s not found; bundle ffmpeg with make dist or set the path in settings", fallback)
}

func HasAudioStream(ffprobeBin, path string) (bool, error) {
	bin, err := resolve(ffprobeBin, "ffprobe")
	if err != nil {
		return false, err
	}
	cmd := exec.Command(bin, "-v", "error", "-select_streams", "a:0",
		"-show_entries", "stream=codec_type", "-of", "csv=p=0", path)
	sidecar.Prep(cmd)
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return strings.Contains(strings.ToLower(string(out)), "audio"), nil
}

func DurationSeconds(ffprobeBin, path string) (float64, error) {
	bin, err := resolve(ffprobeBin, "ffprobe")
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(bin, "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", path)
	sidecar.Prep(cmd)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(out))
	if s == "" || s == "N/A" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}

// NormalizeToWAV converts audio or the first audio track of a video to mono 16 kHz PCM WAV.
func NormalizeToWAV(ffmpegBin, src, dst string) error {
	bin, err := resolve(ffmpegBin, "ffmpeg")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	cmd := exec.Command(bin,
		"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-i", src,
		"-map", "0:a:0", "-vn",
		"-ac", "1", "-ar", "16000",
		"-c:a", "pcm_s16le",
		dst,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	sidecar.Prep(cmd)
	if err := cmd.Run(); err != nil {
		detail := strings.ToLower(stderr.String())
		if strings.Contains(detail, "matches no streams") || strings.Contains(detail, "does not contain any stream") {
			return fmt.Errorf("no audio track in file; cannot transcribe")
		}
		return fmt.Errorf("ffmpeg failed: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func CopyLimit(src, dst string, limit int64) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	if limit > 0 && st.Size() > limit {
		kind := "audio"
		if Ext(src) == ".mp4" {
			kind = "video"
		}
		return fmt.Errorf("file too large: %d bytes exceeds %s limit %d bytes", st.Size(), kind, limit)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func UniqueWAV(uploadsDir, jobID string) string {
	return filepath.Join(uploadsDir, jobID+".wav")
}

func PrepareUpload(ffmpegBin, ffprobeBin, src, uploadsDir, jobID string, audioLimit, videoLimit int64, maxDurSec int) (wavPath string, origName string, err error) {
	origName = filepath.Base(src)
	ext := Ext(src)
	if !IsAllowed(src) {
		return "", origName, fmt.Errorf("unsupported format %s", ext)
	}
	limit := audioLimit
	if ext == ".mp4" {
		limit = videoLimit
	}
	tmp := filepath.Join(uploadsDir, jobID+ext)
	if err := CopyLimit(src, tmp, limit); err != nil {
		return "", origName, err
	}
	if ext == ".mp4" {
		ok, herr := HasAudioStream(ffprobeBin, tmp)
		if herr == nil && !ok {
			_ = os.Remove(tmp)
			return "", origName, fmt.Errorf("no audio track in video; cannot transcribe")
		}
	}
	wavPath = UniqueWAV(uploadsDir, jobID)
	if err := NormalizeToWAV(ffmpegBin, tmp, wavPath); err != nil {
		_ = os.Remove(tmp)
		_ = os.Remove(wavPath)
		return "", origName, err
	}
	if ext == ".mp4" {
		_ = os.Remove(tmp)
	} else if tmp != wavPath {
		_ = os.Remove(tmp)
	}
	if maxDurSec > 0 {
		d, _ := DurationSeconds(ffprobeBin, wavPath)
		if d > float64(maxDurSec) {
			_ = os.Remove(wavPath)
			return "", origName, fmt.Errorf("audio too long: %.0fs exceeds %ds", d, maxDurSec)
		}
	}
	_ = os.Chtimes(wavPath, time.Now(), time.Now())
	return wavPath, origName, nil
}
