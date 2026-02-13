package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// ErrConfigNotFound occurs when requested config file does not exists
var ErrConfigNotFound = errors.New("config file not found")

// MaxImportFileSize is the maximum size of an imported file (10MB)
const MaxImportFileSize = 10 * 1024 * 1024

// httpTimeout is the timeout for HTTP requests when fetching imported files
const httpTimeout = 30 * time.Second

// Loader reads and parses config files
//
// It recursively imports file/urls/git-paths via the import statement
type Loader struct {
	dst           *Config
	imports       map[string]bool
	dir           string
	homeDir       string
	strictDecoder bool
}

// NewConfigLoader is Loader constructor
func NewConfigLoader(dst *Config) Loader {
	return Loader{
		dst:           dst,
		imports:       make(map[string]bool),
		homeDir:       utils.MustGetUserHomeDir(),
		dir:           utils.MustGetwd(),
		strictDecoder: false,
	}
}

func (c *Loader) WithDir(dir string) *Loader {
	c.dir = dir
	return c
}

// Dir
func (c *Loader) Dir() string {
	return c.dir
}

func (c *Loader) WithStrictDecoder() *Loader {
	c.strictDecoder = true
	return c
}

type loaderContext struct {
	Dir string
}

// Load loads and parses requested config file
func (cl *Loader) Load(file string) (*Config, error) {
	cl.reset()
	lc := &loaderContext{
		Dir: cl.dir,
	}

	_, err := cl.LoadGlobalConfig()
	if err != nil {
		return nil, err
	}

	if file == "" {
		file, err = cl.ResolveDefaultConfigFile()
		if err != nil {
			return cl.dst, err
		}
	}

	if !utils.IsURL(file) && !filepath.IsAbs(file) {
		file = path.Join(cl.dir, file)
	}

	def, err := cl.load(file)
	if err != nil {
		return nil, err
	}

	// Process file import entries - fetch files from remote/local sources
	// and write them to the specified dest path
	if err := cl.processImportFiles(def); err != nil {
		return nil, err
	}

	localCfg, err := buildFromDefinition(def, lc)
	if err != nil {
		return nil, err
	}
	// Overwrite globally loaded config with
	// locally set config file
	err = cl.dst.merge(localCfg)
	if err != nil {
		return nil, err
	}
	cl.dst.Variables.Set("Root", cl.dir)

	logrus.Debugf("config %s loaded", file)
	cl.dst.SourceFile = file

	// validate config
	return cl.Validate()
}

// LoadGlobalConfig load global config file  - ~/.eirctl/config.yaml
func (cl *Loader) LoadGlobalConfig() (*Config, error) {
	if cl.homeDir == "" {
		return nil, nil
	}

	file := path.Join(cl.homeDir, ".eirctl", "config.yaml")
	if !utils.FileExists(file) {
		return cl.dst, nil
	}

	def, err := cl.load(file)
	if err != nil {
		return nil, err
	}

	cfg, err := buildFromDefinition(def, &loaderContext{})
	if err != nil {
		return nil, err
	}

	err = cl.dst.merge(cfg)
	if err != nil {
		return nil, err
	}

	return cl.dst, err
}

var ErrValidation = errors.New("validation failed")

// Validate checks the built config for any missed references
func (cl *Loader) Validate() (*Config, error) {
	// check tasks are correctly set up
	for _, task := range cl.dst.Tasks {
		// check referenced contexts - when declared, that they exist!
		if task.Context != "" {
			if _, ok := cl.dst.Contexts[task.Context]; !ok {
				return nil, fmt.Errorf("%w, task (%s) references missing context (%s)", ErrValidation, task.Name, task.Context)
			}
		}
	}
	return cl.dst, nil
}

func (cl *Loader) reset() {
	cl.imports = make(map[string]bool)
}

type ConfigFunc func(cl *Loader, file string) (bool, *ConfigDefinition, error)

// maintain the order of the slice as it's being loaded/validated
var getGetConfigFunc []ConfigFunc = []ConfigFunc{
	func(cl *Loader, file string) (bool, *ConfigDefinition, error) {
		if utils.IsURL(file) {
			logrus.Debugf("import (%s) is a URL", file)
			cfg, err := cl.readURL(file)
			return cfg != nil && err == nil, cfg, err
		}
		return false, nil, nil
	},
	func(cl *Loader, file string) (bool, *ConfigDefinition, error) {
		if IsGit(file) {
			logrus.Debugf("import (%s) is a git path", file)
			gs, err := NewGitSource(file)
			if err != nil {
				return false, nil, fmt.Errorf("%w\nerror: %v", ErrIncorrectlyFormattedGit, err)
			}
			if err := gs.Clone(); err != nil {
				return false, nil, err
			}
			cfg, err := gs.Config()
			return cfg != nil && err == nil, cfg, err
		}
		return false, nil, nil
	},
	func(cl *Loader, file string) (bool, *ConfigDefinition, error) {
		if !utils.FileExists(file) {
			return false, nil, fmt.Errorf("%s: %w", file, ErrConfigNotFound)
		}
		cfg, err := cl.readFile(file)
		return cfg != nil && err == nil, cfg, err
	},
}

