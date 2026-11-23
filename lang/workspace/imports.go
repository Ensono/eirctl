package workspace

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/Ensono/eirctl/internal/config"
)

const gitPrefix = "git::"

type ImportKind string

const (
	ImportKindLocal ImportKind = "local"
	ImportKindURL   ImportKind = "url"
	ImportKindGit   ImportKind = "git"
)

type ResolvedImport struct {
	Raw       string
	Kind      ImportKind
	Path      string
	FromCache bool
}

func ResolveImport(baseDir, homeDir, raw string) ResolvedImport {
	resolved := ResolvedImport{Raw: raw}

	switch {
	case strings.HasPrefix(raw, gitPrefix):
		resolved.Kind = ImportKindGit
		resolved.Path = config.GetCachePath(homeDir, raw)
		resolved.FromCache = true
	case isRemoteURL(raw):
		resolved.Kind = ImportKindURL
		resolved.Path = config.GetCachePath(homeDir, raw)
		resolved.FromCache = true
	case filepath.IsAbs(raw):
		resolved.Kind = ImportKindLocal
		resolved.Path = filepath.Clean(raw)
	default:
		resolved.Kind = ImportKindLocal
		resolved.Path = filepath.Clean(filepath.Join(baseDir, raw))
	}

	return resolved
}

func isRemoteURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}
