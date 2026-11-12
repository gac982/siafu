// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package memguard

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"
)

type testLogger struct {
	warns, errors int
}

func (l *testLogger) Warn(string)  { l.warns++ }
func (l *testLogger) Error(string) { l.errors++ }

func TestCheckThresholds(t *testing.T) {
	w := New(300, 80, &testLogger{}, nil)
	if warn, hard := w.Check(239); warn || hard {
		t.Fatalf("239: warn=%v hard=%v", warn, hard)
	}
	if warn, hard := w.Check(240); !warn || hard {
		t.Fatalf("240: warn=%v hard=%v", warn, hard)
	}
	if warn, hard := w.Check(301); warn || hard {
		t.Fatalf("301 first: warn=%v hard=%v", warn, hard)
	}
	if warn, hard := w.Check(301); warn || !hard {
		t.Fatalf("301 second: warn=%v hard=%v", warn, hard)
	}
}

func TestWatcherCallsOnHard(t *testing.T) {
	logger := &testLogger{}
	hard := make(chan struct{}, 1)
	w := New(300, 80, logger, func() { hard <- struct{}{} })
	w.sample = func() (int, error) { return 400, nil }
	w.interval = 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	select {
	case <-hard:
	case <-time.After(time.Second):
		t.Fatal("onHard was not called")
	}
	if logger.errors == 0 {
		t.Fatal("hard limit was not logged")
	}
}

func TestSampleRSS(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("process RSS is only tested on linux and darwin")
	}
	mb, err := SampleRSSMB()
	if err != nil {
		t.Fatal(err)
	}
	if mb <= 0 {
		t.Fatalf("RSS = %d MiB", mb)
	}
}

func TestWatcherSampleError(t *testing.T) {
	logger := &testLogger{}
	w := New(300, 80, logger, nil)
	w.sample = func() (int, error) { return 0, fmt.Errorf("sample failed") }
	w.interval = 10 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	w.Run(ctx)
	if logger.warns == 0 {
		t.Fatal("sample error was not logged")
	}
}