// load is called with the base eirctl.yaml file for the first time
// recursively called by other imports if they exist and their imports
func (cl *Loader) load(file string) (*ConfigDefinition, error) {
	// ensures a recursive forever loop does not occur and set file to visited
	cl.imports[file] = true
	config := &ConfigDefinition{}
	for _, fn := range getGetConfigFunc {
		ok, cfg, err := fn(cl, file)
		if err != nil {
			return nil, err
		}
		if ok && cfg != nil {
			config = cfg
			break
		}
	}

	// The config will be cumulatively built over the imports
	// NOTE: we want to fail on duplicate keys detected
	if err := cl.parseImports(config, filepath.Dir(file)); err != nil {
		return nil, err
	}
	return config, nil
}

// parseImports map[string]any is a pointer for intents and purpose so we mutate it here with recursive merges
func (cl *Loader) parseImports(baseConfig *ConfigDefinition, importDir string) error {

	for _, entry := range baseConfig.Import {
		// Skip file imports — they are handled by processImportFiles
		if entry.IsFileImport() {
			continue
		}

		val := entry.Src

		if utils.IsURL(val) {
			if cl.imports[val] {
				// already visited and parsed import file
				// continuing to next
				continue
			}
			importedConfig, err := cl.load(val)
			if err != nil {
				return fmt.Errorf("load import error: %v", err)
			}
			if err := mergeExistingWithImported(baseConfig, importedConfig, val); err != nil {
				return err
			}
			// iterate through next import
			continue
		}

		if IsGit(val) {
			if cl.imports[val] {
				// already visited and parsed import file
				// continuing to next
				continue
			}
			importedConfig, err := cl.load(val)
			if err != nil {
				return fmt.Errorf("load import error: %v", err)
			}
			if err := mergeExistingWithImported(baseConfig, importedConfig, val); err != nil {
				return err
			}
			// iterate through next import
			continue
		}

		var importFile string

		// This is the value passed in if the loaded file was originally a URL
		// TODO: Need to talk through the intentions here, as this magically
		// worked for *nix systems but not Windows...
		if importDir != "http:" {
			importFile = filepath.Join(importDir, val)
		}

		if filepath.IsAbs(val) {
			importFile = val
		}
		if cl.imports[importFile] {
			continue
		}

		fi, err := os.Stat(importFile)
		if err != nil {
			return fmt.Errorf("%s: %v", importFile, err)
		}
		if fi.IsDir() {
			importedConfig, err := cl.loadDir(importFile)
			if err != nil {
				return fmt.Errorf("load import error: %v", err)
			}
			if err := mergeExistingWithImported(baseConfig, importedConfig, val); err != nil {
				return err
			}
			// iterate through next import
			continue
		}
		importedConfig, err := cl.load(importFile)
		if err != nil {
			return fmt.Errorf("load import error: %v", err)
		}
		if err := mergeExistingWithImported(baseConfig, importedConfig, importFile); err != nil {
			return err
		}
	}

	return nil
}

var ErrImportKeyClash = errors.New("imported file contains a clash")

// ErrImportFileFailed occurs when a file import entry fails to be fetched or written
var ErrImportFileFailed = errors.New("import file failed")

// ErrPathTraversal occurs when a dest path escapes the project root
var ErrPathTraversal = errors.New("dest path traverses outside project root")

// ErrHashMismatch occurs when a downloaded file's hash does not match the expected hash
var ErrHashMismatch = errors.New("integrity hash mismatch")

// ErrUnsupportedHashAlgorithm occurs when an unsupported hash algorithm is specified
var ErrUnsupportedHashAlgorithm = errors.New("unsupported hash algorithm")

// GetBaseFilename extracts the basename from a source path, stripping query parameters
// e.g. "path/file.sh?ref=v1.0" -> "file.sh"
func GetBaseFilename(src string) string {
	// For git URLs, strip the ?ref= part before getting basename
	if idx := strings.Index(src, "?"); idx != -1 {
		src = src[:idx]
	}
	return filepath.Base(src)
}

