<a href="https://zerodha.tech"><img src="https://zerodha.tech/static/images/github-badge.svg" align="right" /></a>

# profiler

profiler is a tiny wrapper around runtime/pprof for easily profiling Go programs. It allows running one or more profiles (cpu, mem etc.) simultaneously while avoiding boilerplate. The results are dumped to files. This is a rewrite of [pkg/profile](https://github.com/pkg/profile/) with the key difference of supporting multiple concurrent profiles.

## Install
```shell
go get github.com/knadh/profiler
````

## Usage
```go
import "github.com/knadh/profiler"

func main() {
	// Pass one or more modes: Block, Cpu, Goroutine, Mem, Mutex, ThreadCreate, Trace ...
	p := profiler.New(profiler.Conf{}, profiler.Cpu, profiler.Mem ...)
	p.Start()

	// Stuff ...

	p.Stop()
}
```

### Optional config
```go
profiler.Conf{
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
```
