// Package hooktest contains utilities for testing gocharm hooks.
package hooktest

import (
	"encoding/json"

	"github.com/juju/gocharm/hook"
)

// Find out which commands will be generated by which Context methods.

// Runner is implemention of hook.Runner suitable for use in tests.
// It calls the given RunFunc function whenever the Run method
// and records all the calls in the Record field, with the
// exception of the calls mentioned below.
//
// Any calls to juju-log are logged using Logger, but otherwise ignored.
// Calls to config-get from the Config field and not invoked through RunFunc.
// Likewise, calls to unit-get will be satisfied from the PublicAddress
// and PrivateAddress fields.
type Runner struct {
	RegisterHooks func(r *hook.Registry)
	// The following fields hold information that will
	// be available through the hook context.
	Relations   map[hook.RelationId]map[hook.UnitId]map[string]string
	RelationIds map[string][]hook.RelationId
	Config      map[string]interface{}

	PublicAddress  string
	PrivateAddress string

	// State holds the persistent state.
	// If it is nil, it will be set to a hooktest.MemState
	// instance.
	State hook.PersistentState

	// RunFunc is called when a hook tool runs.
	// It may be nil, in which case it will be assumed
	// that the hook tool runs successfully with no output.
	RunFunc func(string, ...string) ([]byte, error)
	Record  [][]string
	Logger  interface {
		Logf(string, ...interface{})
	}

	// Close records whether the Close method has been called.
	Closed bool
}

// RunHook runs a hook in the context of the Runner. If it's a relation
// hook, then relId should hold the current relation id and
// relUnit should hold the unit that the relation hook is running for.
//
// Any hook tools that have been run will be stored in r.Record.
func (runner *Runner) RunHook(hookName string, relId hook.RelationId, relUnit hook.UnitId) error {
	if runner.State == nil {
		runner.State = make(MemState)
	}
	r := hook.NewRegistry()
	runner.RegisterHooks(r)
	hook.RegisterMainHooks(r)
	hctxt := &hook.Context{
		UUID:        UUID,
		Unit:        "someunit/0",
		CharmDir:    "/nowhere",
		HookName:    hookName,
		Runner:      runner,
		Relations:   runner.Relations,
		RelationIds: runner.RelationIds,
	}
	if relId != "" {
		hctxt.RelationId = relId
		hctxt.RemoteUnit = relUnit
	loop:
		for name, ids := range runner.RelationIds {
			for _, id := range ids {
				if id == hctxt.RelationId {
					hctxt.RelationName = name
					break loop
				}
			}
		}
		if hctxt.RelationName == "" {
			panic("relation id not found")
		}
	}
	return hook.Main(r, hctxt, runner.State)
}

// Run implements hook.Runner.Run.
func (r *Runner) Run(cmd string, args ...string) ([]byte, error) {
	if cmd == "juju-log" {
		if len(args) != 1 {
			panic("expected exactly one argument to juju-log")
		}
		r.Logger.Logf("%s", args[0])
		return nil, nil
	}
	switch cmd {
	case "config-get":
		var val interface{}
		if len(args) < 4 {
			// config-get --format json
			val = r.Config
		} else {
			// config-get --format json -- key
			key := args[3]
			val = r.Config[key]
		}
		data, err := json.Marshal(val)
		if err != nil {
			panic(err)
		}
		return data, nil
	case "unit-get":
		if len(args) != 1 {
			panic("expected exactly one argument to unit-get")
		}
		switch args[0] {
		case "public-address":
			return []byte(r.PublicAddress), nil
		case "private-address":
			return []byte(r.PrivateAddress), nil
		default:
			panic("unexpected argument to unit-get")
		}
	}
	rec := []string{cmd}
	rec = append(rec, args...)
	r.Record = append(r.Record, rec)
	if r.RunFunc != nil {
		return r.RunFunc(cmd, args...)
	}
	return nil, nil
}

// Run implements hook.Runner.Close.
// It panics if called more than once.
func (r *Runner) Close() error {
	if r.Closed {
		panic("runner closed twice")
	}
	r.Closed = true
	return nil
}

// MemState implements hook.PersistentState in memory.
// Each element of the map holds the value key stored in the state.
type MemState map[string][]byte

func (s MemState) Save(name string, data []byte) error {
	s[name] = data
	return nil
}

func (s MemState) Load(name string) ([]byte, error) {
	return s[name], nil
}

// UUID holds an arbitrary environment UUID for testing purposes.
const UUID = "373b309b-4a86-4f13-88e2-c213d97075b8"