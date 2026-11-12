// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package results

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreLockAcrossProcesses(t *testing.T) {
	if os.Getenv("SIAFU_LOCK_HELPER") == "1" {
		store, err := OpenStore(os.Getenv("SIAFU_LOCK_DIR"))
		if err != nil {
			t.Fatal(err)
		}
		if err := store.withLock(func() error {
			if err := os.WriteFile(os.Getenv("SIAFU_LOCK_MARKER"), []byte("locked"), 0o600); err != nil {
				return err
			}
			time.Sleep(300 * time.Millisecond)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return
	}

	dir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "marker")
	cmd := exec.Command(os.Args[0], "-test.run=TestStoreLockAcrossProcesses")
	cmd.Env = append(os.Environ(),
		"SIAFU_LOCK_HELPER=1",
		"SIAFU_LOCK_DIR="+dir,
		"SIAFU_LOCK_MARKER="+marker,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper process did not acquire the lock")
		}
		time.Sleep(10 * time.Millisecond)
	}

	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	begin := time.Now()
	if err := store.withLock(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(begin)
	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper process = %v", err)
	}
	if elapsed < 100*time.Millisecond {
		t.Fatalf("lock did not wait for the helper: %s", elapsed)
	}
}
