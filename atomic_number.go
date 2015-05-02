// ~!!! 121 !!!~
// Go-Bikini (Go Atomic Tests)
// Copyright 2015, Joubin Muhammad Houshyar
//
// All parts of the original works in Go-Bikini are licensed under the
// GNU GENERAL PUBLIC LICENSE Version 3.
//
// The terms of this license are specified in the "LICENSE" file included
//in the Go-Bikini src repository and distribution.

// Comparative performance test of concurrent counter variable mutators
// using the sync/atomic package. Here we compare the provided atomic adders
// with equivalent functionality using the provided CAS functions.

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"time"
)

// processer cache-line sized structure
type clb struct {
	data [8]uint64
}

type report struct {
	name  string
	delta int64
}
type deltas struct {
	reported, observed int64
}

type mutatorTask func(*clb, int, int, chan *report)

/// basic setup and main boiler plate ////////////////////////////

var iters int = 1000 * 1000 * 100
var option = struct {
	iters, acnt, scnt, qlen int
	quiet                   bool
	load                    bool
}{
	iters: 1000 * 1000 * 10,
	acnt:  1,
	scnt:  1,
	qlen:  0,
	quiet: false,
	load:  false,
}

var emitter emitFn = emit

func init() {
	flag.IntVar(&option.iters, "n", option.iters, "number of mutator access ops")
	flag.IntVar(&option.acnt, "a", option.acnt, "number of counter + workers")
	flag.IntVar(&option.scnt, "s", option.scnt, "number of counter - workers")
	flag.IntVar(&option.qlen, "q", option.qlen, "chan len - defval 0 means eq. to number of workers")
	flag.BoolVar(&option.quiet, "quiet", option.quiet, "supress individual worker reports")
	flag.BoolVar(&option.load, "cpu-load", option.load, "simulate additional orthogonal load")
}

func main() {
	fmt.Printf("Salaam!\n")
	fmt.Printf("comparative test of concurrent counter mutators using explicit CAS and atomic Addders\n")

	flag.Parse()
	if option.quiet {
		emitter = quiet
	}
	if option.load {
		simulateLoad()
		fmt.Printf("with simulated cpu-load\n")
	}
	var qlen = option.qlen
	if qlen == 0 {
		qlen = option.acnt + option.scnt
	}
	fmt.Printf("with channel len %d\n", qlen)

	runtime.GOMAXPROCS(runtime.NumCPU())
	run(option.iters, option.acnt, option.scnt, qlen)
}

func tasks(acnt int, addt mutatorTask, scnt int, subt mutatorTask) []mutatorTask {
	tasks := make([]mutatorTask, acnt+scnt)
	var idx = 0
	for i := 0; i < scnt; i++ {
		tasks[idx] = subt
		idx++
	}
	for i := 0; i < acnt; i++ {
		tasks[idx] = addt
		idx++
	}
	return tasks
}

func run(iters, acnt, scnt, qlen int) {

	casDeltas := runTest("access-with-CAS", iters, qlen, tasks(acnt, CASAdder, scnt, CASSubtracter)...)
	atomicDeltas := runTest("access-with-Atomic", iters, qlen, tasks(acnt, AtomicAdder, scnt, AtomicSubtracter)...)

	fmt.Println("\n---------------------")
	displayResults("reported", casDeltas.reported, atomicDeltas.reported)
	displayResults("observed", casDeltas.observed, atomicDeltas.observed)
}

func displayResults(result string, casDelta, atomicDelta int64) {
	var diff int64
	var info = ""
	switch casDelta < atomicDelta {
	case true:
		diff = atomicDelta - casDelta
		info = "CAS"
	default:
		diff = casDelta - atomicDelta
		info = "Atomic"
	}
	perMOP := diff / int64(iters)
	fmt.Printf("%s: %6s access faster by % 10d nsecs (%d nsec/mutation-op)\n", result, info, diff, perMOP)
}

func runTest(id string, iters, qlen int, tasks ...mutatorTask) *deltas {
	fmt.Printf("--- %s\n", id)

	var wcnt = len(tasks)
	if wcnt == 0 {
		fmt.Printf("NOP %s with no tasks provided\n", id)
		return nil
	}

	done := make(chan *report, qlen)
	var v clb

	// begin
	start0 := time.Now().UnixNano()
	for _, task := range tasks {
		go task(&v, 0, iters, done)
	}

	var acks int
	var deltaZ int64
	for acks < wcnt {
		rpt := <-done
		emitter("ack (% 12d ns/access %s)\n", rpt.delta, rpt.name)
		acks++
		deltaZ += rpt.delta
	}
	dt_observed := time.Now().UnixNano() - start0
	dt := deltaZ / int64(wcnt)
	e2e_oh := dt_observed - dt // end-to-end overhead
	e2e_ohratio := float64(e2e_oh*100) / float64(dt_observed)
	close(done)
	// end

	fmt.Printf("\n\tdelta:[reported:% 12d observed:% 12d] e2e-overhead:[%d (nsec) %.3f (%%)] [%s]\n", dt, dt_observed, e2e_oh, e2e_ohratio, id)

	return &deltas{dt, dt_observed}
}

/// using atomic adders ////////////////////////////////////

func AtomicAdder(p *clb, idx, n int, done chan *report) {
	ptr := &(p.data[idx])
	start := time.Now().UnixNano()
	for i := 0; i < n; i++ {
		atomic.AddUint64(ptr, uint64(1))
	}
	delta := time.Now().UnixNano() - start
	done <- &report{"AtomicAdder", delta}
}

func AtomicSubtracter(p *clb, idx, n int, done chan *report) {
	ptr := &(p.data[idx])

	start := time.Now().UnixNano()
	for i := 0; i < n; i++ {
		atomic.AddUint64(ptr, ^uint64(0))
	}
	delta := time.Now().UnixNano() - start
	done <- &report{"AtomicSubtracter", delta}
}

/// using atomic CAS /////////////////////////////////////

func CASAdder(p *clb, idx, n int, done chan *report) {
	ptr := &(p.data[idx])

	start := time.Now().UnixNano()
	for i := 0; i < n; i++ {
		for {
			v0 := atomic.LoadUint64(ptr)
			v := v0 + 1
			if atomic.CompareAndSwapUint64(ptr, v0, v) {
				break
			}
			runtime.Gosched()
		}
	}
	delta := time.Now().UnixNano() - start
	done <- &report{"CASAdder", delta}
}

func CASSubtracter(p *clb, idx, n int, done chan *report) {
	ptr := &(p.data[idx])

	start := time.Now().UnixNano()
	for i := 0; i < n; i++ {
		for {
			v0 := atomic.LoadUint64(ptr)
			v := v0 - 1
			if atomic.CompareAndSwapUint64(ptr, v0, v) {
				break
			}
			runtime.Gosched()
		}
	}
	delta := time.Now().UnixNano() - start
	done <- &report{"CASSubtracter", delta}
}

/// helpers ////////////////////////////////////////////

type emitFn func(string, ...interface{})

func emit(fmtstr string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, fmtstr, args...)
}

func quiet(fmtstr string, args ...interface{}) {}

func simulateLoad() {
	dwcnt := runtime.NumCPU()
	loadvars := make([]int64, dwcnt)
	for dw := 0; dw < dwcnt; dw++ {
		go func(idx int) {
			for {
				loadvars[idx] += time.Now().UnixNano()
			}
		}(dw)
	}
}
