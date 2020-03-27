// Package env defines a test environment.
package env

import (
	"container/list"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// ErrEarlyTerm is the error passed over the error channel when a
// sub-process termiantes prior to the Cleanup.
var ErrEarlyTerm = errors.New("env: sub-process exited early")

// CleanupFn is the cleanup hook function prototype.
type CleanupFn func()

// TestInstanceInfo contains information of the current test run.
type TestInstanceInfo struct {
	// Test is the name of the test.
	Test string `json:"test"`

	// Instance is the instance name of the test. e.g. oasis-test-runner123456
	Instance string `json:"instance"`

	// ParameterSet is the paramater set the test was run with.
	ParameterSet map[string]string `json:"parameter_set"`

	// Run is the number of run.
	Run int `json:"run"`
}

// Env is a (nested) test environment.
type Env struct {
	name string

	parent     *Env
	parentElem *list.Element
	children   *list.List

	dir         *Dir
	testInfo    TestInstanceInfo
	cleanupFns  []CleanupFn
	cleanupCmds []*cmdMonitor
	cleanupLock sync.Mutex

	isInCleanup bool
}

// Name returns the environment name.
func (env *Env) Name() string {
	return env.name
}

// Dir returns the path to this test environment's data directory.
func (env *Env) Dir() string {
	return env.dir.String()
}

// CurrentDir returns the test environment's Dir.
func (env *Env) CurrentDir() *Dir {
	return env.dir
}

// NewSubDir creates a new subdirectory under the test environment.
func (env *Env) NewSubDir(subDirName string) (*Dir, error) {
	return env.dir.NewSubDir(subDirName)
}

// TestInfo returns the test instance information.
func (env *Env) TestInfo() TestInstanceInfo {
	return env.testInfo
}

// AddOnCleanup adds a cleanup routine to be called durring the environment's
// cleanup.  Routines will be called in reverse order that they were
// registered.
func (env *Env) AddOnCleanup(fn CleanupFn) {
	env.cleanupLock.Lock()
	defer env.cleanupLock.Unlock()

	env.cleanupFns = append([]CleanupFn{fn}, env.cleanupFns...)
}

// AddTermOnCleanup adds a process that will be terminated during the
// environment's cleanup, and will return a channel that will be
// closed (after an error is sent if applicable when the process
// terminates.
//
// Process will be terminated in the reverse order that they were
// registered.
//
// Processes are torn down *BEFORE* the on-cleanup hooks are run.
func (env *Env) AddTermOnCleanup(cmd *exec.Cmd) chan error {
	env.cleanupLock.Lock()
	defer env.cleanupLock.Unlock()

	m := &cmdMonitor{
		env:    env,
		cmd:    cmd,
		doneCh: make(chan error),
	}
	go m.wait()

	env.cleanupCmds = append([]*cmdMonitor{m}, env.cleanupCmds...)

	return m.doneCh
}

// Cleanup cleans up all of the environment's children, followed by the
// environment.
//
// Note: Unless the env is a root (top-level) environment, the directory
// will not be cleaned up.
func (env *Env) Cleanup() {
	env.cleanupLock.Lock()
	env.isInCleanup = true
	env.cleanupLock.Unlock()

	// Remove this from the parent's children list.
	if env.parentElem != nil {
		env.parent.children.Remove(env.parentElem)
	}

	for {
		childElem := env.children.Front()
		if childElem == nil {
			break
		}

		child := childElem.Value.(*Env)
		child.Cleanup()
	}

	// Tear down this environment's commands.
	for _, v := range env.cleanupCmds {
		v.termOrKill()
	}

	// Do the cleanup for this environment.
	for _, v := range env.cleanupFns {
		v()
	}

	// If this is the root, clean up the entire directory tree.
	if env.parent == nil {
		env.dir.Cleanup()
	}
}

// NewChild returns a new child test environment.
func (env *Env) NewChild(childName string, testInfo TestInstanceInfo) (*Env, error) {
	var parentDir *Dir
	if env.parent != nil {
		parentDir = env.parent.dir
	} else {
		parentDir = env.dir
	}

	subDir, err := parentDir.NewSubDir(childName)
	if err != nil {
		return nil, err
	}

	child := &Env{
		name:     childName,
		parent:   env,
		children: list.New(),
		dir:      subDir,
		testInfo: testInfo,
	}
	child.parentElem = env.children.PushBack(child)

	return child, nil
}

// WriteParamSetToFile dumps test instance parameter set to test_instance_info.json file for debugging afterwards.
func (env *Env) WriteTestInstanceInfo() error {
	b, err := json.Marshal(env.testInfo)
	if err != nil {
		return err
	}
	if err = ioutil.WriteFile(env.Dir()+"/test_instance_info.json", b, 0644); err != nil {
		return err
	}

	return nil
}

// New creates a new root test environment.
func New(dir *Dir) *Env {
	return &Env{
		children: list.New(),
		dir:      dir,
	}
}

type cmdMonitor struct {
	env    *Env
	cmd    *exec.Cmd
	doneCh chan error
}

func (m *cmdMonitor) wait() {
	defer close(m.doneCh)

	err := m.cmd.Wait()

	m.env.cleanupLock.Lock()
	defer m.env.cleanupLock.Unlock()
	if !m.env.isInCleanup {
		if err == nil {
			err = ErrEarlyTerm
		}
		m.doneCh <- err
	} else {
		m.doneCh <- nil
	}
}

func (m *cmdMonitor) termOrKill() {
	// Send a SIGTERM and wait a bit first to see if the process will
	// gracefully terminate.
	_ = m.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-time.After(5 * time.Second):
	case <-m.doneCh:
		return
	}

	// Send a SIGKILL and blow the process away.
	_ = m.cmd.Process.Kill()
	<-m.doneCh
}
