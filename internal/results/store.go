// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package results

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const (
	maxScanLine = 1 << 20
	indexFile   = "index.jsonl"
	recordFile  = "run.json"
	seriesFile  = "series.jsonl"
	summaryFile = "summary.txt"
)

var (
	idPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	storeLocks sync.Map
)

// Store persists run records under Dir.
type Store struct {
	Dir     string
	Now     func() time.Time
	Rand    io.Reader
	Sync    func(*os.File) error
	SyncDir func(string) error
	Warn    func(string)
}

// OpenStore opens a results directory without creating it.
func OpenStore(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("directorio de resultados vacío")
	}
	return &Store{
		Dir:     dir,
		Now:     time.Now,
		Rand:    rand.Reader,
		Sync:    func(file *os.File) error { return file.Sync() },
		SyncDir: syncDir,
	}, nil
}

// NewStore creates a store and ensures its directory exists.
func NewStore(dir string) (*Store, error) {
	store, err := OpenStore(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("crear directorio de resultados: %w", err)
	}
	return store, nil
}

// ValidID reports whether id is a safe run identifier.
func ValidID(id string) bool {
	if id == "" || id == "." || id == ".." || strings.Contains(id, "..") {
		return false
	}
	if strings.ContainsAny(id, `/\`) {
		return false
	}
	return idPattern.MatchString(id)
}

// Save writes the record and series atomically and appends the index entry.
func (s *Store) Save(rec RunRecord, series []SeriesPoint, summary string) (RunRecord, error) {
	if s == nil {
		return rec, errors.New("store nil")
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.Rand == nil {
		s.Rand = rand.Reader
	}
	rec.SchemaVersion = SchemaVersion
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = s.Now().UTC()
	} else {
		rec.CreatedAt = rec.CreatedAt.UTC()
	}
	if rec.RunID == "" {
		id, err := s.newID(rec)
		if err != nil {
			return rec, err
		}
		rec.RunID = id
	}
	if !ValidID(rec.RunID) {
		return rec, fmt.Errorf("run id inválido: %q", rec.RunID)
	}

	finalDir := filepath.Join(s.Dir, rec.RunID)
	if _, err := os.Stat(finalDir); err == nil {
		return rec, fmt.Errorf("la corrida %s ya existe", rec.RunID)
	}
	tmpDir := filepath.Join(s.Dir, "."+rec.RunID+".tmp")
	if err := os.Mkdir(tmpDir, 0o700); err != nil {
		return rec, fmt.Errorf("crear directorio temporal: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	runData, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return rec, fmt.Errorf("serializar run.json: %w", err)
	}
	if err := writeSynced(filepath.Join(tmpDir, recordFile), append(runData, '\n'), s.Sync); err != nil {
		return rec, err
	}

	var seriesData bytes.Buffer
	for _, point := range series {
		line, err := json.Marshal(point)
		if err != nil {
			return rec, fmt.Errorf("serializar serie: %w", err)
		}
		seriesData.Write(line)
		seriesData.WriteByte('\n')
	}
	if err := writeSynced(filepath.Join(tmpDir, seriesFile), seriesData.Bytes(), s.Sync); err != nil {
		return rec, err
	}
	if err := writeSynced(filepath.Join(tmpDir, summaryFile), []byte(summary), s.Sync); err != nil {
		return rec, err
	}
	if err := s.syncDirectory(tmpDir); err != nil {
		return rec, fmt.Errorf("sincronizar directorio temporal: %w", err)
	}

	if err := s.withLock(func() error {
		if err := os.Rename(tmpDir, finalDir); err != nil {
			return fmt.Errorf("publicar corrida: %w", err)
		}
		cleanup = false
		if err := s.syncDirectory(s.Dir); err != nil {
			return fmt.Errorf("sincronizar directorio de resultados: %w", err)
		}
		return s.appendIndex(rec.IndexEntry())
	}); err != nil {
		return rec, err
	}
	return rec, nil
}

// List returns the index entries, rebuilding them from run directories if the
// index is missing or corrupt.
func (s *Store) List() ([]IndexEntry, error) {
	entries, err := s.readIndex()
	if err == nil && len(entries) > 0 {
		ids, idsErr := s.runIDs()
		if idsErr == nil && sameIDSet(entries, ids) {
			sortEntries(entries)
			return entries, nil
		}
	}
	return s.scan()
}

// Load reads one run record.
func (s *Store) Load(id string) (RunRecord, error) {
	if !ValidID(id) {
		return RunRecord{}, fmt.Errorf("run id inválido: %q", id)
	}
	data, err := os.ReadFile(filepath.Join(s.Dir, id, recordFile))
	if err != nil {
		return RunRecord{}, err
	}
	var rec RunRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return rec, fmt.Errorf("leer %s: %w", id, err)
	}
	if rec.SchemaVersion != SchemaVersion {
		return rec, fmt.Errorf("schema_version %d no soportada (esperada %d)", rec.SchemaVersion, SchemaVersion)
	}
	if rec.RunID != id {
		return rec, fmt.Errorf("run.json pertenece a %q, no a %q", rec.RunID, id)
	}
	return rec, nil
}

// Summary reads the stored plain-text report of one run.
func (s *Store) Summary(id string) (string, error) {
	if !ValidID(id) {
		return "", fmt.Errorf("run id inválido: %q", id)
	}
	data, err := os.ReadFile(filepath.Join(s.Dir, id, summaryFile))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Series reads the per-second points of one run.
func (s *Store) Series(id string) ([]SeriesPoint, error) {
	if !ValidID(id) {
		return nil, fmt.Errorf("run id inválido: %q", id)
	}
	file, err := os.Open(filepath.Join(s.Dir, id, seriesFile))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var points []SeriesPoint
	scanner := newStoreScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var point SeriesPoint
		if err := json.Unmarshal([]byte(line), &point); err != nil {
			return nil, fmt.Errorf("leer serie de %s: %w", id, err)
		}
		points = append(points, point)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

// Prune keeps the newest keep runs and removes the rest.
func (s *Store) Prune(keep int) (int, error) {
	if keep < 0 {
		return 0, errors.New("keep debe ser mayor o igual que 0")
	}
	if _, err := os.Stat(s.Dir); os.IsNotExist(err) {
		return 0, nil
	}
	removed := 0
	err := s.withLock(func() error {
		entries, err := s.List()
		if err != nil {
			return err
		}
		if keep > len(entries) {
			keep = len(entries)
		}
		ids := make([]string, 0, len(entries)-keep)
		for _, entry := range entries[keep:] {
			ids = append(ids, entry.RunID)
		}
		var pruneErr error
		removed, pruneErr = s.pruneIDsLocked(ids)
		return pruneErr
	})
	return removed, err
}

// PruneIDs removes exactly the runs whose IDs are listed. IDs are validated and
// deduplicated; missing runs are ignored.
func (s *Store) PruneIDs(ids []string) (int, error) {
	if _, err := os.Stat(s.Dir); os.IsNotExist(err) {
		return 0, nil
	}
	removed := 0
	err := s.withLock(func() error {
		var pruneErr error
		removed, pruneErr = s.pruneIDsLocked(ids)
		return pruneErr
	})
	return removed, err
}

func (s *Store) pruneIDsLocked(ids []string) (int, error) {
	seen := make(map[string]bool, len(ids))
	removed := 0
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if !ValidID(id) {
			return removed, fmt.Errorf("run_id inválido en el índice: %q", id)
		}
		dir := filepath.Clean(filepath.Join(s.Dir, id))
		root := filepath.Clean(s.Dir)
		if dir == root || filepath.Dir(dir) != root {
			return removed, fmt.Errorf("ruta de corrida inválida: %q", id)
		}
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, s.reindexLocked()
}

// Reindex rebuilds index.jsonl from the run directories.
func (s *Store) Reindex() error {
	return s.withLock(s.reindexLocked)
}

func (s *Store) reindexLocked() error {
	entries, err := s.scan()
	if err != nil {
		return err
	}
	var data bytes.Buffer
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		data.Write(line)
		data.WriteByte('\n')
	}
	tmp := filepath.Join(s.Dir, indexFile+".tmp")
	if err := writeSynced(tmp, data.Bytes(), s.Sync); err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(s.Dir, indexFile)); err != nil {
		return err
	}
	return s.syncDirectory(s.Dir)
}

func (s *Store) withLock(fn func() error) error {
	mutex, _ := storeLocks.LoadOrStore(s.Dir, &sync.Mutex{})
	mu := mutex.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	file, err := os.OpenFile(filepath.Join(s.Dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("abrir lock: %w", err)
	}
	defer file.Close()
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("bloquear resultados: %w", err)
	}
	defer func() { _ = unix.Flock(int(file.Fd()), unix.LOCK_UN) }()
	return fn()
}

func (s *Store) newID(rec RunRecord) (string, error) {
	slug := slugify(rec.Name)
	for attempt := 0; attempt < 10; attempt++ {
		var suffix [8]byte
		if _, err := io.ReadFull(s.Rand, suffix[:]); err != nil {
			return "", fmt.Errorf("generar id: %w", err)
		}
		id := fmt.Sprintf("%s-%s-%x", rec.CreatedAt.UTC().Format("20060102-150405"), slug, suffix)
		if _, err := os.Stat(filepath.Join(s.Dir, id)); os.IsNotExist(err) {
			return id, nil
		}
	}
	return "", errors.New("no se pudo generar un id único")
}

func (s *Store) readIndex() ([]IndexEntry, error) {
	file, err := os.Open(filepath.Join(s.Dir, indexFile))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []IndexEntry
	seen := map[string]bool{}
	scanner := newStoreScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry IndexEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, err
		}
		if !ValidID(entry.RunID) {
			return nil, fmt.Errorf("índice inválido: run_id %q", entry.RunID)
		}
		if seen[entry.RunID] {
			continue
		}
		seen[entry.RunID] = true
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

func (s *Store) scan() ([]IndexEntry, error) {
	dirEntries, err := os.ReadDir(s.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []IndexEntry
	for _, entry := range dirEntries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if !ValidID(entry.Name()) {
			continue
		}
		rec, err := s.Load(entry.Name())
		if err != nil {
			s.warnf("corrida ignorada %s: %v", entry.Name(), err)
			continue
		}
		entries = append(entries, rec.IndexEntry())
	}
	sortEntries(entries)
	return entries, nil
}

func (s *Store) appendIndex(entry IndexEntry) error {
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("serializar índice: %w", err)
	}
	file, err := os.OpenFile(filepath.Join(s.Dir, indexFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("abrir índice: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("escribir índice: %w", err)
	}
	if s.Sync != nil {
		if err := s.Sync(file); err != nil {
			return fmt.Errorf("sincronizar índice: %w", err)
		}
	}
	return nil
}

func sortEntries(entries []IndexEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].CreatedAt.Equal(entries[j].CreatedAt) {
			return entries[i].RunID < entries[j].RunID
		}
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})
}

func slugify(name string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			builder.WriteRune(r)
			lastDash = false
		case builder.Len() > 0 && !lastDash:
			builder.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if len(slug) > 40 {
		slug = strings.TrimRight(slug[:40], "-")
	}
	if slug == "" {
		return "run"
	}
	return slug
}

func writeSynced(path string, data []byte, sync func(*os.File) error) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("escribir %s: %w", filepath.Base(path), err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("escribir %s: %w", filepath.Base(path), err)
	}
	if sync != nil {
		if err := sync(file); err != nil {
			_ = file.Close()
			return fmt.Errorf("sincronizar %s: %w", filepath.Base(path), err)
		}
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("cerrar %s: %w", filepath.Base(path), err)
	}
	return nil
}

func (s *Store) syncDirectory(dir string) error {
	if s.SyncDir == nil {
		return nil
	}
	return s.SyncDir(dir)
}

func syncDir(dir string) error {
	file, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := file.Sync(); err != nil && !errors.Is(err, syscall.EINVAL) && !errors.Is(err, syscall.ENOTSUP) {
		return err
	}
	return nil
}

func (s *Store) runIDs() (map[string]bool, error) {
	dirEntries, err := os.ReadDir(s.Dir)
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool)
	for _, entry := range dirEntries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && ValidID(entry.Name()) {
			ids[entry.Name()] = true
		}
	}
	return ids, nil
}

func sameIDSet(entries []IndexEntry, ids map[string]bool) bool {
	if len(entries) != len(ids) {
		return false
	}
	for _, entry := range entries {
		if !ids[entry.RunID] {
			return false
		}
	}
	return true
}

func newStoreScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), maxScanLine)
	return scanner
}

func (s *Store) warnf(format string, args ...any) {
	if s.Warn != nil {
		s.Warn(fmt.Sprintf(format, args...))
	}
}
