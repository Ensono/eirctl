package config

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// ErrConfigNotFound occurs when requested config file does not exists
var ErrConfigNotFound = errors.New("config file not found")

// EirctlScriptsDir is the directory where imported files are placed
const EirctlScriptsDir = ".eirctl/scripts"

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

	// Process import_files entries - fetch files from remote/local sources
	// and write them to .eirctl/scripts/ directory
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

	for _, val := range baseConfig.Import {
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

// ErrImportFileFailed occurs when an import_files entry fails to be fetched or written
var ErrImportFileFailed = errors.New("import file failed")

// GetBaseFilename extracts the basename from a source path, stripping query parameters
// e.g. "path/file.sh?ref=v1.0" -> "file.sh"
func GetBaseFilename(src string) string {
	// For git URLs, strip the ?ref= part before getting basename
	if idx := strings.Index(src, "?"); idx != -1 {
		src = src[:idx]
	}
	return filepath.Base(src)
}

// processImportFiles processes the import_files entries from the config definition
// fetching file content from git/URL/local sources.
// When dest is specified, files are written relative to the project root.
// When dest is omitted, files are written to .eirctl/scripts/<basename>.
func (cl *Loader) processImportFiles(config *ConfigDefinition) error {
	if len(config.ImportFiles) == 0 {
		return nil
	}

	for _, importFile := range config.ImportFiles {
		if importFile.Src == "" {
			return fmt.Errorf("%w: import_files entry has empty src", ErrImportFileFailed)
		}

		content, err := cl.fetchFileContent(importFile.Src)
		if err != nil {
			return fmt.Errorf("%w: failed to fetch %s: %v", ErrImportFileFailed, importFile.Src, err)
		}

		var destPath string
		if importFile.Dest != "" {
			// explicit dest is relative to the project root
			destPath = filepath.Join(cl.dir, importFile.Dest)
		} else {
			// default to .eirctl/scripts/<basename> (strip query params)
			destPath = filepath.Join(cl.dir, EirctlScriptsDir, GetBaseFilename(importFile.Src))
		}

		// support nested dest paths by creating subdirectories
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("%w: failed to create directory for %s: %v", ErrImportFileFailed, destPath, err)
		}

		// write with executable permissions for scripts
		if err := os.WriteFile(destPath, content, 0755); err != nil {
			return fmt.Errorf("%w: failed to write %s: %v", ErrImportFileFailed, destPath, err)
		}
		logrus.Debugf("import_files: %s -> %s", importFile.Src, destPath)
	}

	return nil
}

// fetchFileContent retrieves the raw bytes of a file from a git, URL, or local source
func (cl *Loader) fetchFileContent(src string) ([]byte, error) {
	if IsGit(src) {
		gs, err := NewGitSource(src)
		if err != nil {
			return nil, fmt.Errorf("%w\nerror: %v", ErrIncorrectlyFormattedGit, err)
		}
		if err := gs.Clone(); err != nil {
			return nil, err
		}
		return gs.FileContent()
	}

	if utils.IsURL(src) {
		resp, err := http.Get(src)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%d: file request failed - %s", resp.StatusCode, src)
		}
		return io.ReadAll(resp.Body)
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