// IsDirectoryImport returns true if the src path indicates a directory import.
// A trailing slash on the file/directory path (after stripping query params) signals
// that the import target is a directory rather than a single file.
func IsDirectoryImport(src string) bool {
	// strip query params first
	clean := src
	if idx := strings.Index(clean, "?"); idx != -1 {
		clean = clean[:idx]
	}

	// For git URLs, check the path portion after //
	if IsGit(clean) {
		parts := strings.SplitN(clean, "//", 3)
		if len(parts) == 3 {
			return strings.HasSuffix(parts[2], "/")
		}
		return false
	}

	return strings.HasSuffix(clean, "/") || strings.HasSuffix(clean, string(filepath.Separator))
}

// processImportFiles processes file import entries from the import list,
// fetching file content from git/URL/local sources and optionally verifying
// integrity hashes. The dest field is required and specifies where files are
// written relative to the project root.
// A trailing slash on the src path signals a directory import — all files in the
// directory are fetched and written preserving their relative structure.
func (cl *Loader) processImportFiles(config *ConfigDefinition) error {
	var fileImports []ImportEntry

	for _, entry := range config.Import {
		if entry.IsFileImport() {
			fileImports = append(fileImports, entry)
		}
	}

	if len(fileImports) == 0 {
		return nil
	}

	for _, importFile := range fileImports {
		if importFile.Src == "" {
			return fmt.Errorf("%w: import entry has empty src", ErrImportFileFailed)
		}

		if IsDirectoryImport(importFile.Src) {
			if err := cl.processDirectoryImport(importFile); err != nil {
				return err
			}
			continue
		}

		content, err := cl.fetchFileContent(importFile.Src)
		if err != nil {
			return fmt.Errorf("%w: failed to fetch %s: %v", ErrImportFileFailed, importFile.Src, err)
		}

		// Verify hash if specified
		if importFile.Hash != "" {
			if err := verifyHash(content, importFile.Hash); err != nil {
				return fmt.Errorf("%w: %s", err, importFile.Src)
			}
		}

		if err := cl.writeImportedFile(importFile.Src, importFile.Dest, content); err != nil {
			return err
		}
	}

	return nil
}

// writeImportedFile writes a single imported file to the specified destination,
// applying path traversal protection. dest is required and relative to the project root.
func (cl *Loader) writeImportedFile(src, dest string, content []byte) error {
	destPath := filepath.Join(cl.dir, dest)

	// Guard against path traversal (e.g. dest: "../../.bashrc")
	cleanRoot := filepath.Clean(cl.dir) + string(filepath.Separator)
	cleanDest := filepath.Clean(destPath)
	if !strings.HasPrefix(cleanDest, cleanRoot) && cleanDest != filepath.Clean(cl.dir) {
		return fmt.Errorf("%w: %s resolves outside project root %s", ErrPathTraversal, dest, cl.dir)
	}

	// support nested dest paths by creating subdirectories
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("%w: failed to create directory for %s: %v", ErrImportFileFailed, destPath, err)
	}

	// write with executable permissions for scripts
	if err := os.WriteFile(destPath, content, 0755); err != nil {
		return fmt.Errorf("%w: failed to write %s: %v", ErrImportFileFailed, destPath, err)
	}
	logrus.Debugf("import: %s -> %s", src, destPath)
	return nil
}

// processDirectoryImport handles a single import entry that targets a directory.
// All files within the directory are fetched and written preserving relative paths.
// If a hash is specified, the canonical hash of the directory is verified.
func (cl *Loader) processDirectoryImport(importFile ImportEntry) error {
	files, err := cl.fetchDirContent(importFile.Src)
	if err != nil {
		return fmt.Errorf("%w: failed to fetch directory %s: %v", ErrImportFileFailed, importFile.Src, err)
	}

	// Verify directory hash if specified
	if importFile.Hash != "" {
		if err := verifyDirectoryHash(files, importFile.Hash); err != nil {
			return fmt.Errorf("%w: %s", err, importFile.Src)
		}
	}

	for relPath, content := range files {
		destPath := filepath.Join(importFile.Dest, relPath)

		if err := cl.writeImportedFile(importFile.Src, destPath, content); err != nil {
			return err
		}
	}

	return nil
}

