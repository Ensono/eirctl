package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ensono/eirctl/internal/schema"
	"github.com/sirupsen/logrus"
)

// ErrImportFileFailed occurs when a file import entry fails to be fetched or written
var ErrImportFileFailed = errors.New("import file failed")

// ErrPathTraversal occurs when a dest path escapes the project root
var ErrPathTraversal = errors.New("dest path traverses outside project root")

// ErrHashMismatch occurs when a downloaded file's hash does not match the expected hash
var ErrHashMismatch = errors.New("integrity hash mismatch")

func StoreImportedFile(cl *Loader, entry schema.ImportEntry, content io.ReadCloser) error {
	return storeImportedFile(cl.Dir(), entry, content)
}

// writeImportedFile writes a single imported file to the specified destination,
// applying path traversal protection. dest is required and relative to the project root.
func (cl *Loader) writeImportedFile(entry schema.ImportEntry, content io.ReadCloser) error {
	defer content.Close()

	checkHash, h, err := entry.HasHash()
	if err != nil {
		return err
	}

	if checkHash {
		// bytes for hash checking and content writing
		hb, cb := &bytes.Buffer{}, &bytes.Buffer{}
		// Copying the reader into multiple writers
		if _, err := io.Copy(io.MultiWriter(hb, cb), content); err != nil {
			return err
		}
		if err := VerifyHash(hb.Bytes(), h.Typ, h.Val); err != nil {
			return err
		}
		return storeImportedFile(cl.Dir(), entry, cb)
	}
	return storeImportedFile(cl.Dir(), entry, content)
}

// storeImportedFiles writes files to dest
func storeImportedFile(baseDir string, entry schema.ImportEntry, content io.Reader) error {
	cleanDest := filepath.Clean(filepath.Join(baseDir, entry.Dest))
	if filepath.IsAbs(entry.Dest) {
		cleanDest = filepath.Clean(entry.Dest)
	}

	// keeping this here for now - but should be gated in the future as
	// there are genuine reasons writing outside of project root
	if !strings.HasPrefix(cleanDest, filepath.Clean(baseDir)) {
		return fmt.Errorf("%w: %s resolves outside project root %s", ErrPathTraversal, entry.Dest, baseDir)
	}

	// support nested dest paths by creating subdirectories
	if err := os.MkdirAll(filepath.Dir(cleanDest), 0755); err != nil {
		return fmt.Errorf("%w: failed to create directory for %s: %v", ErrImportFileFailed, cleanDest, err)
	}

	f, err := os.Create(cleanDest)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, content); err != nil {
		// copy error
		return err
	}
	logrus.Debugf("import: %s -> %s", entry.Src, cleanDest)
	return nil
}

// VerifyHash checks that the SHA-256 hash of content matches the expected hash.
// expectedHash must be in the format "algorithm:hex_digest", e.g.
// "sha256:236d1b7e77d01309bf704065c74b3e4baf589dac240a7b199c4c22e9fc4630e6"
// Currently only sha256 is supported.
func VerifyHash(content []byte, hashTyp schema.HashType, expectedDigest string) error {

	switch hashTyp {
	case schema.Sha256:
		h := sha256.Sum256(content)
		// h := sha256.New()
		// io.Copy()
		actualDigest := hex.EncodeToString(h[:])
		if actualDigest != expectedDigest {
			return fmt.Errorf("%w: expected %s:%s, got %s:%s", ErrHashMismatch, hashTyp, expectedDigest, hashTyp, actualDigest)
		}
		return nil
	default:
		return fmt.Errorf("%w: %s is unsupported (supported: sha256)", schema.ErrUnsupportedHashAlgorithm, hashTyp)
	}
}
