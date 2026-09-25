package sidecar

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return filepath.Dir(exe)
	}
	return filepath.Dir(exe)
}

func resourceDirs() []string {
	dir := exeDir()
	var out []string
	if dir != "" {
		out = append(out,
			dir,
			filepath.Join(dir, "sidecar"),
			filepath.Join(dir, "sidecar", "ffmpeg"),
			filepath.Join(dir, "sidecar", "whisper"),
			filepath.Join(dir, "sidecar", "llama"),
			filepath.Join(dir, "models"),
		)
		if runtime.GOOS == "darwin" {
			res := filepath.Join(dir, "..", "Resources")
			out = append(out,
				res,
				filepath.Join(res, "sidecar"),
				filepath.Join(res, "sidecar", "ffmpeg"),
				filepath.Join(res, "sidecar", "whisper"),
				filepath.Join(res, "sidecar", "llama"),
				filepath.Join(res, "models"),
			)
		}
	}
	if wd, err := os.Getwd(); err == nil {
		tp := filepath.Join(wd, "third_party")
		out = append(out,
			filepath.Join(tp, "ffmpeg"),
			filepath.Join(tp, "whisper"),
			filepath.Join(tp, "llama"),
			filepath.Join(tp, "models"),
		)
	}
	return out
}

func lookupFile(names ...string) string {
	for _, dir := range resourceDirs() {
		for _, name := range names {
			for _, rel := range []string{name, filepath.Join("bin", name), filepath.Join("build", "bin", name)} {
				p := filepath.Join(dir, rel)
				if st, err := os.Stat(p); err == nil && !st.IsDir() {
					return p
				}
			}
		}
	}
	return ""
}

func lookupPath(explicit, envName string, names ...string) string {
	try := func(v string) string {
		v = strings.TrimSpace(v)
		if v == "" {
			return ""
		}
		if p, err := exec.LookPath(v); err == nil {
			return p
		}
		if _, err := os.Stat(v); err == nil {
			return v
		}
		return ""
	}
	if p := try(explicit); p != "" {
		return p
	}
	if envName != "" {
		if p := try(os.Getenv(envName)); p != "" {
			return p
		}
	}
	if p := lookupFile(names...); p != "" {
		return p
	}
	for _, name := range names {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func FFmpeg(explicit string) string {
	names := []string{"ffmpeg"}
	if runtime.GOOS == "windows" {
		names = []string{"ffmpeg.exe", "ffmpeg"}
	}
	return lookupPath(explicit, "ATM_FFMPEG_BIN", names...)
}

func FFprobe(explicit string) string {
	names := []string{"ffprobe"}
	if runtime.GOOS == "windows" {
		names = []string{"ffprobe.exe", "ffprobe"}
	}
	return lookupPath(explicit, "ATM_FFPROBE_BIN", names...)
}

func Whisper(explicit string) string {
	names := []string{"whisper-cli", "whisper", "main"}
	if runtime.GOOS == "windows" {
		names = []string{"whisper-cli.exe", "whisper.exe", "main.exe", "whisper-cli", "whisper"}
	}
	return lookupPath(explicit, "ATM_WHISPER_BIN", names...)
}

func Llama(explicit string) string {
	names := []string{"llama-server", "llama-cli"}
	if runtime.GOOS == "windows" {
		names = []string{"llama-server.exe", "llama-cli.exe", "llama-server", "llama-cli"}
	}
	return lookupPath(explicit, "ATM_LLAMA_BIN", names...)
}

func FlavorFile() string {
	return lookupFile("flavor.json")
}

func FindModel(explicit, envName, defaultName string) string {
	names := []string{}
	if defaultName != "" {
		names = append(names, defaultName)
	}
	return lookupPath(explicit, envName, names...)
}

// Prep sets cwd and library path so sidecar dylibs next to the binary load.
func Prep(cmd *exec.Cmd) {
	if cmd == nil || cmd.Path == "" {
		return
	}
	dir := filepath.Dir(cmd.Path)
	cmd.Dir = dir
	env := os.Environ()
	switch runtime.GOOS {
	case "darwin":
		env = append(env, "DYLD_LIBRARY_PATH="+dir, "DYLD_FALLBACK_LIBRARY_PATH="+dir)
	case "windows":
		env = append(env, "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	default:
		env = append(env, "LD_LIBRARY_PATH="+dir)
	}
	cmd.Env = env
}
