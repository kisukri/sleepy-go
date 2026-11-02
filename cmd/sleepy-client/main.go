package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/BurntSushi/toml"
)

const version = "0.1.0"

const template = `# Sleepy Go client
# Fill in server, secret, device_id and device_name, then start the client again.

# Server address, for example https://status.example.com (or http://1.2.3.4:9010)
server = ""

# Use main.device_secret if you set one (it can only report devices), otherwise main.secret
secret = ""

# Unique id and the name shown on the page
device_id = "my-pc"
device_name = "My PC"

# Check every N seconds
interval_seconds = 5

# Send again every N seconds even if nothing changed (keeps auto_dead happy)
heartbeat_seconds = 300

# No mouse or keyboard input for N minutes = not in use
idle_minutes = 15

# Add battery level like [🔋82%⚡] in front of the title (laptops only)
battery = true

# "file.txt - Notepad" -> "Notepad - file.txt"
app_first = false

# Extra window titles to ignore (the last title stays)
skip_titles = []

# Extra window titles that mean "not in use", e.g. a lock screen app
not_using_titles = []
`

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func openLog(path string) io.Writer {
	if fi, err := os.Stat(path); err == nil && fi.Size() > 1<<20 {
		_ = os.Rename(path, path+".old")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return os.Stderr
	}
	return io.MultiWriter(f, os.Stderr)
}

func main() {
	dir := exeDir()
	cfgPath := flag.String("config", filepath.Join(dir, "sleepy-client.toml"), "config file")
	logPath := flag.String("log", filepath.Join(dir, "sleepy-client.log"), "log file")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println("sleepy-client", version)
		return
	}
	log.SetOutput(openLog(*logPath))

	cfg := defaultConfig()
	if _, err := os.Stat(*cfgPath); os.IsNotExist(err) {
		if err := os.WriteFile(*cfgPath, []byte(template), 0o600); err != nil {
			log.Fatalf("cannot create %s: %v", *cfgPath, err)
		}
		log.Fatalf("created %s - fill it in and start again", *cfgPath)
	}
	if _, err := toml.DecodeFile(*cfgPath, &cfg); err != nil {
		log.Fatalf("%s: %v", *cfgPath, err)
	}
	if err := cfg.validate(); err != nil {
		log.Fatalf("%s: %v", *cfgPath, err)
	}
	sampler, err := newSampler()
	if err != nil {
		log.Fatal(err)
	}

	// windows: logoff / shutdown / console close arrive as SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("sleepy-client %s: %s as %q -> %s", version, cfg.DeviceID, cfg.DeviceName, endpoint(cfg.Server))
	NewClient(cfg, sampler, log.Printf).Run(ctx)
	log.Print("stopped")
}
