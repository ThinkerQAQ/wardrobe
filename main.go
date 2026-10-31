package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

//go:embed web/*
var assets embed.FS

//go:embed skills/*/SKILL.md
var bundledSkills embed.FS

//go:embed skills/*/references/*.md
var bundledSkillReferences embed.FS

func main() {
	exe, _ := os.Executable()
	configPath := flag.String("config", filepath.Join(filepath.Dir(exe), "Data", "config.toml"), "TOML configuration file")
	data := flag.String("data", "", "override configured data directory")
	addr := flag.String("addr", "127.0.0.1:17936", "loopback listen address")
	open := flag.Bool("open", true, "open browser")
	stdio := flag.Bool("mcp-stdio", false, "serve MCP over stdin/stdout")
	flag.Parse()
	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	cfg, err := loadConfig(*configPath, explicit["config"])
	if err != nil {
		log.Fatal(err)
	}
	if !explicit["addr"] {
		*addr = cfg.Server.Address
	}
	if !explicit["data"] {
		*data = cfg.Storage.DataDir
	}
	if !explicit["open"] {
		*open = cfg.Server.OpenBrowser
	}
	if err := validateAddress(*addr); err != nil {
		log.Fatal(err)
	}
	s, err := newStore(*data)
	if err != nil {
		log.Fatal(err)
	}
	defer s.db.Close()
	ui, _ := fs.Sub(assets, "web")
	app := &application{s: s, ui: http.FileServer(http.FS(ui)), host: *addr}
	mcpServer := app.newMCPServer()
	if *stdio {
		if err := runReinitializableMCPStdio(context.Background(), app.newMCPServer, os.Stdin, os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}
	token, err := ensureMCPToken(*data)
	if err != nil {
		log.Fatal(err)
	}
	app.mcp = mcpHTTP(mcpServer, token)
	url := "http://" + *addr
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		if *open && existingWardrobe(url) {
			openURL(url)
			return
		}
		log.Fatalf("Cannot start %s: %v", url, err)
	}
	fmt.Println("Wardrobe / 衣橱 " + url + "\nData: " + *data)
	if *open {
		go func() { time.Sleep(300 * time.Millisecond); openURL(url) }()
	}
	srv := &http.Server{Handler: app, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.Serve(ln))
}
func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("Open %s manually: %v", url, err)
	}
}

func existingWardrobe(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	r, e := client.Get(url + "/api/health")
	if e != nil {
		return false
	}
	defer r.Body.Close()
	var h struct {
		App string `json:"app"`
	}
	return r.StatusCode == 200 && json.NewDecoder(r.Body).Decode(&h) == nil && h.App == "wardrobe"
}
