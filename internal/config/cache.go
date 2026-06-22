package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

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
}

type CacheOpt func(*Cache)

func NewCache() *Cache {
	return &Cache{
		fo: filesystemOps{},
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
// NB: the caller needs to pass in a multiplexed or teed reader in first as the stream will be emptied once read
func (c *Cache) Store(fullPath string, content io.Reader) error {
	w, err := c.createCacheWriter(fullPath)
	if err != nil {
		return err
	}

	// store in CACHE_DIR
	if _, err := io.Copy(w, content); err != nil {
		return fmt.Errorf("%w, %v", ErrCacheStreamCopyFailed, err)
	}

	return nil
}

// Get returns a successful io.Reader if content exists else an error
func (c *Cache) Get(file schema.ImportEntry) (*ConfigDefinition, error) {

	contents, err := c.fo.Open(getCachePath(file.Src))
	if err != nil {
		// custom error file not found
		// caller needs to handle creation and
		if perr, ok := errors.AsType[*fs.PathError](err); ok {
			logrus.Debugf("Cache File (%s) Not Found, should store in cache...", perr.Path)
			return nil, fmt.Errorf("%w, file: %s", ErrFileNotInCache, perr.Path)
		}
		return nil, fmt.Errorf("%w, error: %v", ErrFailedToGetFromCache, err)
	}
	// when import is a writeable file and exists in cache we copy it from there
	if file.IsFileImport() {
		importBuffer := &bytes.Buffer{}
		if _, err := io.Copy(importBuffer, contents); err != nil {
			return nil, fmt.Errorf("%w, %s", ErrCacheStreamCopyFailed, err)
		}
		if err := c.writeImport(file, io.NopCloser(importBuffer)); err != nil {
			return nil, fmt.Errorf("%w, import writer err: %v", ErrFailedToWriteImport, err)
		}
		return &ConfigDefinition{}, nil
	}

	cfgBytes, err := io.ReadAll(contents)
	if err != nil {
		return nil, fmt.Errorf("%w, err: %v", ErrFailedToReadFromCache, err)
	}

	cfg := &ConfigDefinition{}
	if err := yaml.Unmarshal(cfgBytes, cfg); err != nil {
		return nil, fmt.Errorf("%w, err: %v", ErrFailedToParse, err)
	}
	return cfg, nil
}

func (c *Cache) createCacheWriter(fullPath string) (io.Writer, error) {
	cp := getCachePath(fullPath)
	// Ensure the cache directory exists
	if err := c.fo.MkdirAll(filepath.Dir(cp), 0777); err != nil {
		return nil, fmt.Errorf("%w, %s", ErrCacheDirCreationFailed, err)
	}
	f, err := c.fo.Create(cp)
	if err != nil {
		return nil, fmt.Errorf("%w, %s", ErrCacheWriterFailed, err)
	}
	return f, nil
}

// getCachePath always returns to path to the file in the cache directory
func getCachePath(fullPath string) string {
	return filepath.Join(utils.MustGetUserHomeDir(), ".eirctl", "cache", utils.EncodeBase62(fullPath))
}
