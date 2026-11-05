//go:build !windows

package main

import "errors"

func openNativeGUI(action string) error { return errors.New("native GUI launcher is Windows-only") }
func notifyGUIError(err error)          {}
