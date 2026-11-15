package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// startUpdate hands the install to a detached script: YAM replaces this very
// plugin payload, and the receiver has to stop before YAM can take its lock.
func startUpdate() {
	root := yamRoot()
	model := lunaModel()
	script := strings.NewReplacer("@@ROOT@@", shellQuote(root), "@@MODEL@@", shellQuote(model)).Replace(updateBody)
	if err := os.WriteFile("/tmp/airplay-update.sh", []byte(script), 0o755); err != nil {
		return
	}
	command := exec.Command("/bin/sh", "/tmp/airplay-update.sh")
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	_ = command.Start()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// yamRoot finds the YAM tree from this executable's payload path. The plugin
// runs from <root>/pkgs/<owner>/<package>/<version>/share/..., which also works
// on devices that keep YAM outside /data.
func yamRoot() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for depth := 0; depth < 10; depth++ {
			if filepath.Base(dir) == "pkgs" {
				return filepath.Dir(dir)
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if root := lunaYamRoot(); root != "" {
		return root
	}
	return "/data/yam"
}

func lunaYamRoot() string {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || name[0] < '0' || name[0] > '9' {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", name, "cmdline"))
		if err != nil {
			continue
		}
		args := strings.Split(string(data), "\x00")
		if len(args) == 0 || filepath.Base(args[0]) != "luna" {
			continue
		}
		for i, arg := range args {
			if arg == "--yam-root" && i+1 < len(args) {
				return args[i+1]
			}
			if strings.HasPrefix(arg, "--yam-root=") {
				return strings.TrimPrefix(arg, "--yam-root=")
			}
		}
	}
	return ""
}

func lunaModel() string {
	client := &http.Client{Timeout: 5 * time.Second}
	request, err := http.NewRequest("GET", "http://127.0.0.1:8099/api/device", nil)
	if err != nil {
		return ""
	}
	request.Header.Set("X-Luna-Local", "1")
	response, err := client.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	var device struct {
		Model string `json:"model"`
	}
	if json.NewDecoder(response.Body).Decode(&device) != nil {
		return ""
	}
	return device.Model
}

const updateBody = `#!/bin/sh
LOG=/tmp/airplay-update.log
URL=http://127.0.0.1:8099
ROOT=@@ROOT@@
MODEL=@@MODEL@@
log() { echo "$(date '+%Y-%m-%d %H:%M:%S') $*" >>"$LOG"; }
post() { wget -q -O /dev/null -T 20 --header='Content-Type: application/json' --post-data='{}' "$URL$1" 2>/dev/null; }
: >"$LOG"
if [ -z "$MODEL" ] || [ ! -x "$ROOT/bin/yam" ]; then
  log "update: yam or model unavailable (root=$ROOT model=$MODEL)"
  exit 1
fi
log "update: stopping the panel and the receiver"
# Luna takes a shared YAM lease for running plugins, including this Lua panel,
# so the receiver alone is not enough to free the lock.
post /api/plugins/airplay.panel/stop
post /api/plugins/airplay.panel-command/stop
post /api/plugins/airplay.airplay-luna/stop
i=0
while [ "$i" -lt 30 ] && pidof station-airplay >/dev/null 2>&1; do
  i=$((i+1)); sleep 1
done
sleep 1
log "update: plugins stopped after ${i}s"
attempt=1
while :; do
  out=$("$ROOT/bin/yam" --platform "$MODEL" --profiles "$ROOT/platforms" install -y airplay/airplay-luna 2>&1)
  rc=$?
  printf '%s\n' "$out" >>"$LOG"
  case "$out" in
    *"another yam transaction is running"*)
      attempt=$((attempt+1))
      if [ "$attempt" -gt 3 ]; then log "update: lock still busy"; break; fi
      log "update: lock busy, retry $attempt"
      sleep 4
      continue ;;
  esac
  break
done
log "update: yam result $rc"
post /api/plugins/airplay.airplay-luna/service/enable
log "update: enable requested"
`
