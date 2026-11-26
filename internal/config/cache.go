package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ensono/eirctl/internal/schema"
	"github.com/Ensono/eirctl/internal/utils"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var (
	ErrCacheWriterFailed      = errors.New("failed to create cache writer")
	ErrCacheDirCreationFailed = errors.New("failed to create cache dir")
	ErrCacheStreamCopyFailed  = errors.New("failed to copy buffer for cache")
	ErrFailedToGetFromCache   = errors.New("failed to get from cache")
	ErrFailedToReadFromCache  = errors.New("failed to read from cache location")
	ErrFailedToParse          = errors.New("failed to unmarshal from stream")
	ErrFileNotInCache         = errors.New("file does not exist in cache")
	ErrFailedToWriteImport    = errors.New("failed to write import")
)

type fsOps interface {
	MkdirAll(path string, perm os.FileMode) error
	Create(name string) (io.Writer, error)
	Open(name string) (io.Reader, error)
}

type filesystemOps struct{}

func (f filesystemOps) Create(name string) (io.Writer, error) {
	return os.Create(name)
}

func (f filesystemOps) Open(name string) (io.Reader, error) {
	return os.Open(name)
}

func (f filesystemOps) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Cache struct manages the storage and retrieval of configuration items/import files
// in a globally accessible cache directory
type Cache struct {
	fo          fsOps
	writeImport func(entry schema.ImportEntry, content io.ReadCloser) error
	homedir     string
}

type CacheOpt func(*Cache)

func NewCache(homedir string) *Cache {
	return &Cache{
		fo:      filesystemOps{},
		homedir: homedir,
	}
}

func (c *Cache) WithFsOps(fo fsOps) *Cache {
	c.fo = fo
	return c
}

func (c *Cache) WithWriteImport(wi func(entry schema.ImportEntry, content io.ReadCloser) error) *Cache {
	c.writeImport = wi
	return c
}

// Store creates a cache entry in the provided path in base62 encoded string for the path
// writes the contents from io.Reader by copying the reader
func (c *Cache) Store(fullPath string, content io.Reader) error {
	w, err := c.createCacheWriter(fullPath)
	if err != nil {
		return err
	}

	// store in CACHE_DIR
	_, copyErr := io.Copy(w, content)
	if closer, ok := w.(io.Closer); ok {
		copyErr = errors.Join(copyErr, closer.Close())
	}
	if copyErr != nil {
		return fmt.Errorf("%w: %w", ErrCacheStreamCopyFailed, copyErr)
	}

	return nil
}

// Get returns a successful io.Reader if content exists else an error
func (c *Cache) Get(file schema.ImportEntry) (*ConfigDefinition, error) {

	contents, err := c.fo.Open(GetCachePath(c.homedir, file.Src))
	if err != nil {
		// custom error file not found
		// caller needs to handle creation and subsequent content update
		if errors.Is(err, fs.ErrNotExist) {
			logrus.Debugf("Cache file not found for %s", file.Src)
			return nil, fmt.Errorf("%w, file: %s", ErrFileNotInCache, file.Src)
		}
		return nil, fmt.Errorf("%w, error: %v", ErrFailedToGetFromCache, err)
	}
	if closer, ok := contents.(io.Closer); ok {
		defer closer.Close()
	}
	data, err := readImportBytes(file, contents)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToReadFromCache, err)
	}
	if file.IsFileImport() {
		if err := c.writeImport(file, io.NopCloser(bytes.NewReader(data))); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFailedToWriteImport, err)
		}
		return &ConfigDefinition{}, nil
	}

	cfg := &ConfigDefinition{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("%w, err: %v", ErrFailedToParse, err)
	}
	return cfg, nil
}

func (c *Cache) createCacheWriter(fullPath string) (io.Writer, error) {
	cp := GetCachePath(c.homedir, fullPath)
	// Ensure the cache directory exists
	if err := c.fo.MkdirAll(filepath.Dir(cp), 0744); err != nil {
		return nil, fmt.Errorf("%w, %s", ErrCacheDirCreationFailed, err)
	}
	f, err := c.fo.Create(cp)
	if err != nil {
		return nil, fmt.Errorf("%w, %s", ErrCacheWriterFailed, err)
	}
	return f, nil
}

// GetCachePath always returns to path to the file in the cache directory
func GetCachePath(homedir, fullPath string) string {
	// Older remote entries may have been fetched over an unverified transport.
	if utils.IsURL(fullPath) || (IsGit(fullPath) && !strings.HasPrefix(fullPath, "git::file:")) {
		return filepath.Join(homedir, ".eirctl", "cache", "secure-transport-v1", utils.EncodeBase62(fullPath))
	}
	return filepath.Join(homedir, ".eirctl", "cache", utils.EncodeBase62(fullPath))
}
