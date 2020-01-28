package lazymove

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// DefaultTimeout is the default value for Mover.Timeout
	DefaultTimeout = 5 * time.Minute
	// DefaultMinFileAge is the default value for Mover.MinFileAge
	DefaultMinFileAge = 5 * time.Minute
	// DefaultMinDirAge is the default value for Mover.MinDirAge
	DefaultMinDirAge = time.Hour
)

// DefaultErrorFunc is the default function for Mover.ErrorFunc,
// it logs to log.Printf and returns resume=true.
func DefaultErrorFunc(m *Mover, err error) (resume bool) {
	log.Printf("Error while moving files: %v", err)
	return true
}

// Mover will lazily move files from SourceDir into DestDir.
// It will do this iteration each Timeout,
// only moving files with modification times older than MinFileAge,
// and only removing empty directories with times older than MinDirAge.
// ErrorFunc is called on any errors that occur during a move,
// true can be returned to resume moving, or false to abort.
// If ErrorFunc returns false, it aborts the rest of the move iteration, and
// it is called again with a *MoveAbortedError, in this case the return value
// controls whether Run will resume or return.
// Behavior is undefined if SourceDir refers to the same location as DestDir.
// Do not modify fields after calling Run.
type Mover struct {
	SourceDir  string
	DestDir    string
	Timeout    time.Duration                     // default is DefaultTimeout
	MinFileAge time.Duration                     // default is DefaultMinFileAge
	MinDirAge  time.Duration                     // default is DefaultMinDirAge
	ErrorFunc  func(*Mover, error) (resume bool) // default is DefaultErrorFunc
	running    bool
}

// Run the mover.
// Returns if the ctx is done, or ErrorFunc says so (see Mover)
func (m *Mover) Run(ctx context.Context) error {
	if m.running {
		return errors.New("already running")
	}
	m.running = true
	defer func() { m.running = false }()

	if m.SourceDir == "" {
		panic("empty SourceDir")
	}
	if m.DestDir == "" {
		panic("empty DestDir")
	}

	if m.Timeout <= 0 {
		m.Timeout = DefaultTimeout
	}
	if m.MinFileAge <= 0 {
		m.MinFileAge = DefaultMinFileAge
	}
	if m.MinDirAge <= 0 {
		m.MinDirAge = DefaultMinDirAge
	}
	if m.ErrorFunc == nil {
		m.ErrorFunc = DefaultErrorFunc
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.Timeout):
			err := m.runIter(ctx)
			if err != nil {
				err = &MoveAbortedError{m, err}
				if !m.ErrorFunc(m, err) {
					return err
				}
			}
		}
	}
}

// Do a single iteration.
func (m *Mover) runIter(ctx context.Context) error {
	dirsBefore := time.Now().Add(-m.MinDirAge)
	filesBefore := time.Now().Add(-m.MinFileAge)
	type ent struct {
		info os.FileInfo
		path string
	}
	var dirs []ent
	var files []ent
	isFirst := true
	err := filepath.Walk(m.SourceDir, func(path string, info os.FileInfo, err error) error {
		if isFirst {
			// Ignore the dir itself, don't want to delete the sourceDir.
			isFirst = false
			return nil
		}
		if info.IsDir() {
			if info.ModTime().Before(dirsBefore) {
				dirs = append(dirs, ent{info, path})
			}
		} else {
			if info.ModTime().Before(filesBefore) {
				files = append(files, ent{info, path})
			}
		}
		return nil
	})
	if err != nil {
		err = fmt.Errorf("while listing SourceDir: %v", err)
		if !m.ErrorFunc(m, err) {
			return err
		}
	}
	if len(dirs)+len(files) == 0 {
		return nil
	}

	// Move these old files.
	for _, fe := range files {
		subpath := strings.TrimPrefix(fe.path, m.SourceDir)
		newpath := filepath.Join(m.DestDir, subpath)
		err = func() error { // Move the file:
			err := os.MkdirAll(filepath.Dir(newpath), 0751)
			if err != nil {
				return err
			}
			//fmode := 0640
			fmode := fe.info.Mode() // use original mode
			fout, err := os.OpenFile(newpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fmode)
			if err != nil {
				return err
			}
			moved := false
			defer func() {
				fout.Close()
				if !moved { // Clean up newpath if not fully moved.
					os.Remove(newpath)
				}
			}()
			fin, err := os.Open(fe.path)
			if err != nil {
				return err
			}
			defer fin.Close()
			// Copy contents:
			nwrote, err := io.Copy(fout, fin)
			if err != nil {
				return err
			}
			if nwrote != fe.info.Size() {
				// Fail if didn't write the full expected amount.
				// Also fail if it wrote more, as it means there's new activity.
				return errors.New("did not write expected byte count to " + newpath)
			}
			err = fout.Close()
			if err != nil {
				return err
			}
			fout.Sync()
			// Remove the original file:
			err = os.Remove(fe.path)
			if err != nil {
				return err
			}
			moved = true
			return nil
		}()
		if err != nil {
			err = fmt.Errorf("while moving file to DestDir: %v", err)
			if !m.ErrorFunc(m, err) {
				return err
			}
		}
	}

	// Sort dirs by length, longest first:
	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[j].path) < len(dirs[i].path)
	})
	// Now attempt to delete all these old dirs, longest paths first.
	// Failures are not critical in case the dir is not empty.
	for _, de := range dirs {
		if err := os.Remove(de.path); err != nil {
			log.Printf("INFO dir remove from DestDir: %v", err)
		}
	}

	return nil
}

// MoveAbortedError is used with Mover.ErrorFunc,
// see Mover for more info.
// Mover is the *Mover that was aborted,
// and Err is the error that aborted the interation.
// Note that this intentionally does not Unwrap Err.
type MoveAbortedError struct {
	Mover *Mover
	Err   error // what caused the abort
}

func (err *MoveAbortedError) Error() string {
	return "move iteration aborted"
}
