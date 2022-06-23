// Package profiler is a simple abstraction for easily running runtime
// profiles and storing the results to files.
package profiler

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sync/atomic"
)

const (
	// Profiler modes.
	Block = iota
	Cpu
	Goroutine
	Mem
	Mutex
	ThreadCreate
	Trace
)

type profile struct {
	File  string
	Start func(f io.Writer)
	Stop  func(f io.Writer)

	f io.Writer
}

// Conf contains the profiler config.
type Conf struct {
	// Directory path to dump the profile output to. Default is current directory.
	DirPath string

	// Quiet disables info log output.
	Quiet bool

	// NoShutdownHook controls whether the profiling package should
	// hook SIGINT to automatically Stop().
	NoShutdownHook bool

	// MemProfileRate is the rate for the memory profiler. Default is 4096.
	// To include every allocated block in the profile, set MemProfileRate to 1.
	MemProfileRate int

	// MemProfileType = heap or alloc. Default is heap.
	MemProfileType string
}

// Profiler represents an active profiling session.
type Profiler struct {
	c                 Conf
	oldMemProfileRate int
	profiles          []*profile
	log               *log.Logger
}

// Flag to block concurrent Start() of the profiler.
var running uint32

// New returns a new Profiler. One or more modes can be provided.
// eg: `prof := New(profiler.Cpu, profiler.Mem ...)`
// Configuration can be directly applied once `prof` is initiailized.
// eg: `prof.Path = "./otuput"`
func New(c Conf, modes ...int) *Profiler {
	if len(modes) == 0 {
		modes = append(modes, Cpu)
	}
	if c.MemProfileRate < 1 {
		c.MemProfileRate = 4096
	}
	if c.MemProfileType != "heap" && c.MemProfileType != "alloc" {
		c.MemProfileType = "heap"
	}

	// Setup the output directory.
	if c.DirPath != "" {
		if err := os.MkdirAll(c.DirPath, 0777); err != nil {
			log.Fatalf("error creating output directory '%s': %v", c.DirPath, err)
		}
	}

	prof := &Profiler{c: c}

	// Initialize the logger.
	if prof.c.Quiet {
		prof.log = log.New(ioutil.Discard, "", log.Ldate|log.Ltime)
	} else {
		prof.log = log.New(os.Stdout, "profiler: ", log.Ldate|log.Ltime)
	}

	// Initialize the requested profile modes.
	all := prof.all()
	for _, mode := range modes {
		if p, ok := all[mode]; ok {
			prof.profiles = append(prof.profiles, p)
		}
	}

	return prof
}

func (prof *Profiler) Start() {
	if !atomic.CompareAndSwapUint32(&running, 0, 1) {
		log.Fatal("profiler is already running")
	}

	// If shutdown hooks are enabled, listen to SIGINT and automatically Stop().
	if !prof.c.NoShutdownHook {
		go func() {
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			<-c

			prof.log.Println("caught SIGINT. stopping.")
			prof.Stop()
			os.Exit(0)
		}()
	}

	// Start the profilers.
	for _, pr := range prof.profiles {
		path := filepath.Join(prof.c.DirPath, pr.File)

		f, err := os.Create(path)
		if err != nil {
			log.Fatalf("error creating file %s: %v", path, err)
		}

		prof.log.Printf("will dump to %s", path)
		pr.f = f
		pr.Start(f)
	}

	atomic.StoreUint32(&running, 1)
}

// Stop runs all the profile stop functions.
func (pr *Profiler) Stop() {
	for _, p := range pr.profiles {
		pr.log.Printf("finishing %s", path.Join(pr.c.DirPath, p.File))
		p.Stop(p.f)

		// Close the file handler.
		p.f.(*os.File).Close()
	}
}

func (pr *Profiler) all() map[int]*profile {
	return map[int]*profile{
		Cpu: {
			File:  "cpu.pprof",
			Start: func(f io.Writer) { pprof.StartCPUProfile(f) },
			Stop:  func(f io.Writer) { pprof.StopCPUProfile() },
		},
		Mem: {
			File: "mem.pprof",
			Start: func(f io.Writer) {
				// Record the old rate to reset the profiler on Stop().
				pr.oldMemProfileRate = runtime.MemProfileRate
				runtime.MemProfileRate = pr.c.MemProfileRate
			},
			Stop: func(f io.Writer) {
				pprof.Lookup(pr.c.MemProfileType).WriteTo(f, 0)
				runtime.MemProfileRate = pr.oldMemProfileRate
			},
		},
		Mutex: {
			File:  "mutex.pprof",
			Start: func(f io.Writer) { runtime.SetMutexProfileFraction(1) },
			Stop: func(f io.Writer) {
				if mp := pprof.Lookup("mutex"); mp != nil {
					mp.WriteTo(f, 0)
				}
				runtime.SetMutexProfileFraction(0)
			},
		},
		Block: {
			File:  "block.pprof",
			Start: func(f io.Writer) { runtime.SetBlockProfileRate(1) },
			Stop: func(f io.Writer) {
				pprof.Lookup("block").WriteTo(f, 0)
				runtime.SetBlockProfileRate(0)
			},
		},
		ThreadCreate: {
			File:  "threadcreate.pprof",
			Start: func(f io.Writer) {},
			Stop: func(f io.Writer) {
				if mp := pprof.Lookup("threadcreate"); mp != nil {
					mp.WriteTo(f, 0)
				}
			},
		},
		Trace: {
			File: "trace.out",
			Start: func(f io.Writer) {
				if err := trace.Start(f); err != nil {
					pr.log.Fatalf("profile: could not start trace: %v", err)
				}
			},
			Stop: func(f io.Writer) { trace.Stop() },
		},
		Goroutine: {
			File:  "goroutine.pprof",
			Start: func(f io.Writer) {},
			Stop: func(f io.Writer) {
				if mp := pprof.Lookup("goroutine"); mp != nil {
					mp.WriteTo(f, 0)
				}
			},
		},
	}
}
