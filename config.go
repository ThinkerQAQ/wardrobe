package main

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pelletier/go-toml/v2"
)

type config struct {
	Server struct {
		Address     string `toml:"address"`
		OpenBrowser bool   `toml:"open_browser"`
	} `toml:"server"`
	Storage struct {
		DataDir string `toml:"data_dir"`
	} `toml:"storage"`
}

func loadConfig(path string, explicit bool) (config, error) {
	var c config
	c.Server.Address = "127.0.0.1:17936"
	c.Server.OpenBrowser = true
	c.Storage.DataDir = "."
	b, err := os.ReadFile(path)
	if err != nil && !(errors.Is(err, os.ErrNotExist) && !explicit) {
		return c, err
	}
	if err == nil {
		if err = toml.NewDecoder(bytes.NewReader(b)).DisallowUnknownFields().Decode(&c); err != nil {
			return c, fmt.Errorf("invalid config: %w", err)
		}
	}
	if c.Storage.DataDir == "" {
		return c, fmt.Errorf("storage.data_dir cannot be empty")
	}
	if !filepath.IsAbs(c.Storage.DataDir) {
		c.Storage.DataDir = filepath.Join(filepath.Dir(path), c.Storage.DataDir)
	}
	return c, nil
}

func validateAddress(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	n, e := strconv.Atoi(port)
	if err != nil || e != nil || n < 1 || n > 65535 || (host != "127.0.0.1" && host != "localhost") {
		return fmt.Errorf("server.address must use localhost or 127.0.0.1 and port 1..65535")
	}
	return nil
}
