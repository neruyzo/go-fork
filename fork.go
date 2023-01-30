package fork

import (
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"syscall"
)

// A Function struct describes a fork process.  Usually this is only used internally, but if you want a bit more control of the sub-process,
// say, to assing new namespaces to the child, this will provide it.
//
// A Fork is a wrapper around an `exec.Cmd` with some of parts the data structure exposed (that we don't need to control directly).
type Function struct {
	// SysProcAttr holds optional, operating system-sepcific attributes.
	SysProcAttr *syscall.SysProcAttr
	// Process will hold the os.Process once the Fork has been Run().
	Process *os.Process
	// ProcessState will hold the os.ProcessState after Wait() has been called.
	ProcessState *os.ProcessState
	// Name is the string we use to identify this func
	Name string
	// Where to send stdout (default: os.Stdout)
	Stdout *os.File
	// Where to send stderr (default: os.Stderr)
	Stderr *os.File
	// Where to get stdin (default: os.Stdin)
	Stdin *os.File

	// contains filtered or unexported fields
	Command  exec.Cmd
	fn reflect.Value
}

// NewFork createas and initializes a Fork
// A Fork object can be manipluated to control how a process is launched.
// E.g. you can set new namespaces in the SysProcAttr property...
//      or, you can set custom args with the (optional) variatic args aparameters.
//      If you set args, the first should be the program name (Args[0]), which may
//		Which may or may not match the  executable.
// If no args are specified, args is set to []string{os.Args[0]}
func NewFork(n string, fn interface{}, args ...string) (f *Function) {
	f = &Function{}
	f.Command = exec.Cmd{}
	// os.Executable might not be the most robust way to do this, but it is portable.
	f.Command.Path, _ = os.Executable()
	f.Command.Args = args
	f.Command.Stderr = os.Stderr
	f.Command.Stdout = os.Stdout
	f.Command.Stdin = os.Stdin
	if len(args) == 0 {
		f.Command.Args = []string{os.Args[0]}
	}
	// we don't check for errors here, but it would be a pretty bad thing if this failed
	//f.c.Args = func() []string { s, _ := ioutil.ReadFile("/proc/self/comm"); return []string{string(s)} }()
	f.fn = reflect.ValueOf(fn)
	if f.fn.Kind() != reflect.Func {
		return nil
	}
	f.Name = n
	return
}

// Fork starts a process and prepares it to call the defined fork
func (f *Function) Fork(args ...interface{}) (err error) {
	if err = f.validateArgs(args...); err != nil {
		return
	}
	f.Command.Stderr = f.Stderr
	f.Command.Stdout = f.Stdout
	f.Command.Stdin = f.Stdin
	f.Command.SysProcAttr = f.SysProcAttr
	f.Command.Env = os.Environ()
	f.Command.Env = append(f.Command.Env, nameVar+"="+f.Name)
	af, err := ioutil.TempFile("", "gofork_*")
	f.Command.Env = append(f.Command.Env, argsVar+"="+af.Name())
	if err != nil {
		return
	}
	enc := gob.NewEncoder(af)
	for _, iv := range args {
		enc.EncodeValue(reflect.ValueOf(iv))
	}
	af.Close()
	if err = f.Command.Start(); err != nil {
		return
	}
	f.Process = f.Command.Process
	return
}

// Combine NewFork and Fork with privious function configuration
func (f *Function) ReFork(args ...interface{}) (err error) {
	previous := f.Command
	f.Command = exec.Cmd{}
	f.Command.Path, _ = os.Executable()
	f.Command.Args = previous.Args
	f.Command.Stderr = f.Stderr
	f.Command.Stdout = f.Stdout
	f.Command.Stdin = f.Stdin
	f.Command.SysProcAttr = f.SysProcAttr
	f.Command.Env = os.Environ()
	f.Command.Env = append(f.Command.Env, nameVar+"="+f.Name)
	af, err := ioutil.TempFile("", "gofork_*")
	f.Command.Env = append(f.Command.Env, argsVar+"="+af.Name())
	if err != nil {
		return
	}
	enc := gob.NewEncoder(af)
	for _, iv := range args {
		enc.EncodeValue(reflect.ValueOf(iv))
	}
	af.Close()
	if err = f.Command.Start(); err != nil {
		return
	}
	f.Process = f.Command.Process
	return
}

// Wait provides a wrapper around exec.Cmd.Wait()
func (f *Function) Wait() (err error) {
	if err = f.Command.Wait(); err != nil {
		return
	}
	f.ProcessState = f.Command.ProcessState
	return
}

// private

func (f *Function) validateArgs(args ...interface{}) (err error) {
	t := f.fn.Type()
	if len(args) != t.NumIn() {
		return fmt.Errorf("incorrect number of args for: %s", t.String())
	}
	for i := 0; i < t.NumIn(); i++ {
		if t.In(i).Kind() != reflect.TypeOf(args[i]).Kind() {
			return fmt.Errorf("argument mismatch (1) %s != %s", reflect.TypeOf(args[i]).Kind(), t.In(i).Kind())
		}
	}
	return
}
