package cloudbuild

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func writeTarGz(w io.Writer, dir string) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Excludes for MVP
	exclude := func(rel string) bool {
		rel = filepath.ToSlash(rel)
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			return true
		}
		// node_modules is rebuilt by buildpacks; archiving it is slow and can break tar
		// extraction because of many symlinks from local package managers.
		if hasPathSegment(rel, "node_modules") {
			return true
		}
		if rel == "advncd" || strings.HasPrefix(rel, "advncd/") {
			return true
		}
		if rel == "bin" || strings.HasPrefix(rel, "bin/") {
			return true
		}
		return false
	}

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if exclude(rel) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		linkName := ""
		if info.Mode()&os.ModeSymlink != 0 {
			linkName, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		hdr, err := tar.FileInfoHeader(info, linkName)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		hdr.ModTime = time.Unix(0, 0) // stable-ish

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	})
}

// TarGzBytes archives dir to tar.gz bytes (MVP: in-memory).
func TarGzBytes(dir string) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeTarGz(&buf, dir); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func hasPathSegment(rel, segment string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, p := range parts {
		if p == segment {
			return true
		}
	}
	return false
}
