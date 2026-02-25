package config

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/Ensono/eirctl/internal/schema"
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

// os implementation
//
// os.Create
// os.MkdirAll
// os.Stat
// os.Open
// os.ReadFile

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
// This only called from the command itself and would be initially pointing to the local eirctl.yaml
//
// NOTE: it then recursively builds a full config definition based on all the imports
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

	def, err := cl.load(schema.ImportEntry{Src: file})
	if err != nil {
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

	def, err := cl.load(schema.ImportEntry{Src: file})
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

type ConfigFunc func(cl *Loader, file schema.ImportEntry) (bool, *ConfigDefinition, error)

// maintain the order of the slice as it's being loaded/validated
var getGetConfigFunc []ConfigFunc = []ConfigFunc{
	func(cl *Loader, file schema.ImportEntry) (bool, *ConfigDefinition, error) {
		if utils.IsURL(file.Src) {
			logrus.Debugf("import (%s) is a URL", file)
			cfg, err := cl.readURL(file)
			return cfg != nil && err == nil, cfg, err
		}
		return false, nil, nil
	},
	func(cl *Loader, file schema.ImportEntry) (bool, *ConfigDefinition, error) {
		if IsGit(file.Src) {
			logrus.Debugf("import (%s) is a git path", file)
			gs, err := NewGitSource(file)
			if err != nil {
				return false, nil, fmt.Errorf("%w\nerror: %v", ErrIncorrectlyFormattedGit, err)
			}
			if err := gs.Clone(); err != nil {
				return false, nil, err
			}
			if file.IsFileImport() {
				contents, err := gs.File()
				if err != nil {
					return false, nil, err
				}
				if err := cl.writeImportedFile(file, contents); err != nil {
					return false, nil, err
				}
				return true, &ConfigDefinition{}, nil
			}
			cfg, err := gs.Config()
			return cfg != nil && err == nil, cfg, err
		}
		return false, nil, nil
	},
	func(cl *Loader, file schema.ImportEntry) (bool, *ConfigDefinition, error) {
		if !utils.FileExists(file.Src) {
			return false, nil, fmt.Errorf("%s: %w", file, ErrConfigNotFound)
		}
		cfg, err := cl.readFile(file)
		return cfg != nil && err == nil, cfg, err
	},
}

// load is called with the base eirctl.yaml file for the first time
// recursively called by other imports if they exist and their imports
func (cl *Loader) load(file schema.ImportEntry) (*ConfigDefinition, error) {
	// ensures a recursive forever loop does not occur and set file to visited
	cl.imports[file.Src] = true
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
	if err := cl.parseImports(config, filepath.Dir(file.Src)); err != nil {
		return nil, err
	}
	return config, nil
}

// parseImports map[string]any is a pointer for all intents and purpose so we mutate it here with recursive merges
//
// NOTE: we also write any file imports to disk (their `Dest` location)
func (cl *Loader) parseImports(baseConfig *ConfigDefinition, importDir string) error {

	for _, entry := range baseConfig.Import {
		// Handle URI resources first
		// continue to next when done or error
		if utils.IsURL(entry.Src) {
			if cl.imports[entry.Src] {
				// already visited and parsed import file
				// continuing to next
				continue
			}
			importedConfig, err := cl.load(entry)
			if err != nil {
				return fmt.Errorf("load import error: %v", err)
			}
			if err := mergeExistingWithImported(baseConfig, importedConfig, entry.Src); err != nil {
				return err
			}
			// iterate through next import
			continue
		}

		// Handle Git resources second
		// continue to next when done or error
		if IsGit(entry.Src) {
			if cl.imports[entry.Src] {
				// already visited and parsed import file
				// continuing to next
				continue
			}
			importedConfig, err := cl.load(entry)
			if err != nil {
				return fmt.Errorf("load import error: %v", err)
			}
			if err := mergeExistingWithImported(baseConfig, importedConfig, entry.Src); err != nil {
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
			importFile = filepath.Join(importDir, entry.Src)
		}

		if filepath.IsAbs(entry.Src) {
			importFile = entry.Src
		}
		// setting the Src to absolute path here
		entry.Src = importFile

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
			if err := mergeExistingWithImported(baseConfig, importedConfig, entry.Src); err != nil {
				return err
			}
			// iterate through next import
			continue
		}
		importedConfig, err := cl.load(entry)
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

		cml, err := cl.load(schema.ImportEntry{Src: importFile})
		if err != nil {
			return nil, fmt.Errorf("%s: %v", importFile, err)
		}
		if err := mergeExistingWithImported(cm, cml, importFile); err != nil {
			return nil, err
		}
	}

	return cm, nil
}

func (cl *Loader) readURL(urlStr schema.ImportEntry) (*ConfigDefinition, error) {
	resp, err := http.Get(urlStr.Src)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%d: config request failed - %s", resp.StatusCode, urlStr)
	}

	cm := &ConfigDefinition{}

	if urlStr.IsFileImport() {
		if err := cl.writeImportedFile(urlStr, resp.Body); err != nil {
			return nil, err
		}
		return cm, nil
	}

	if err := yaml.NewDecoder(resp.Body).Decode(&cm); err != nil {
		return nil, err
	}
	return cm, nil
}

func (cl *Loader) readFile(entry schema.ImportEntry) (*ConfigDefinition, error) {
	data, err := os.Open(entry.Src)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", entry.Src, err)
	}

	cm := &ConfigDefinition{}

	if entry.IsFileImport() {
		if err := cl.writeImportedFile(entry, data); err != nil {
			return nil, err
		}
		return cm, nil
	}

	if err := yaml.NewDecoder(data).Decode(cm); err != nil {
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
