package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const mcpExampleID = "tunnel_0123456789abcdef0123456789abcdef"
const mcpRunnerCommand = "tunnel-run"

var nativeLifecycleMu sync.Mutex

type nativeMCPPaths struct {
	Profile, Health, Private, Key, Revision, Restart, State, Runner, Stop string
}

func mcpNativePaths(goos, dataDir string) nativeMCPPaths {
	profile, health := mcpPaths(dataDir)
	private := filepath.Dir(health)
	key := filepath.Join(private, "runtime.env")
	if goos == "windows" {
		key = filepath.Join(private, "runtime-key.dpapi")
	}
	return nativeMCPPaths{profile, health, private, key,
		filepath.Join(private, "configuration-revision"), filepath.Join(private, "restart-required"),
		filepath.Join(private, "setup-state"), filepath.Join(private, "runner.json"), filepath.Join(private, "stop-request")}
}
func localMCPPaths() (nativeMCPPaths, error) {
	dataDir, err := ConfigDir()
	if err != nil {
		return nativeMCPPaths{}, err
	}
	return mcpNativePaths(runtime.GOOS, dataDir), nil
}
func validNativeTunnel(id string) bool { return tunnelIDPattern.MatchString(id) && id != mcpExampleID }
func validRuntimeKey(key string) bool {
	if len(key) < 16 || len(key) > 512 {
		return false
	}
	for _, c := range key {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
func nativeRevision() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func privateWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".wardrobe-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err = temp.Chmod(0600); err != nil {
		temp.Close()
		return err
	}
	if _, err = temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func mcpReadKey(p nativeMCPPaths) (string, error) {
	data, err := os.ReadFile(p.Key)
	if err != nil {
		return "", err
	}
	plain, err := mcpUnprotectKey(data)
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(plain))
	if !validRuntimeKey(key) {
		return "", errors.New("stored runtime key is invalid")
	}
	return key, nil
}
func mcpEncodeKey(key string) ([]byte, error) {
	if !validRuntimeKey(key) {
		return nil, errors.New("invalid runtime key")
	}
	if runtime.GOOS != "windows" {
		return []byte(key), nil
	}
	return mcpProtectKey([]byte(key))
}
func mcpSnapshot(path string) (data []byte, exists bool, err error) {
	data, err = os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}
