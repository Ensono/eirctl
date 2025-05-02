package config

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// ErrConfigNotFound occurs when requested config file does not exists
var ErrConfigNotFound = errors.New("config file not found")

// Loader reads and parses config files
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
	return cl.dst, nil
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

func (cl *Loader) reset() {
	cl.imports = make(map[string]bool)
}

// load is called with the base eirctl.yaml|json|toml files for the first time
// recursively called by other imports if they exist and their imports
func (cl *Loader) load(file string) (config *ConfigDefinition, err error) {
	// ensures a recursive forever loop does not occur and set file to visited
	cl.imports[file] = true

	if utils.IsURL(file) {
		config, err = cl.readURL(file)
	} else {
		if !utils.FileExists(file) {
			return config, fmt.Errorf("%s: %w", file, ErrConfigNotFound)
		}
		config, err = cl.readFile(file)
	}
	if err != nil {
		return nil, err
	}
	// the config will be cumulatively built over the imports
	// NOTE: we want to fail on duplicate keys detected
	if err := cl.parseImports(config, filepath.Dir(file)); err != nil {
		return nil, err
	}
	return config, nil
}

// parseImports map[string]any is a pointer for intents and purpose so we mutate it here with recursive merges
func (cl *Loader) parseImports(baseConfig *ConfigDefinition, importDir string) error {

	for _, val := range baseConfig.Import {
		// switch val := v.(type) {
		// case string:
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
		importFile := path.Join(importDir, val)
		if path.IsAbs(val) {
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
		if err := mergeExistingWithImported(baseConfig, importedConfig, val); err != nil {
			return err
		}
	}
	return nil
}

var ErrImportKeyClash = errors.New("imported file contains a clash")

// mergeExistingWithImported merges top level keys only and errors on duplicate tasks/pipelines/contexts via imports
//
// NOTE: perhaps changet this in the future or allow overwriting
func mergeExistingWithImported(baseConfig, importedConfig *ConfigDefinition, path string) error {
	// merge tasks - fail on
	for name, val := range importedConfig.Tasks {
		if baseConfig.Tasks == nil {
			baseConfig.Tasks = map[string]*TaskDefinition{}
		}
		if _, exists := baseConfig.Tasks[name]; exists {
			return fmt.Errorf("%w, file `%s` contains an already specified task (%s)", ErrImportKeyClash, path, name)
		}
		baseConfig.Tasks[name] = val
	}

	// merge pipelines - fail on
	for name, val := range importedConfig.Pipelines {
		if baseConfig.Pipelines == nil {
			baseConfig.Pipelines = map[string][]*PipelineDefinition{}
		}
		if _, exists := baseConfig.Pipelines[name]; exists {
			return fmt.Errorf("%w, file `%s` contains an already specified pipeline (%s)", ErrImportKeyClash, path, name)

		}
		baseConfig.Pipelines[name] = val
	}
	// merge contexts - fail on
	for name, val := range importedConfig.Contexts {
		if baseConfig.Contexts == nil {
			baseConfig.Contexts = map[string]*ContextDefinition{}
		}
		if _, exists := baseConfig.Contexts[name]; exists {
			return fmt.Errorf("%w, file `%s` contains an already specified context (%s)", ErrImportKeyClash, path, name)
		}
		baseConfig.Contexts[name] = val
	}
	return nil
}

func (cl *Loader) loadDir(dir string) (*ConfigDefinition, error) {
	// this is only going to work on yaml files
	// this program seems to want to accept json/toml and yaml
	//
	// TODO: remove json/toml support - unnecessary
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
