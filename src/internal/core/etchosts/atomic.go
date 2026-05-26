package etchosts

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// WriteEtcHosts atomically replaces `path` with `content`. Refuses to follow
// symlinks. Uses same-FS tempfile to avoid EXDEV. Clamps mode to 0o644 (no
// setuid/setgid/sticky/world-write). Best-effort chown to current uid/gid,
// swallowing only unix.EPERM. Tempfile is unlinked on every error exit
// including SIGINT/SIGTERM (via signal.Notify trap).
//
// Properties enforced (per .spikes/go-migration/p2-atomic-write/FINDINGS.md):
//  1. Symlink rejection — os.Lstat (NOT os.Stat) refuses to follow symlinks
//  2. Same-FS tempfile — os.CreateTemp(filepath.Dir(target), ...) guarantees no EXDEV
//  3. Mode clamp — mode & 0o777 & ^0o022 strips setuid/setgid/sticky/world-write
//  4. EPERM-only chown swallow — propagates all errors except unix.EPERM
//  5. Signal cleanup — signal.Notify(SIGINT, SIGTERM) + goroutine unlinks tempfile
func WriteEtcHosts(path, content string) error {
	// Property 1: lstat (NOT stat) — refuse to follow symlinks
	if fi, err := os.Lstat(path); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write through symlink: %s", path)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	dir := filepath.Dir(path)
	// Property 2: same-FS tempfile (NOT os.MkdirTemp("", ...) which may land in $TMPDIR on different mount)
	tmp, err := os.CreateTemp(dir, ".hostie-tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	// Property 5: signal cleanup — trap SIGINT/SIGTERM to unlink tempfile
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		_ = os.Remove(tmpPath)
		os.Exit(130)
	}()
	defer signal.Stop(sigCh)

	// Belt + suspenders: defer cleanup for panic path and normal error returns
	defer func() {
		// Only remove if tempfile still exists (rename success removes it)
		if _, statErr := os.Lstat(tmpPath); statErr == nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// Property 3: mode clamp — strip setuid/setgid/sticky/world-write
	const clampMask fs.FileMode = 0o777 &^ 0o022
	mode := fs.FileMode(0o644) & clampMask
	if err := os.Chmod(tmpPath, mode); err != nil {
		return err
	}

	// Property 4: chown — swallow ONLY unix.EPERM; propagate all other errors
	if err := os.Chown(tmpPath, os.Getuid(), os.Getgid()); err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) && errors.Is(pe.Err, unix.EPERM) {
			// swallow — running unprivileged, expected
		} else {
			return err
		}
	}

	// Atomic rename (same-FS guarantees no EXDEV)
	return os.Rename(tmpPath, path)
}
