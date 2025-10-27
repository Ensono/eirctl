package runner_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/runner"
	"github.com/Ensono/eirctl/task"
	"github.com/Ensono/eirctl/variables"
)

func TestContext(t *testing.T) {
	logrus.SetOutput(io.Discard)

	c1 := runner.NewExecutionContext(nil, "/", variables.NewVariables(), &utils.Envfile{}, []string{"true"}, []string{"false"}, []string{"true"}, []string{"false"})
	c2 := runner.NewExecutionContext(nil, "/", variables.NewVariables(), &utils.Envfile{}, []string{"false"}, []string{"false"}, []string{"false"}, []string{"false"})

	runner, err := runner.NewTaskRunner(runner.WithContexts(map[string]*runner.ExecutionContext{"after_failed": c1, "before_failed": c2}))
	if err != nil {
		t.Fatal(err)
	}

	task1 := task.FromCommands("t1", "true")
	task1.Context = "after_failed"

	task2 := task.FromCommands("t2", "true")
	task2.Context = "before_failed"

	err = runner.Run(task1)
	if err != nil || task1.ExitCode() != 0 {
		t.Fatal(err)
	}

	err = runner.Run(task2)
	if err == nil {
		t.Error()
	}

	if c2.StartupError() == nil || task2.ExitCode() != -1 {
		t.Error()
	}

	runner.Finish()
}

func helpSetupCleanUp() (path string, defereCleanUp func()) {
	tmpDir, _ := os.MkdirTemp(os.TempDir(), "context-envfile")
	path = filepath.Join(tmpDir, "generated_task_123.env")
	return path, func() {
		os.RemoveAll(tmpDir)
	}
}

