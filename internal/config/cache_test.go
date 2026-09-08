package config_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/internal/schema"
	"github.com/Ensono/eirctl/internal/utils"
)

type mockfo struct {
	m func(path string, perm os.FileMode) error
	c func(n string) (io.Writer, error)
	o func(n string) (io.Reader, error)
}

func (m mockfo) Create(n string) (io.Writer, error) {
	if m.c != nil {
		return m.c(n)
	}
	return &bytes.Buffer{}, nil
}

func (m mockfo) Open(n string) (io.Reader, error) {
	if m.o != nil {
		return m.o(n)
	}
	return &bytes.Buffer{}, nil
}

func (m mockfo) MkdirAll(path string, perm os.FileMode) error {
	if m.m != nil {
		return m.m(path, perm)
	}
	return nil
}

type errWriter struct {
	err error
}

func (w errWriter) Write([]byte) (int, error) {
	return 0, w.err
}

type storeInCacheTestCase struct {
	mockFsOp func(t *testing.T, mw io.Writer) mockfo
	wantErr  error
}

func Test_StoreInCache(t *testing.T) {
	testCases := map[string]storeInCacheTestCase{
		"successfully stores in path": {
			mockFsOp: func(t *testing.T, mw io.Writer) mockfo {
				m := mockfo{}
				want := filepath.Join("/foo", ".eirctl", "cache", utils.EncodeBase62("some-path"))
				m.c = func(n string) (io.Writer, error) {
					if n != want {
						t.Errorf("got %s, want: '%s'", n, want)
					}
					return mw, nil
				}
				return m
			},
			wantErr: nil,
		},
		"fails to copy stream": {
			mockFsOp: func(t *testing.T, mw io.Writer) mockfo {
				m := mockfo{}
				m.c = func(n string) (io.Writer, error) {
					return errWriter{err: errors.New("writeError")}, nil
				}
				return m
			},
			wantErr: config.ErrCacheStreamCopyFailed,
		},
		"fails to create cache dir structure": {
			mockFsOp: func(t *testing.T, mw io.Writer) mockfo {
				m := mockfo{}
				want := filepath.Join("/foo", ".eirctl", "cache", utils.EncodeBase62("some-path"))
				m.c = func(n string) (io.Writer, error) {
					if n != want {
						t.Errorf("got %s, want: '%s'", n, want)
					}
					return mw, nil
				}

				m.m = func(path string, perm os.FileMode) error {
					return errors.New("failed to create dir")
				}
				return m
			},
			wantErr: config.ErrCacheDirCreationFailed,
		},
	}

	for name, tt := range testCases {
		t.Run(name, func(t *testing.T) {
			runStoreInCacheTest(t, tt)
		})
	}
}

func runStoreInCacheTest(t *testing.T, tt storeInCacheTestCase) {
	t.Helper()

	output := &bytes.Buffer{}
	cache := config.NewCache("/foo").WithFsOps(tt.mockFsOp(t, output))
	err := cache.Store("some-path", bytes.NewBuffer([]byte(`context: {}`)))
	if tt.wantErr != nil {
		if !errors.Is(err, tt.wantErr) {
			t.Fatalf("wrong error type got: %v, want: %v", err, tt.wantErr)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if output.String() != "context: {}" {
		t.Errorf("got %s, want: 'context: {}'", output.String())
	}
}

type getFromCacheTestCase struct {
	wantErr error
	want    *config.ContextDefinition
	cache   func() *config.Cache
	entry   schema.ImportEntry
}

func Test_Get_fromCache(t *testing.T) {
	ttests := map[string]getFromCacheTestCase{
		"successfully gets from cache": {
			cache: func() *config.Cache {
				m := mockfo{o: func(n string) (io.Reader, error) {
					r := new(bytes.Buffer)
					_, _ = r.WriteString(`contexts:
  foo:
    container:
      name: "image:123"
`)
					return r, nil
				}}
				return config.NewCache("/foo").WithFsOps(m)

			},
			entry:   schema.ImportEntry{Src: "/foo/.eirctl/cache/3245gertg"},
			wantErr: nil,
			want:    &config.ContextDefinition{Container: &utils.Container{Name: "image:123"}},
		},
		"encounters path error": {
			cache: func() *config.Cache {
				m := mockfo{o: func(n string) (io.Reader, error) {
					perr := &fs.PathError{Op: "get", Path: "/foo/.eirctl/cache/3245gertg", Err: errors.New("file not found")}
					return nil, perr
				}}
				return config.NewCache("/foo").WithFsOps(m)
			},
			entry:   schema.ImportEntry{Src: "/foo/.eirctl/cache/3245gertg"},
			wantErr: config.ErrFileNotInCache,
			want:    nil,
		},
		"encounters unknonw error": {
			cache: func() *config.Cache {
				m := mockfo{o: func(n string) (io.Reader, error) {
					return nil, errors.New("unknonw error")
				}}
				return config.NewCache("/foo").WithFsOps(m)
			},
			wantErr: config.ErrFailedToGetFromCache,
			entry:   schema.ImportEntry{Src: "/foo/.eirctl/cache/3245gertg"},
			want:    nil,
		},
		"import file succeeds": {
			cache: func() *config.Cache {
				m := mockfo{o: func(n string) (io.Reader, error) {
					r := new(bytes.Buffer)
					_, _ = r.WriteString(`contexts:
  foo:
    container:
      name: "image:123"
`)
					return r, nil
				}}
				return config.NewCache("/foo").WithFsOps(m).WithWriteImport(func(entry schema.ImportEntry, content io.ReadCloser) error {
					return nil
				})
			},
			entry:   schema.ImportEntry{Src: "/foo/.eirctl/cache/3245gertg", Dest: "/foo/bar"},
			wantErr: nil,
			want:    nil,
		},
		"import file fails on writeImport": {
			cache: func() *config.Cache {
				m := mockfo{o: func(n string) (io.Reader, error) {
					r := new(bytes.Buffer)
					_, _ = r.WriteString(`contexts:
  foo:
    container:
      name: "image:123"
`)
					return r, nil
				}}
				return config.NewCache("/foo").WithFsOps(m).WithWriteImport(func(entry schema.ImportEntry, content io.ReadCloser) error {
					return fmt.Errorf("failed to write")
				})
			},
			entry:   schema.ImportEntry{Src: "/foo/.eirctl/cache/3245gertg", Dest: "/foo/bar"},
			wantErr: config.ErrFailedToWriteImport,
			want:    nil,
		},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			runGetFromCacheTest(t, tt)
		})
	}
}

func runGetFromCacheTest(t *testing.T, tt getFromCacheTestCase) {
	t.Helper()
	t.Setenv("HOME", "/foo")
	t.Setenv("USERPROFILE", "/foo")

	got, err := tt.cache().Get(tt.entry)
	if tt.wantErr != nil {
		if !errors.Is(err, tt.wantErr) {
			t.Fatalf("wrong error, got %v, want %v", err, tt.wantErr)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if tt.want == nil {
		return
	}
	value, ok := got.Contexts["foo"]
	if !ok || value == nil {
		t.Fatalf("got %v, want %v", value, &utils.Container{Name: "image:123"})
	}
	if !reflect.DeepEqual(*value, *tt.want) {
		t.Errorf("objects don't match, got %v, want %v", value, tt.want)
	}
}
