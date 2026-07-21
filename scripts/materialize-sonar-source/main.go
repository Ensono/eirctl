// Command materialize-sonar-source retrieves a bounded set of passive Go source
// files from immutable GitHub Git Data API objects. It is intended to run from
// protected default-branch code in the trusted SonarCloud PR analyzer.
package main

import (
	"context"
	"crypto/sha1" //nolint:gosec // Git object identities are SHA-1 by protocol.
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxTreeEntries = 384
	maxGoFiles     = 160
	maxPathBytes   = 160
	maxFileBytes   = 128 * 1024
	maxTotalBytes  = 1024 * 1024

	reviewedBounds = "tree=384,go=160,path=160,file=131072,total=1048576"
	maxAPIResponse = 16 * 1024 * 1024
)

var (
	fullSHA       = regexp.MustCompile(`^[0-9a-f]{40}$`)
	repository    = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	errShortWrite = errors.New("short file write")
)

type config struct {
	APIURL         string
	BaseRepository string
	BaseBranch     string
	HeadRepository string
	HeadSHA        string
	PullRequest    int
	Output         string
	Bounds         string
	Token          string
}

type apiClient struct {
	baseURL *url.URL
	token   string
	client  *http.Client
}

type commitResponse struct {
	SHA  string `json:"sha"`
	Tree struct {
		SHA string `json:"sha"`
	} `json:"tree"`
}

type treeResponse struct {
	SHA       string      `json:"sha"`
	Truncated bool        `json:"truncated"`
	Tree      []treeEntry `json:"tree"`
}

type treeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size *int64 `json:"size,omitempty"`
}

