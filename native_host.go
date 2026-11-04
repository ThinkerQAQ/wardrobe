package main

// Native Messaging provides an explicitly installed browser-to-local launcher.
// It never exposes SQLite records, MCP credentials or an arbitrary command runner.
import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const nativeHostName = "com.thinkerqaq.wardrobe"
const maxNativeBytes = 64 << 10
const localWardrobeURL = "http://127.0.0.1:17936"

var validExtensionID = regexp.MustCompile(`^[a-p]{32}$`)
var nativeInvocation = regexp.MustCompile(`^(?:chrome|edge)-extension://[a-p]{32}/?$`)

type nativeHostManifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

func isNativeHostInvocation(args []string) bool {
	return len(args) > 0 && (args[0] == "--native-host" || nativeInvocation.MatchString(args[0]))
}
func readNativeMessage(r io.Reader) ([]byte, error) {
	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return nil, err
	}
	if n == 0 || n > maxNativeBytes {
		return nil, errors.New("invalid native message size")
	}
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	return b, err
}
func writeNativeMessage(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(data) > maxNativeBytes {
		return errors.New("native response too large")
	}
	if err = binary.Write(w, binary.LittleEndian, uint32(len(data))); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
func ensureLocalWardrobe() error {
	if existingWardrobe(localWardrobeURL) {
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	command := newWardrobeDaemonCommand(executable)
	if err = command.Start(); err != nil {
		return fmt.Errorf("start Wardrobe: %w", err)
	}
	if command.Process != nil {
		_ = command.Process.Release()
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if existingWardrobe(localWardrobeURL) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return errors.New("Wardrobe did not become ready at 127.0.0.1:17936; check config and logs")
}
func runWardrobeNativeHost(r io.Reader, w io.Writer) error {
	for {
		message, err := readNativeMessage(r)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var request struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(message, &request) != nil {
			if err = writeNativeMessage(w, map[string]any{"ok": false, "error": "invalid request"}); err != nil {
				return err
			}
			continue
		}
		if request.Command != "status" && request.Command != "ensure_bridge" {
			if err = writeNativeMessage(w, map[string]any{"ok": false, "error": "unsupported command"}); err != nil {
				return err
			}
			continue
		}
		if request.Command == "status" {
			if err = writeNativeMessage(w, map[string]any{"ok": true, "running": existingWardrobe(localWardrobeURL)}); err != nil {
				return err
			}
			continue
		}
		if err = ensureLocalWardrobe(); err != nil {
			if err = writeNativeMessage(w, map[string]any{"ok": false, "error": err.Error()}); err != nil {
				return err
			}
			continue
		}
		if err = writeNativeMessage(w, map[string]any{"ok": true, "running": true}); err != nil {
			return err
		}
	}
}
func nativeManifestPath(exe string) string {
	return filepath.Join(filepath.Dir(exe), "Data", "native-host", nativeHostName+".json")
}
func nativeDirs(home string) []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts"),
			filepath.Join(home, "Library", "Application Support", "Microsoft Edge", "NativeMessagingHosts"),
			filepath.Join(home, "Library", "Application Support", "Chromium", "NativeMessagingHosts"),
		}
	default:
		return []string{
			filepath.Join(home, ".config", "google-chrome", "NativeMessagingHosts"),
			filepath.Join(home, ".config", "microsoft-edge", "NativeMessagingHosts"),
			filepath.Join(home, ".config", "chromium", "NativeMessagingHosts"),
		}
	}
}
func buildNativeManifest(executable, extensionID string, old []byte) (nativeHostManifest, error) {
	if !validExtensionID.MatchString(extensionID) {
		return nativeHostManifest{}, errors.New("extension ID must contain 32 a-p letters")
	}
	origins := map[string]bool{"chrome-extension://" + extensionID + "/": true}
	var prior nativeHostManifest
	if len(old) > 0 {
		if err := json.Unmarshal(old, &prior); err != nil || prior.Name != nativeHostName || prior.Type != "stdio" {
			return nativeHostManifest{}, errors.New("refusing to replace unrecognized Native Messaging manifest")
		}
	}
	if len(old) > 0 {
		for _, origin := range prior.AllowedOrigins {
			if strings.HasPrefix(origin, "chrome-extension://") && strings.HasSuffix(origin, "/") {
				id := strings.TrimSuffix(strings.TrimPrefix(origin, "chrome-extension://"), "/")
				if validExtensionID.MatchString(id) {
					origins[origin] = true
				}
			}
		}
	}
	values := make([]string, 0, len(origins))
	for origin := range origins {
		values = append(values, origin)
	}
	sort.Strings(values)
	return nativeHostManifest{Name: nativeHostName, Description: "Wardrobe local service launcher", Path: executable, Type: "stdio", AllowedOrigins: values}, nil
}
func registerWardrobeNativeHost(extensionID string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	runnerExecutable := exe
	if runtime.GOOS == "windows" {
		gui := filepath.Join(filepath.Dir(exe), "wardrobe-host.exe")
		if st, err := os.Stat(gui); err == nil && !st.IsDir() {
			runnerExecutable = gui // Windows GUI subsystem for browser Native Messaging
		}
	}
	manifest := nativeManifestPath(exe)
	previous, readErr := os.ReadFile(manifest)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	descriptor, err := buildNativeManifest(runnerExecutable, extensionID, previous)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(manifest), 0700); err != nil {
		return err
	}
	if err = os.WriteFile(manifest, append(data, '\n'), 0600); err != nil {
		return err
	}
	switch runtime.GOOS {
	case "windows":
		for _, browser := range []string{`Google\Chrome`, `Microsoft\Edge`} {
			key := `HKCU\Software\` + browser + `\NativeMessagingHosts\` + nativeHostName
			out, err := exec.Command("reg.exe", "add", key, "/ve", "/t", "REG_SZ", "/d", manifest, "/f").CombinedOutput()
			if err != nil {
				return fmt.Errorf("register %s: %w: %s", browser, err, strings.TrimSpace(string(out)))
			}
		}
	case "darwin", "linux":
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		for _, dir := range nativeDirs(home) {
			if err = os.MkdirAll(dir, 0700); err != nil {
				return err
			}
			if err = os.WriteFile(filepath.Join(dir, nativeHostName+".json"), append(data, '\n'), 0600); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported operating system %s", runtime.GOOS)
	}
	return nil
}
