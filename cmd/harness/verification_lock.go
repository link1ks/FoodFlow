package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func verificationLock(root string) (func(), error) {
	dir := filepath.Join(root, ".cache/harness")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "verification.lock")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("full/fast/acceptance already running or interrupted: inspect .cache/harness/verification.lock; never remove a live run's lock")
	}
	if _, err := fmt.Fprintf(file, "pid=%d\n", os.Getpid()); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return func() { _ = os.Remove(path) }, nil
}
