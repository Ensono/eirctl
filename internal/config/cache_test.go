package config_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"

	cache "github.com/Ensono/eirctl/internal/config"
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

func Test_StoreInCache(t *testing.T) {
	testCases := map[string]struct {
		mockFsOp func(t *testing.T, mw io.Writer) mockfo
		wantErr  error
	}{
		"successfully stores in path": {
			mockFsOp: func(t *testing.T, mw io.Writer) mockfo {
				m := mockfo{}
				m.c = func(n string) (io.Writer, error) {
					if n != "/foo/.eirctl/cache/"+utils.EncodeBase62("some-path") {
						t.Errorf("got %s, want: '%s'", n, "/foo/.eirctl/cache/"+utils.EncodeBase62("some-path"))
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
			wantErr: cache.ErrCacheStreamCopyFailed,
		},
		"fails to create cache dir structure": {
			mockFsOp: func(t *testing.T, mw io.Writer) mockfo {
				m := mockfo{}
				m.c = func(n string) (io.Writer, error) {
					if n != "/foo/.eirctl/cache/"+utils.EncodeBase62("some-path") {
						t.Errorf("got %s, want: '%s'", n, "/foo/.eirctl/cache/"+utils.EncodeBase62("some-path"))
					}
					return mw, nil
				}

				m.m = func(path string, perm os.FileMode) error {
					return errors.New("failed to create dir")
				}
				return m
			},
			wantErr: cache.ErrCacheDirCreationFailed,
		},
	}

	for name, tt := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			os.Setenv("HOME", "/foo")
			os.Setenv("USERPROFILE", "/foo")

			defer os.Clearenv()

			mw := &bytes.Buffer{}

			c := cache.New().WithFsOps(tt.mockFsOp(t, mw))

			w := bytes.NewBuffer([]byte(`context: {}`))
			if err := c.StoreFromReader("some-path", w); err != nil {
				if tt.wantErr == nil {
					t.Fatal(err)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("wrong error type got: %v, want: %v", err, tt.wantErr)
				}
			} else {
				if mw.String() != "context: {}" {
					t.Errorf("got %s, want: 'context: {}'", mw.String())
				}
			}
		})
	}
}

func Test_Get_fromCache(t *testing.T) {
	ttests := map[string]struct {
		mockFsOp    func(t *testing.T, mw io.Writer) mockfo
		wantErr     error
		wantContent string
	}{
		"successfully gets from cache": {
			mockFsOp: func(t *testing.T, mw io.Writer) mockfo {
				m := mockfo{o: func(n string) (io.Reader, error) {
					r := new(bytes.Buffer)
					_, _ = r.WriteString(`contexts:
  foo:
    container:
      name: "image:123"
`)
					return r, nil
				}}
				return m
			},
			wantErr:     nil,
			wantContent: `context: {}`,
		},
		"encounters path error": {
			mockFsOp: func(t *testing.T, mw io.Writer) mockfo {
				m := mockfo{o: func(n string) (io.Reader, error) {
					perr := &fs.PathError{Op: "get", Path: "/foo/.eirctl/cache/3245gertg", Err: errors.New("file not found")}
					return nil, perr
				}}
				return m
			},
			wantErr:     cache.ErrFileNotInCache,
			wantContent: ``,
		},
		"encounters unknonw error": {
			mockFsOp: func(t *testing.T, mw io.Writer) mockfo {
				m := mockfo{o: func(n string) (io.Reader, error) {
					return nil, errors.New("unknonw error")
				}}
				return m
			},
			wantErr:     cache.ErrFailedToGetFromCache,
			wantContent: ``,
		},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			os.Setenv("HOME", "/foo")
			os.Setenv("USERPROFILE", "/foo")

			defer os.Clearenv()

			mw := &bytes.Buffer{}

			c := cache.New().WithFsOps(tt.mockFsOp(t, mw))

			got, err := c.Get("/foo/.eirctl/cache/3245gertg")

			if tt.wantErr != nil && err == nil {
				t.Fatalf("got nil err but wanted %v", tt.wantErr)
			}
			if err != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("wrong error, got %v, want %v", err, tt.wantErr)
				}
				return
			}
			// content, _ := io.ReadAll(got)
			// 			if string(content) != `context:
			//   foo:
			//     container: image:123
			// ` {
			// 				t.Error("does not match")
			// 			}
			if v, ok := got.Contexts["foo"]; !ok {
				t.Errorf("got %v, want %v", v, &utils.Container{Name: "image:123"})
				// t.Errorf("got %v, want Name: image:123", v)
			}
		})
	}
}