func Test_Generate_Env_file(t *testing.T) {
	t.Run("with correctly merged output in env file from os and user supplied Env", func(t *testing.T) {

		osEnvVars := variables.FromMap(map[string]string{"var1": "original", "var2": "original222"})
		userEnvVars := variables.FromMap(map[string]string{"foo": "bar", "var1": "userOverwrittemdd"})
		ef := utils.NewEnvFile()

		contents := genEnvFileHelperTestRunner(t, osEnvVars.Merge(userEnvVars), ef)

		if strings.Contains(contents, "var1=original") {
			t.Fatal("incorrectly merged and overwritten env vars")
		}
	})
	t.Run("with forbidden variable names correctly stripped out", func(t *testing.T) {
		osEnvVars := variables.FromMap(map[string]string{"var1": "original", "var2": "original222", "!::": "whatever val will never be added"})
		userEnvVars := variables.FromMap(map[string]string{"foo": "bar", "var1": "userOverwrittemdd"})
		ef := utils.NewEnvFile()

		contents := genEnvFileHelperTestRunner(t, osEnvVars.Merge(userEnvVars), ef)

		if strings.Contains(contents, "!::=whatever val will never be added") {
			t.Fatal("invalid cahrs not skipped properly and overwritten env vars")
		}
	})
	t.Run("with exclude variable names correctly stripped out", func(t *testing.T) {
		osEnvVars := variables.FromMap(map[string]string{"var1": "original", "var2": "original222", "!::": "whatever val will never be added", "=::": "whatever val will never be added",
			"": "::=::", " ": "::=::", "excld1": "bye bye", "exclude3": "sadgfddf"})
		userEnvVars := variables.FromMap(map[string]string{"foo": "bar", "var1": "userOverwrittemdd", "userSuppliedButExcluded": `¯\_(ツ)_/¯`, "UPPER_VAR_make_me_bigger": "this_key_is_large"})
		ef := utils.NewEnvFile(func(e *utils.Envfile) {
			e.Exclude = append(e.Exclude, []string{"excld1", "exclude3", "userSuppliedButExcluded"}...)
			e.Modify = append(e.Modify, []utils.ModifyEnv{
				{Pattern: "^(?P<keyword>TF_VAR_)(?P<varname>.*)", Operation: "lower"},
				{Pattern: "^(?P<keyword>UPPER_VAR_)(?P<varname>.*)", Operation: "upper"},
			}...)
		})

		contents := genEnvFileHelperTestRunner(t, osEnvVars.Merge(userEnvVars), ef)

		for _, excluded := range []string{"excld1=bye bye", "exclude3=sadgfddf", `userSuppliedButExcluded=¯\_(ツ)_/¯`} {
			if slices.Contains(strings.Split(contents, "\n"), excluded) {
				t.Fatal("invalid chars not skipped properly and overwritten env vars")
			}
		}

		if slices.Contains(strings.Split(contents, "\n"), "=::=whatever val will never be added") {
			t.Fatal("invalid chars not skipped properly and overwritten env vars")
		}

		if slices.Contains(strings.Split(contents, "\n"), "!::=whatever val will never be added") {
			t.Fatal("invalid chars not skipped properly and overwritten env vars")
		}

		if !slices.Contains(strings.Split(contents, "\n"), "UPPER_VAR_MAKE_ME_BIGGER=this_key_is_large") {
			t.Fatal("Modify not changed the values properly")
		}
	})

	t.Run("with include variable names correctly set", func(t *testing.T) {
		osEnvVars := variables.FromMap(map[string]string{"var1": "original", "var2": "original222", "!::": "whatever val will never be added", "=::": "whatever val will never be added 2", "incld1": "welcome var", "exclude3": "sadgfddf"})
		userEnvVars := variables.FromMap(map[string]string{"foo": "bar", "var1": "userOverwrittemdd", "userSuppliedButExcluded": `¯\_(ツ)_/¯`})
		ef := utils.NewEnvFile(func(e *utils.Envfile) {
			e.Exclude = append(e.Exclude, []string{}...)
			e.Include = append(e.Include, "incld1")
		})

		contents := genEnvFileHelperTestRunner(t, osEnvVars.Merge(userEnvVars), ef)

		for _, included := range []string{"incld1=welcome var"} {
			if !slices.Contains(strings.Split(contents, "\n"), included) {
				t.Fatal("invalid vars not skipped properly and overwritten env vars")
			}
		}
	})

	// Note about this test case
	// it will include exclude from the injected env
	// however the merging of environment variables is still case sensitive
	t.Run("with case insensitive comparison on exclude", func(t *testing.T) {
		osEnvVars := variables.FromMap(map[string]string{"var1": "original", "var2": "original222", "!::": "whatever val will never be added", "=::": "whatever val will never be added 2", "incld1": "welcome var", "exclude3": "sadgfddf"})
		userEnvVars := variables.FromMap(map[string]string{"foo": "bar", "VAR1": "userOverwrittemdd", "userSuppliedButExcluded": `¯\_(ツ)_/¯`})

		ef := utils.NewEnvFile(func(e *utils.Envfile) {
			e.Exclude = append(e.Exclude, []string{"var1", "FOO", "UserSuppliedButEXCLUDED"}...)
		})

		contents := genEnvFileHelperTestRunner(t, osEnvVars.Merge(userEnvVars), ef)

		got := strings.Split(contents, "\n")
		for _, checkExcluded := range []string{"var1=original", "VAR1=userOverwrittemdd", "foo=bar", `userSuppliedButExcluded=¯\_(ツ)_/¯`} {
			if slices.Contains(got, checkExcluded) {
				t.Fatalf("invalid vars\ngot: %q\nshould have skipped ( %s )\n", got, checkExcluded)
			}
		}

		for _, checkIncluded := range []string{"var2=original222", "incld1=welcome var", "exclude3=sadgfddf"} {
			if !slices.Contains(got, checkIncluded) {
				t.Fatalf("invalid vars\ngot: %q\nshould have included ( %s )\n", got, checkIncluded)
			}
		}
	})

	t.Run("with case insensitive comparison on include", func(t *testing.T) {
		osEnvVars := variables.FromMap(map[string]string{"var1": "original", "var2": "original222", "!::": "whatever val will never be added", "=::": "whatever val will never be added 2", "incld1": "welcome var", "exclude3": "sadgfddf"})
		userEnvVars := variables.FromMap(map[string]string{"foo": "bar", "VAR1": "userOverwrittemdd", "userSuppliedButExcluded": `¯\_(ツ)_/¯`})

		ef := utils.NewEnvFile(func(e *utils.Envfile) {
			e.Include = []string{"var1", "FOO", "UserSuppliedButEXCLUDED"}
		})

		contents := genEnvFileHelperTestRunner(t, osEnvVars.Merge(userEnvVars), ef)

		got := strings.Split(contents, "\n")
		for _, checkExcluded := range []string{"var1=original", "VAR1=userOverwrittemdd", "foo=bar", `userSuppliedButExcluded=¯\_(ツ)_/¯`} {
			if !slices.Contains(got, checkExcluded) {
				t.Fatalf("invalid vars\ngot: %q\nshould have skipped ( %s )\n", got, checkExcluded)
			}
		}

		for _, checkIncluded := range []string{"var2=original222", "incld1=welcome var", "exclude3=sadgfddf"} {
			if slices.Contains(got, checkIncluded) {
				t.Fatalf("invalid vars\ngot: %q\nshould have included ( %s )\n", got, checkIncluded)
			}
		}
	})

	t.Run("with include/exclude variable both set return error", func(t *testing.T) {
		outputFilePath, cleanUp := helpSetupCleanUp()

		defer cleanUp()

		osEnvVars := variables.FromMap(map[string]string{"var1": "original", "var2": "original222", "!::": "whatever val will never be added", "incld1": "welcome var", "exclude3": "sadgfddf"})
		userEnvVars := variables.FromMap(map[string]string{"foo": "bar", "var1": "userOverwrittemdd", "userSuppliedButExcluded": `¯\_(ツ)_/¯`})
		envVars := osEnvVars.Merge(userEnvVars)

		execContext := runner.NewExecutionContext(nil, "", envVars, utils.NewEnvFile(func(e *utils.Envfile) {
			e.PathValue = []string{outputFilePath}
			e.Exclude = append(e.Exclude, []string{"excld1", "exclude3", "userSuppliedButExcluded"}...)
			e.Include = append(e.Include, "incld1")
		}), []string{}, []string{}, []string{}, []string{})

		if err := execContext.ProcessEnvfile(envVars); err == nil {
			t.Fatal("got nil, wanted an error")
		}

	})

}