func mcpRestore(path string, data []byte, exists bool) error {
	if !exists {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return privateWrite(path, data)
}
func mcpRunClient(ctx context.Context, client string, key string, arguments ...string) error {
	cmd := exec.CommandContext(ctx, client, arguments...)
	cmd.Env = append(os.Environ(), "CONTROL_PLANE_API_KEY="+key)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	mcpHideProcess(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("official Tunnel client %s failed: %w", arguments[0], err)
	}
	return nil
}
func nativeSave(ctx context.Context, in mcpConnectionAction) (err error) {
	if err := validateMCPConfig(); err != nil {
		return err
	}
	if !validNativeTunnel(in.TunnelID) {
		return errors.New("invalid or example Tunnel ID")
	}
	p, err := localMCPPaths()
	if err != nil {
		return err
	}
	// A Save transaction and runner startup must never see a torn profile/key.
	release, err := mcpConfigurationLock(p.Private)
	if err != nil {
		return err
	}
	defer release()
	client := findTunnelClient(runtime.GOOS, func() string { h, _ := os.UserHomeDir(); return h }())
	if client == "" {
		return errors.New("official Tunnel client is missing")
	}
	currentID := readTunnelID(p.Profile)
	if currentID != "" && (!in.Replace || in.PreviousTunnelID != currentID) {
		return errors.New("saved Tunnel ID changed: refresh before replacing")
	}
	if currentID == "" && in.Replace {
		return errors.New("no existing Tunnel to replace")
	}
	if in.ReuseKey && in.RuntimeAPIKey != "" {
		return errors.New("key reuse and replacement conflict")
	}
	var key string
	if in.ReuseKey {
		if currentID == "" {
			return errors.New("cannot reuse key without saved Tunnel")
		}
		key, err = mcpReadKey(p)
		if err != nil {
			return errors.New("saved Runtime API Key unavailable")
		}
	} else {
		key = in.RuntimeAPIKey
		if !validRuntimeKey(key) {
			return errors.New("invalid Runtime API Key")
		}
	}
	if currentID == in.TunnelID {
		if oldKey, readErr := mcpReadKey(p); readErr == nil && oldKey == key {
			if revision, readErr := os.ReadFile(p.Revision); readErr == nil && len(revision) > 0 {
				return nil // identical Save is idempotent across GUI and CLI.
			}
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.Private, 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.Profile), 0700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(p.Private, "pending-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	ctx, cancel := context.WithTimeout(ctx, 95*time.Second)
	defer cancel()
	args := []string{"init", "--sample", "sample_mcp_stdio_local", "--profile-dir", staging, "--profile", "wardrobe",
		"--tunnel-id", in.TunnelID, "--mcp-command", mcpStdioCommand(exe, mcpConfigPath()),
		"--health-listen-addr", "127.0.0.1:0"}
	if err = mcpRunClient(ctx, client, key, args...); err != nil {
		return err
	}
	candidate := filepath.Join(staging, "wardrobe.yaml")
	yaml, err := os.ReadFile(candidate)
	if err != nil || readTunnelID(candidate) != in.TunnelID {
		return errors.New("candidate Tunnel profile invalid")
	}
	if len(yaml) == 0 || len(yaml) > 128<<10 {
		return errors.New("candidate Tunnel profile out of bounds")
	}
	if err = mcpRunClient(ctx, client, key, "doctor", "--config", candidate); err != nil {
		return err
	}
	encrypted, err := mcpEncodeKey(key)
	if err != nil {
		return err
	}
	// Snapshot every file, including markers, before changing persistent state.
	type backup struct {
		path   string
		data   []byte
		exists bool
	}
	backups := make([]backup, 0, 4)
	for _, path := range []string{p.Profile, p.Key, p.Revision, p.Restart} {
		data, exists, readErr := mcpSnapshot(path)
		if readErr != nil {
			return readErr
		}
		backups = append(backups, backup{path, data, exists})
	}
	revision, err := nativeRevision()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		for _, old := range backups {
			if restoreErr := mcpRestore(old.path, old.data, old.exists); restoreErr != nil {
				err = errors.Join(err, fmt.Errorf("configuration rollback failed: %w", restoreErr))
			}
		}
	}()
	if err = privateWrite(p.Profile, yaml); err != nil {
		return err
	}
	if err = privateWrite(p.Key, encrypted); err != nil {
		return err
	}
	if err = privateWrite(p.Revision, []byte(revision)); err != nil {
		return err
	}
	if mcpRunnerRunning(p) {
		if err = privateWrite(p.Restart, []byte("true")); err != nil {
			return err
		}
	} else if err = os.Remove(p.Restart); err != nil && !os.IsNotExist(err) {
		return err
	}
	committed = true
	return nil
}
func readTunnelID(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 128<<10 {
		return ""
	}
	m := profileTunnelID.FindSubmatch(data)
	if len(m) != 2 {
		return ""
	}
	id := string(m[1])
	if !validNativeTunnel(id) {
		return ""
	}
	return id
}
func writeMCPState(path, state string) { _ = privateWrite(path, []byte(state)) }

type tunnelRunnerInfo struct {
	PID       int    `json:"pid"`
	Token     string `json:"token"`
	Exe       string `json:"exe"`
	StartedAt uint64 `json:"started_at,omitempty"`
}

func readRunnerInfo(p nativeMCPPaths) tunnelRunnerInfo {
	data, err := os.ReadFile(p.Runner)
	if err != nil || len(data) > 4096 {
		return tunnelRunnerInfo{}
	}
	var info tunnelRunnerInfo
	if json.Unmarshal(data, &info) != nil || info.PID <= 0 || len(info.Token) != 32 {
		return tunnelRunnerInfo{}
	}
	return info
}
func mcpRunnerRunning(p nativeMCPPaths) bool {
	info := readRunnerInfo(p)
	if info.PID <= 0 || !mcpSameProcess(info.PID, info.Exe, info.StartedAt) {
		return false
	}
	return true
}
func nativeRequestStop(p nativeMCPPaths) error {
	info := readRunnerInfo(p)
	if info.PID == 0 || !mcpSameProcess(info.PID, info.Exe, info.StartedAt) {
		return nil
	}
	if err := privateWrite(p.Stop, []byte(info.Token)); err != nil {
		return err
	}
	for i := 0; i < 40; i++ {
		if !mcpRunnerRunning(p) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return errors.New("Tunnel runner did not stop")
}
func nativeStart() error {
	if err := validateMCPConfig(); err != nil {
		return err
	}
	p, err := localMCPPaths()
	if err != nil {
		return err
	}
	if mcpRunnerRunning(p) {
		return nil
	}
	if !validNativeTunnel(readTunnelID(p.Profile)) {
		return errors.New("save Tunnel ID before starting")
	}
	if _, err = mcpReadKey(p); err != nil {
		return errors.New("save Runtime API Key before starting")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, append([]string{mcpRunnerCommand}, mcpConfigArguments()...)...)
	cmd.Dir = filepath.Dir(exe)
	mcpDetachProcess(cmd)
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	// The runner owns a cross-process lock and writes its own PID after start.
	for attempt := 0; attempt < 40; attempt++ {
		if mcpRunnerRunning(p) {
			_ = os.Remove(p.Restart)
			return nil
		}
		time.Sleep(125 * time.Millisecond)
	}
	return errors.New("Tunnel runner failed to start")
}
func nativeAction(action string) error {
	nativeLifecycleMu.Lock()
	defer nativeLifecycleMu.Unlock()
	p, err := localMCPPaths()
	if err != nil {
		return err
	}
	switch action {
	case "start":
		return nativeStart()
	case "stop":
		return nativeRequestStop(p)
	case "restart":
		if err = nativeRequestStop(p); err != nil {
			return err
		}
		return nativeStart()
	default:
		return errors.New("unsupported native Tunnel action")
	}
}

// RunTunnel runs a local supervisor even when automatic Tunnel connection is disabled.
// Connect/disconnect are authenticated by a per-process token in private files and
// never stop the supervisor. Only a service Restart adopts saved configuration.
func RunTunnel(ctx context.Context) error {
	if err := validateMCPConfig(); err != nil {
		return err
	}
	p, err := localMCPPaths()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(p.Private, 0700); err != nil {
		return err
	}
	unlock, err := mcpRunnerExclusive(p.Private)
	if err != nil {
		return err
	}
	defer unlock()
	configRelease, err := mcpConfigurationLock(p.Private)
	if err != nil {
		return err
	}
	key, err := mcpReadKey(p)
	if err != nil {
		configRelease()
		return err
	}
	profile, err := os.ReadFile(p.Profile)
	if err != nil || len(profile) == 0 || len(profile) > 128<<10 {
		configRelease()
		return errors.New("Tunnel profile missing or invalid")
	}
	// Freeze the profile with the same revision as the key. Reconnect must not
	// combine old credentials with a newer saved Tunnel ID.
	active := filepath.Join(p.Private, "active-"+strconv.Itoa(os.Getpid())+".yaml")
	if err = privateWrite(active, profile); err != nil {
		configRelease()
		return err
	}
	// Hold the lock until the runner marker is visible to Save.
	// A concurrent Save must either precede this snapshot or mark Restart.
	locked := true
	defer func() {
		if locked {
			configRelease()
		}
	}()
	defer os.Remove(active)
	home, _ := os.UserHomeDir()
	client := findTunnelClient(runtime.GOOS, home)
	if client == "" {
		return errors.New("official Tunnel client missing")
	}
	token, err := nativeRevision()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	startedAt, err := mcpProcessStartTime(os.Getpid())
	if err != nil {
		return fmt.Errorf("read Tunnel runner process identity: %w", err)
	}
	info := tunnelRunnerInfo{PID: os.Getpid(), Token: token, Exe: exe, StartedAt: startedAt}
	record, err := json.Marshal(info)
	if err != nil {
		return err
	}
	if err = privateWrite(p.Runner, record); err != nil {
		return err
	}
	connecting := mcpAutoConnectEnabled(p)
	current := mcpRunnerState{Token: token, Connecting: connecting}
	if err = mcpWriteRunnerState(p, current); err != nil {
		return err
	}
	configRelease()
	locked = false
	defer func() {
		if state, ok := mcpReadRunnerState(p); ok && state.Token == token {
			_ = os.Remove(mcpRunnerStatePath(p))
		}
		if readRunnerInfo(p).Token == token {
			_ = os.Remove(p.Runner)
		}
		_ = os.Remove(mcpRunnerCommandPath(p))
		if b, e := os.ReadFile(p.Stop); e == nil && strings.TrimSpace(string(b)) == token {
			_ = os.Remove(p.Stop)
		}
	}()
	_ = os.Remove(p.Stop)

	var child *exec.Cmd
	var terminate func() error
	var exited chan error
	var retry <-chan time.Time
	stopChild := func() error {
		retry = nil
		if child == nil {
			return nil
		}
		if err := terminate(); err != nil {
			return fmt.Errorf("could not stop owned Tunnel process: %w", err)
		}
		child = nil
		channel := exited
		exited = nil
		select {
		case <-channel:
		case <-time.After(4 * time.Second):
			return errors.New("owned Tunnel child did not stop")
		}
		terminate = nil
		_ = os.Remove(p.Health)
		return nil
	}
	defer func() { _ = stopChild() }()
	startChild := func() {
		if !connecting || child != nil {
			return
		}
		_ = os.Remove(p.Health)
		command := exec.Command(client, "run", "--config", active, "--health.url-file", p.Health)
		command.Env = append(os.Environ(), "CONTROL_PLANE_API_KEY="+key)
		command.Stdin = nil
		command.Stdout = io.Discard
		command.Stderr = io.Discard
		stop, launchErr := mcpLaunchChild(command)
		if launchErr != nil {
			retry = time.After(5 * time.Second)
			return
		}
		terminate = stop
		child = command
		exited = make(chan error, 1)
		go func(c *exec.Cmd, ch chan error) { ch <- c.Wait() }(command, exited)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	startChild()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-signals:
			return nil
		case <-exited:
			if terminate != nil {
				_ = terminate()
				terminate = nil
			}
			child = nil
			exited = nil
			if connecting {
				retry = time.After(5 * time.Second)
			}
		case <-retry:
			retry = nil
			startChild()
		case <-ticker.C:
			if data, e := os.ReadFile(p.Stop); e == nil && strings.TrimSpace(string(data)) == token {
				return nil
			}
			raw, e := os.ReadFile(mcpRunnerCommandPath(p))
			if e != nil {
				continue
			}
			_ = os.Remove(mcpRunnerCommandPath(p))
			if len(raw) > 2048 {
				continue
			}
			var request mcpRunnerControlRequest
			if json.Unmarshal(raw, &request) != nil || request.Token != token || len(request.ID) != 32 {
				continue
			}
			switch request.Action {
			case "connect":
				connecting = true
				startChild()
			case "disconnect":
				connecting = false
				if err := stopChild(); err != nil {
					continue
				}
			default:
				continue
			}
			current.Connecting = connecting
			current.ActionID = request.ID
			_ = mcpWriteRunnerState(p, current)
		}
	}
}