type blobResponse struct {
	SHA      string `json:"sha"`
	Size     int64  `json:"size"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

type pullResponse struct {
	Number int `json:"number"`
	Base   struct {
		Ref  string `json:"ref"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"base"`
	Head struct {
		SHA  string `json:"sha"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`
}

type materializeResult struct {
	TreeSHA   string
	FileCount int
	TotalSize int64
}

type exclusiveFile interface {
	io.Writer
	Close() error
	Chmod(os.FileMode) error
}

type fileSystem interface {
	Mkdir(string, os.FileMode) error
	Lstat(string) (os.FileInfo, error)
	OpenExclusive(string, os.FileMode) (exclusiveFile, error)
	RemoveAll(string) error
}

type osFileSystem struct{}

func (osFileSystem) Mkdir(name string, mode os.FileMode) error { return os.Mkdir(name, mode) }
func (osFileSystem) Lstat(name string) (os.FileInfo, error)    { return os.Lstat(name) }
func (osFileSystem) RemoveAll(name string) error               { return os.RemoveAll(name) }
func (osFileSystem) OpenExclusive(name string, mode os.FileMode) (exclusiveFile, error) {
	return os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "materialize-sonar-source: %v\n", err)
		os.Exit(2)
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "materialize-sonar-source: %v\n", err)
		os.Exit(2)
	}
	result, err := materialize(context.Background(), client, osFileSystem{}, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "materialize-sonar-source: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("materialized %d verified Go blobs (%d bytes) from tree %s\n", result.FileCount, result.TotalSize, result.TreeSHA)
}

func parseConfig(args []string) (config, error) {
	var cfg config
	flags := flag.NewFlagSet("materialize-sonar-source", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&cfg.APIURL, "api-url", "https://api.github.com", "GitHub API base URL")
	flags.StringVar(&cfg.BaseRepository, "base-repository", "", "base owner/repository")
	flags.StringVar(&cfg.BaseBranch, "base-branch", "main", "protected base branch")
	flags.StringVar(&cfg.HeadRepository, "head-repository", "", "verified head owner/repository")
	flags.StringVar(&cfg.HeadSHA, "head-sha", "", "verified full head SHA")
	flags.IntVar(&cfg.PullRequest, "pull-request", 0, "pull-request number")
	flags.StringVar(&cfg.Output, "output", "analysis/source", "new source root")
	flags.StringVar(&cfg.Bounds, "bounds", "", "reviewed source-bounds identity")
	if err := flags.Parse(args); err != nil {
		return config{}, err
	}
	cfg.Token = os.Getenv("GH_TOKEN")
	if flags.NArg() != 0 {
		return config{}, errors.New("positional arguments are not accepted")
	}
	if !repository.MatchString(cfg.BaseRepository) || !repository.MatchString(cfg.HeadRepository) {
		return config{}, errors.New("base and head repositories must be owner/name identifiers")
	}
	if cfg.BaseBranch != "main" {
		return config{}, errors.New("base branch must be main")
	}
	if !fullSHA.MatchString(cfg.HeadSHA) {
		return config{}, errors.New("head SHA must be 40 lowercase hexadecimal characters")
	}
	if cfg.PullRequest <= 0 {
		return config{}, errors.New("pull-request number must be positive")
	}
	if cfg.Bounds != reviewedBounds {
		return config{}, fmt.Errorf("bounds must equal reviewed policy %q", reviewedBounds)
	}
	if cfg.Token == "" {
		return config{}, errors.New("GH_TOKEN is required")
	}
	if cfg.Output == "" || !filepath.IsLocal(cfg.Output) || filepath.Clean(cfg.Output) != cfg.Output {
		return config{}, errors.New("output must be a canonical local path")
	}
	return cfg, nil
}

func newAPIClient(cfg config) (*apiClient, error) {
	baseURL, err := url.Parse(cfg.APIURL)
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, errors.New("API URL must be an absolute URL without query or fragment")
	}
	return &apiClient{baseURL: baseURL, token: cfg.Token, client: http.DefaultClient}, nil
}

func (c *apiClient) get(ctx context.Context, endpoint string, target any) error {
	reference, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse API endpoint: %w", err)
	}
	requestURL := c.baseURL.ResolveReference(reference)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return fmt.Errorf("create API request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", endpoint, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("GET %s: unexpected HTTP status %d", endpoint, response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxAPIResponse+1))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode GET %s: %w", endpoint, err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("decode GET %s: trailing JSON content", endpoint)
	}
	return nil
}

func materialize(ctx context.Context, client *apiClient, fs fileSystem, cfg config) (result materializeResult, err error) {
	if cfg.Bounds != reviewedBounds {
		return materializeResult{}, errors.New("unreviewed source bounds")
	}

	var commit commitResponse
	if err := client.get(ctx, repoEndpoint(cfg.HeadRepository, "git/commits/"+cfg.HeadSHA), &commit); err != nil {
		return materializeResult{}, err
	}
	if commit.SHA != cfg.HeadSHA || !fullSHA.MatchString(commit.Tree.SHA) {
		return materializeResult{}, errors.New("commit response does not match the verified head and root tree")
	}

	var tree treeResponse
	endpoint := repoEndpoint(cfg.HeadRepository, "git/trees/"+commit.Tree.SHA) + "?recursive=1"
	if err := client.get(ctx, endpoint, &tree); err != nil {
		return materializeResult{}, err
	}
	if tree.SHA != commit.Tree.SHA {
		return materializeResult{}, errors.New("recursive tree identity does not match the commit")
	}
	selected, totalSize, err := validateTree(tree)
	if err != nil {
		return materializeResult{}, err
	}

	if err := fs.Mkdir(cfg.Output, 0o755); err != nil {
		return materializeResult{}, fmt.Errorf("create fresh source root: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			_ = fs.RemoveAll(cfg.Output)
		}
	}()

	for _, entry := range selected {
		data, err := fetchBlob(ctx, client, cfg.HeadRepository, entry)
		if err != nil {
			return materializeResult{}, err
		}
		if err := writeExclusive(fs, cfg.Output, entry.Path, data); err != nil {
			return materializeResult{}, err
		}
	}

	if err := verifyCurrentHead(ctx, client, cfg); err != nil {
		return materializeResult{}, err
	}
	complete = true
	return materializeResult{TreeSHA: tree.SHA, FileCount: len(selected), TotalSize: totalSize}, nil
}

func validateTree(tree treeResponse) ([]treeEntry, int64, error) {
	if tree.Truncated {
		return nil, 0, errors.New("recursive tree response is truncated")
	}
	if len(tree.Tree) == 0 || len(tree.Tree) > maxTreeEntries {
		return nil, 0, fmt.Errorf("recursive tree has %d entries; allowed range is 1..%d", len(tree.Tree), maxTreeEntries)
	}
	seen := make(map[string]struct{}, len(tree.Tree))
	selected := make([]treeEntry, 0)
	var total int64
	for _, entry := range tree.Tree {
		if err := validateTreePath(entry.Path); err != nil {
			return nil, 0, fmt.Errorf("unsafe tree path %q: %w", entry.Path, err)
		}
		if _, exists := seen[entry.Path]; exists {
			return nil, 0, fmt.Errorf("duplicate normalized tree path %q", entry.Path)
		}
		seen[entry.Path] = struct{}{}
		if !fullSHA.MatchString(entry.SHA) {
			return nil, 0, fmt.Errorf("entry %q has an invalid object identity", entry.Path)
		}
		switch {
		case entry.Type == "tree" && entry.Mode == "040000":
			continue
		case entry.Type == "blob" && (entry.Mode == "100644" || entry.Mode == "100755"):
			// Executable and non-Go blobs are inspected but never materialized.
		default:
			return nil, 0, fmt.Errorf("entry %q has forbidden type/mode %s/%s", entry.Path, entry.Type, entry.Mode)
		}
		if entry.Mode != "100644" || !strings.HasSuffix(entry.Path, ".go") {
			continue
		}
		if entry.Size == nil || *entry.Size < 0 || *entry.Size > maxFileBytes {
			return nil, 0, fmt.Errorf("Go blob %q has invalid or excessive size", entry.Path)
		}
		total += *entry.Size
		if total > maxTotalBytes {
			return nil, 0, errors.New("aggregate Go source size exceeds the reviewed bound")
		}
		selected = append(selected, entry)
		if len(selected) > maxGoFiles {
			return nil, 0, errors.New("selected Go file count exceeds the reviewed bound")
		}
	}
	if len(selected) == 0 {
		return nil, 0, errors.New("recursive tree contains no eligible Go source")
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Path < selected[j].Path })
	return selected, total, nil
}

func validateTreePath(value string) error {
	if value == "" || !utf8.ValidString(value) {
		return errors.New("path must be non-empty UTF-8")
	}
	if len([]byte(value)) > maxPathBytes {
		return fmt.Errorf("path exceeds %d bytes", maxPathBytes)
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, `\`) || path.Clean(value) != value {
		return errors.New("path must be canonical, relative, and slash-separated")
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return errors.New("path contains an empty or traversal segment")
		}
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return errors.New("path contains a control character")
		}
	}
	return nil
}

func fetchBlob(ctx context.Context, client *apiClient, headRepository string, entry treeEntry) ([]byte, error) {
	var blob blobResponse
	if err := client.get(ctx, repoEndpoint(headRepository, "git/blobs/"+entry.SHA), &blob); err != nil {
		return nil, err
	}
	if blob.SHA != entry.SHA || blob.Size != *entry.Size || blob.Encoding != "base64" {
		return nil, fmt.Errorf("blob response for %q does not match its tree identity, size, and encoding", entry.Path)
	}
	encoded := strings.ReplaceAll(blob.Content, "\n", "")
	data, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode blob %q: %w", entry.Path, err)
	}
	if int64(len(data)) != *entry.Size || len(data) > maxFileBytes {
		return nil, fmt.Errorf("decoded blob %q has the wrong size", entry.Path)
	}
	digest := sha1.Sum(append([]byte("blob "+strconv.Itoa(len(data))+"\x00"), data...)) //nolint:gosec // Git object identity.
	if hex.EncodeToString(digest[:]) != entry.SHA {
		return nil, fmt.Errorf("decoded blob %q does not match its Git identity", entry.Path)
	}
	return data, nil
}

func writeExclusive(fs fileSystem, root, relative string, data []byte) error {
	directory := root
	parts := strings.Split(path.Dir(relative), "/")
	if len(parts) == 1 && parts[0] == "." {
		parts = nil
	}
	for _, part := range parts {
		directory = filepath.Join(directory, part)
		info, err := fs.Lstat(directory)
		if errors.Is(err, os.ErrNotExist) {
			if err := fs.Mkdir(directory, 0o755); err != nil {
				return fmt.Errorf("create source directory: %w", err)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect source directory: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("source directory %q is not a real directory", directory)
		}
	}

	destination := filepath.Join(root, filepath.FromSlash(relative))
	file, err := fs.OpenExclusive(destination, 0o600)
	if err != nil {
		return fmt.Errorf("create exclusive source file %q: %w", relative, err)
	}
	if err := writeOnce(file, data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write source file %q: %w", relative, err)
	}
	if err := file.Chmod(0o644); err != nil {
		_ = file.Close()
		return fmt.Errorf("set source file mode %q: %w", relative, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close source file %q: %w", relative, err)
	}
	return nil
}

func writeOnce(writer io.Writer, data []byte) error {
	written, err := writer.Write(data)
	if err != nil {
		return err
	}
	if written != len(data) {
		return errShortWrite
	}
	return nil
}

func verifyCurrentHead(ctx context.Context, client *apiClient, cfg config) error {
	var pull pullResponse
	endpoint := repoEndpoint(cfg.BaseRepository, "pulls/"+strconv.Itoa(cfg.PullRequest))
	if err := client.get(ctx, endpoint, &pull); err != nil {
		return err
	}
	if pull.Number != cfg.PullRequest || pull.Base.Repo.FullName != cfg.BaseRepository || pull.Base.Ref != cfg.BaseBranch ||
		pull.Head.Repo.FullName != cfg.HeadRepository || pull.Head.SHA != cfg.HeadSHA {
		return errors.New("pull request head changed or repository provenance no longer matches")
	}
	return nil
}

func repoEndpoint(repo, suffix string) string {
	parts := strings.Split(repo, "/")
	return "/repos/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) + "/" + suffix
}