func genEnvFileHelperTestRunner(t *testing.T, envVars *variables.Variables, envFile *utils.Envfile) string {
	t.Helper()

	execContext := runner.NewExecutionContext(nil, "", envVars, envFile, []string{}, []string{}, []string{}, []string{})

	if err := execContext.ProcessEnvfile(envVars); err != nil {
		t.Fatal(err)
	}

	if len(execContext.Env.Map()) < 1 {
		t.Fatal("empty")
	}

	return helperEnvString(execContext.Env)
}

func helperEnvString(envMap *variables.Variables) string {
	s := &strings.Builder{}
	for _, envPair := range utils.ConvertEnv(utils.ConvertToMapOfStrings(envMap.Map())) {
		s.Write([]byte(envPair))
		s.Write([]byte{'\n'})
	}
	return s.String()
}

func ExampleExecutionContext_ProcessEnvfile() {
	osEnvVars := variables.FromMap(map[string]string{"TF_VAR_CAPPED_BY_MSFT": "some value"})
	//  "var2": "original222", "!::": "whatever val will never be added", "incld1": "welcome var", "exclude3": "sadgfddf"})
	userEnvVars := variables.FromMap(map[string]string{})
	envVars := osEnvVars.Merge(userEnvVars)

	ef := utils.NewEnvFile(func(e *utils.Envfile) {
		e.Exclude = append(e.Exclude, []string{"excld1", "exclude3", "userSuppliedButExcluded"}...)
		e.Modify = append(e.Modify, []utils.ModifyEnv{
			{Pattern: "^(?P<keyword>TF_VAR_)(?P<varname>.*)", Operation: "lower"},
		}...)
	})

	execContext := runner.NewExecutionContext(nil, "", envVars, ef, []string{}, []string{}, []string{}, []string{})
	_ = execContext.ProcessEnvfile(envVars)

	// for the purposes of the test example we need to make sure the map is
	// always displayed in same order of keys, which is not a guarantee with a map
	fmt.Println(helperEnvString(execContext.Env))
	//Output:
	// TF_VAR_capped_by_msft=some value
}

func Test_ContainerContext_Volumes(t *testing.T) {
	pwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	ttests := map[string]struct {
		containerArgs []string
		want          []string
	}{
		"all volumes no expansion": {
			containerArgs: []string{"-v /foo/bar:/in", "-v /foo/baz:/two"},
			want:          []string{"/foo/bar:/in", "/foo/baz:/two"},
		},
		"PWD": {
			containerArgs: []string{"-v $PWD/bar:/in", "-v /foo/baz:/two"},
			want:          []string{pwd + "/bar:/in", "/foo/baz:/two"},
		},
		"HOME": {
			containerArgs: []string{"-v $HOME/bar:/in", "--volume ${HOME}/baz:/two"},
			want:          []string{home + "/bar:/in", home + "/baz:/two"},
		},
		"~": {
			containerArgs: []string{"-v ~/bar:/in", "-v ${HOME}/baz:/two"},
			want:          []string{home + "/bar:/in", home + "/baz:/two"},
		},
		// home
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cc := runner.NewContainerContext("image:latest")
			cc.ParseContainerArgs(tt.containerArgs)
			got := cc.Volumes()
			for vol := range got {
				if !slices.Contains(tt.want, vol) {
					t.Errorf("incorrect volume mappging, got: %s, wanted: %s\n", vol, tt.want)
				}
			}
		})
	}
}