// fetchFileContent retrieves the raw bytes of a file from a git, URL, or local source
func (cl *Loader) fetchFileContent(src string) ([]byte, error) {
	if IsGit(src) {
		gs, err := NewGitSource(src)
		if err != nil {
			return nil, fmt.Errorf("incorrectly formatted git source: %w", err)
		}
		if err := gs.Clone(); err != nil {
			return nil, fmt.Errorf("git clone failed: %w", err)
		}
		content, err := gs.FileContent()
		if err != nil {
			return nil, fmt.Errorf("git file retrieval failed: %w", err)
		}
		return content, nil
	}

	if utils.IsURL(src) {
		client := &http.Client{Timeout: httpTimeout}
		resp, err := client.Get(src) //nolint:gosec // src is user-provided in config
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%d: file request failed - %s", resp.StatusCode, src)
		}

		// Fail fast if Content-Length indicates the file is too large.
		if resp.ContentLength > 0 && resp.ContentLength > MaxImportFileSize {
			return nil, fmt.Errorf("remote file too large: %d bytes (max %d bytes)", resp.ContentLength, MaxImportFileSize)
		}

		// Use LimitReader at MaxImportFileSize+1 so we can detect truncation.
		data, err := io.ReadAll(io.LimitReader(resp.Body, MaxImportFileSize+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > MaxImportFileSize {
			return nil, fmt.Errorf("remote file too large: exceeds max allowed size of %d bytes", MaxImportFileSize)
		}

		return data, nil
	}

	// local filesystem
	localPath := src
	if !filepath.IsAbs(src) {
		localPath = filepath.Join(cl.dir, src)
	}

	if !utils.FileExists(localPath) {
		return nil, fmt.Errorf("%s: file not found", localPath)
	}

	return os.ReadFile(localPath)
}

// ErrDirImportNotSupported occurs when attempting to import a directory over HTTP
var ErrDirImportNotSupported = errors.New("directory import not supported over HTTP")

// fetchDirContent retrieves all files in a directory from a git or local source.
// Returns a map of relative file paths to their content.
// Directory import is not supported for plain URL sources.
func (cl *Loader) fetchDirContent(src string) (map[string][]byte, error) {
	if IsGit(src) {
		gs, err := NewGitSource(src)
		if err != nil {
			return nil, fmt.Errorf("incorrectly formatted git source: %w", err)
		}
		if err := gs.Clone(); err != nil {
			return nil, fmt.Errorf("git clone failed: %w", err)
		}
		files, err := gs.DirContent()
		if err != nil {
			return nil, fmt.Errorf("git directory retrieval failed: %w", err)
		}
		return files, nil
	}

	if utils.IsURL(src) {
		return nil, ErrDirImportNotSupported
	}

	// local filesystem directory
	localPath := src
	if !filepath.IsAbs(src) {
		localPath = filepath.Join(cl.dir, src)
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return nil, fmt.Errorf("%s: directory not found: %w", localPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s: not a directory", localPath)
	}

	result := make(map[string][]byte)
	err = filepath.WalkDir(localPath, func(filePath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(localPath, filePath)
		if err != nil {
			return fmt.Errorf("failed to compute relative path: %w", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		// normalise to forward slashes for consistency across platforms
		result[filepath.ToSlash(relPath)] = content
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("%s: directory is empty", localPath)
	}

	return result, nil
}

// verifyHash checks that the SHA-256 hash of content matches the expected hash.
// expectedHash must be in the format "algorithm:hex_digest", e.g.
// "sha256:236d1b7e77d01309bf704065c74b3e4baf589dac240a7b199c4c22e9fc4630e6"
// Currently only sha256 is supported.
func verifyHash(content []byte, expectedHash string) error {
	parts := strings.SplitN(expectedHash, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("%w: hash must be in format 'algorithm:hex_digest', got %q", ErrUnsupportedHashAlgorithm, expectedHash)
	}
	algorithm, expectedDigest := parts[0], parts[1]

	switch strings.ToLower(algorithm) {
	case "sha256":
		h := sha256.Sum256(content)
		actualDigest := hex.EncodeToString(h[:])
		if actualDigest != expectedDigest {
			return fmt.Errorf("%w: expected %s:%s, got %s:%s", ErrHashMismatch, algorithm, expectedDigest, algorithm, actualDigest)
		}
		return nil
	default:
		return fmt.Errorf("%w: %s (supported: sha256)", ErrUnsupportedHashAlgorithm, algorithm)
	}
}

// verifyDirectoryHash verifies the canonical hash of a directory import.
// Files are sorted lexicographically by path and concatenated as "<path>\n<content>"
// before hashing, ensuring a deterministic digest regardless of filesystem ordering.
func verifyDirectoryHash(files map[string][]byte, expectedHash string) error {
	// Sort file paths for canonical ordering
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	// Build canonical content: concatenate "<path>\n<content>" for each file
	h := sha256.New()
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte("\n"))
		h.Write(files[p])
	}

	parts := strings.SplitN(expectedHash, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("%w: hash must be in format 'algorithm:hex_digest', got %q", ErrUnsupportedHashAlgorithm, expectedHash)
	}
	algorithm, expectedDigest := parts[0], parts[1]

	switch strings.ToLower(algorithm) {
	case "sha256":
		actualDigest := hex.EncodeToString(h.Sum(nil))
		if actualDigest != expectedDigest {
			return fmt.Errorf("%w: expected %s:%s, got %s:%s", ErrHashMismatch, algorithm, expectedDigest, algorithm, actualDigest)
		}
		return nil
	default:
		return fmt.Errorf("%w: %s (supported: sha256)", ErrUnsupportedHashAlgorithm, algorithm)
	}
}

// mergeExistingWithImported merges top level keys only and errors on duplicate tasks/pipelines/contexts via imports
//
// NOTE: perhaps change this in the future or allow overwriting
func mergeExistingWithImported(baseConfig, importedConfig *ConfigDefinition, path string) error {
	// merge tasks - fail on already defined tasks
	for name, val := range importedConfig.Tasks {
		if baseConfig.Tasks == nil {
			baseConfig.Tasks = map[string]*TaskDefinition{}
		}
		if _, exists := baseConfig.Tasks[name]; exists {
			return fmt.Errorf("%w, file `%s` contains an already specified task (%s)", ErrImportKeyClash, path, name)
		}
		// enrich with source
		val.SourceFile = path
		baseConfig.Tasks[name] = val
	}

	// merge pipelines - fail on already defined pipelines
	for name, val := range importedConfig.Pipelines {
		if baseConfig.Pipelines == nil {
			baseConfig.Pipelines = map[string][]*PipelineDefinition{}
		}
		if _, exists := baseConfig.Pipelines[name]; exists {
			return fmt.Errorf("%w, file `%s` contains an already specified pipeline (%s)", ErrImportKeyClash, path, name)

		}
		// enrich with source
		enrichedPipelineDef := []*PipelineDefinition{}
		for _, p := range val {
			p.SourceFile = path
			enrichedPipelineDef = append(enrichedPipelineDef, p)
		}
		baseConfig.Pipelines[name] = enrichedPipelineDef
	}
	// merge contexts - fail on already defined contexts
	for name, val := range importedConfig.Contexts {
		if baseConfig.Contexts == nil {
			baseConfig.Contexts = map[string]*ContextDefinition{}
		}
		if _, exists := baseConfig.Contexts[name]; exists {
			return fmt.Errorf("%w, file `%s` contains an already specified context (%s)", ErrImportKeyClash, path, name)
		}
		// enrich with source
		val.SourceFile = path
		baseConfig.Contexts[name] = val
	}
	return nil
}

func (cl *Loader) loadDir(dir string) (*ConfigDefinition, error) {
	pattern := filepath.Join(dir, "*.yaml")
	q, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", dir, err)
	}

	cm := &ConfigDefinition{}
	for _, importFile := range q {
		if cl.imports[importFile] {
			continue
		}

		cml, err := cl.load(importFile)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", importFile, err)
		}
		if err := mergeExistingWithImported(cm, cml, importFile); err != nil {
			return nil, err
		}
	}

	return cm, nil
}

func (cl *Loader) readURL(urlStr string) (*ConfigDefinition, error) {
	resp, err := http.Get(urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%d: config request failed - %s", resp.StatusCode, urlStr)
	}
	cm := &ConfigDefinition{}
	if err := yaml.NewDecoder(resp.Body).Decode(&cm); err != nil {
		return nil, err
	}
	return cm, nil
}

func (cl *Loader) readFile(filename string) (*ConfigDefinition, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", filename, err)
	}
	cm := &ConfigDefinition{}
	if err := yaml.Unmarshal(data, &cm); err != nil {
		return nil, err
	}
	return cm, nil
}

func (cl *Loader) ResolveDefaultConfigFile() (file string, err error) {
	dir := cl.dir
	for dir != filepath.Dir(dir) {
		for _, v := range DefaultFileNames {
			file := filepath.Join(dir, v)
			if utils.FileExists(file) {
				cl.dir = dir
				return file, nil
			}
		}

		dir = filepath.Dir(dir)
	}

	return file, fmt.Errorf("default config resolution failed: %w", ErrConfigNotFound)
}