func Test_ContainerContext_Volume_BindMounts(t *testing.T) {
	ttests := map[string]struct {
		volumes        []string
		wantSourcePath []string
		wantTargetPath []string
	}{
		"unix like": {
			volumes:        []string{"/foo/bar:/in"},
			wantSourcePath: []string{"/foo/bar"},
			wantTargetPath: []string{"/in"},
		},
		"win like": {
			volumes:        []string{"c:/foo/bar/baz/modules:/container/path"},
			wantSourcePath: []string{"c:/foo/bar/baz/modules"},
			wantTargetPath: []string{"/container/path"},
		},
		"darwin like": {
			volumes:        []string{"/foo/:bar:/baz/modules:/container/path"},
			wantSourcePath: []string{"/foo/:bar:/baz/modules"},
			wantTargetPath: []string{"/container/path"},
		},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			cc := runner.NewContainerContext("image:latest")
			cc.WithVolumes(tt.volumes...)
			got := cc.BindMounts()

			if len(got) < 1 {
				t.Fatal("expecting at least 1 BindMount...")
			}

			for _, bm := range got {
				if !slices.Contains(tt.wantSourcePath, bm.SourcePath) {
					t.Errorf("incorrect volume bind mount translation, got: %s, wanted: %v\n", bm.SourcePath, tt.wantSourcePath)
				}
				if !slices.Contains(tt.wantTargetPath, bm.TargetPath) {
					t.Errorf("incorrect volume bind mount translation, got: %s, wanted: %v\n", bm.TargetPath, tt.wantTargetPath)
				}
			}
		})
	}
}

func Test_ContainerContext_ParseArgs_env_replacement(t *testing.T) {
	ttests := map[string]struct {
		containerArgs []string
		want          []string
		setupFunc     func() func()
	}{
		"1 property only": {
			containerArgs: []string{"-v ${VOL}"},
			want:          []string{"/foo/bar:/in"},
			setupFunc: func() func() {
				os.Setenv("VOL", "/foo/bar:/in")
				return func() {
					os.Unsetenv("VOL")
				}
			},
		},
		"user and vol property only": {
			containerArgs: []string{"-v ${VOL}", "--user ${USER_FOO}"},
			want:          []string{"/foo/bar:/in", "123:123"},
			setupFunc: func() func() {
				os.Setenv("VOL", "/foo/bar:/in")
				os.Setenv("USER_FOO", "123:123")
				return func() {
					os.Unsetenv("VOL")
				}
			},
		},
		"user key and val and volume property only": {
			containerArgs: []string{"-v ${VOL}", "${USER_FOO_FULL}"},
			want:          []string{"/foo/bar:/in", "123:123"},
			setupFunc: func() func() {
				os.Setenv("VOL", "/foo/bar:/in")
				os.Setenv("USER_FOO_FULL", "--user 123:123")
				return func() {
					os.Unsetenv("VOL")
				}
			},
		},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			tearDown := tt.setupFunc()
			defer tearDown()

			cc := runner.NewContainerContext("image:latest")
			cc.ParseContainerArgs(tt.containerArgs)

			for vol := range cc.Volumes() {
				if !slices.Contains(tt.want, vol) {
					t.Errorf("incorect volume mappging, got: %s, wanted: %s\n", vol, tt.want)
				}
			}
			if cc.User() != "" && !slices.Contains(tt.want, cc.User()) {
				t.Errorf("incorect user mapping, got: %s, wanted: %s\n", cc.User(), tt.want)
			}
		})
	}
}

func Test_ContainerContext_UserArgs(t *testing.T) {
	ttests := map[string]struct {
		containerArgs []string
		want          string
		expectErr     bool
	}{
		"--user foo": {
			containerArgs: []string{"-v /foo/bar:/in", "-v /foo/baz:/two", "--user foo", "--userns private"},
			want:          "foo",
			expectErr:     false,
		},
		"-u foo": {
			containerArgs: []string{"-v $PWD/bar:/in", "--volume /foo/baz:/two", "-u foo"},
			want:          "foo",
			expectErr:     false,
		},
		" -u foo:bar": {
			containerArgs: []string{"-v $HOME/bar:/in", " -u foo:bar"},
			want:          "foo:bar",
			expectErr:     false,
		},
		"-u bar and --user foo": {
			containerArgs: []string{"-u bar", "-v /foo/bar:/in", "-v /foo/baz:/two", "--user foo"},
			want:          "",
			expectErr:     true,
		},
		"-u bar and --user=foo": {
			containerArgs: []string{"-u bar", "-v /foo/bar:/in", "--user=foo", "-v /foo/baz:/two"},
			want:          "",
			expectErr:     true,
		},
		"-u foo and -u foo": {
			containerArgs: []string{"-u foo", "-v /foo/bar:/in", "-u foo", "-v /foo/baz:/two"},
			want:          "",
			expectErr:     true,
		},
		"--user bar and --user foo": {
			containerArgs: []string{"--user bar", "-v /foo/bar:/in", "--user foo", "-v /foo/baz:/two"},
			want:          "",
			expectErr:     true,
		},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cc := runner.NewContainerContext("image:latest")
			_, err := cc.ParseContainerArgs(tt.containerArgs)
			got := cc.User()

			if err != nil && !tt.expectErr {
				t.Errorf("not expecting an error: %s", err)
				t.FailNow()
			}

			if err == nil {
				if tt.expectErr {
					t.Errorf("expecting an error for duplicate user flags, got: %s", got)
					t.FailNow()
				}

				if got != tt.want {
					t.Errorf("incorrect user mapping, got: %s, wanted: %s\n", got, tt.want)
				}
			}
		})
	}
}

func Test_ContainerContext_PortArgs(t *testing.T) {
	ttests := map[string]struct {
		containerArgs     []string
		wantContainerPort []string
		wantHostPort      []string
		expectErr         bool
	}{
		"--user foo": {
			containerArgs:     []string{"-v /foo/bar:/in", "-v /foo/baz:/two", "--port 3000:3000", "--userns private"},
			wantContainerPort: []string{"3000"},
			wantHostPort:      []string{"3000"},
			expectErr:         false,
		},
		"-p 1111:80 --port 2222:8080": {
			containerArgs:     []string{"-v $PWD/bar:/in", "--volume /foo/baz:/two", "-p 1111:80", "-p 2222:8080"},
			wantContainerPort: []string{"80", "8080"},
			wantHostPort:      []string{"1111", "2222"},
			expectErr:         false,
		},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cc := runner.NewContainerContext("image:latest")
			_, err := cc.ParseContainerArgs(tt.containerArgs)

			if err != nil && !tt.expectErr {
				t.Errorf("not expecting an error: %s", err)
				t.FailNow()
			}

			gotPorts, gotPortMaps := cc.Ports()

			if len(gotPorts) < 1 {
				t.Fatal("expecting at least 1 BindMount...")
			}

			for port := range gotPorts {
				if !slices.Contains(tt.wantContainerPort, port.Port()) {
					t.Errorf("incorrect container expose port translation, got: %s, wanted: %v\n", port.Port(), tt.wantContainerPort)
				}
			}

			for _, portBinding := range gotPortMaps {
				for _, pb := range portBinding {
					if !slices.Contains(tt.wantHostPort, pb.HostPort) {
						t.Errorf("incorrect host port translation, got: %s, wanted: %v\n",
							pb.HostPort, tt.wantHostPort)
					}
				}
			}
		})
	}
}

func Test_ContainerContext_UnsupportedArgs(t *testing.T) {
	ttests := map[string]struct {
		containerArgs []string
		want          string
		expectErr     bool
	}{
		"--foo": {
			containerArgs: []string{"--foo"},
			want:          "",
			expectErr:     true,
		},
		"--bar": {
			containerArgs: []string{"--bar"},
			want:          "",
			expectErr:     true,
		},
		"-u bar and --foo foo": {
			containerArgs: []string{"-u bar", "--foo foo"},
			want:          "",
			expectErr:     true,
		},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cc := runner.NewContainerContext("image:latest")
			_, err := cc.ParseContainerArgs(tt.containerArgs)

			if err != nil && !tt.expectErr {
				t.Errorf("not expecting an error: %s", err)
				t.FailNow()
			}

			got := cc.User()

			if err == nil {
				if tt.expectErr {
					t.Errorf("expecting an error for duplicate user flags, got: %s", got)
					t.FailNow()
				}

				if got != tt.want {
					t.Errorf("incorrect user mapping, got: %s, wanted: %s\n", got, tt.want)
				}
			}
		})
	}
}
