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
	"runtime"
	"sync/atomic"
	"time"
)

// REVU: TODO: try various flavors of vars.

// processer cache-line sized structure
type clb struct {
	data [8]uint64
}

type mutatorTask func(*clb, int, int, chan string)

/// basic setup and main boiler plate ////////////////////////////

var iters int = 1000 * 1000 * 100

func init() {
	flag.IntVar(&iters, "n", iters, "number of mutator access ops")
}

func main() {
	fmt.Printf("Salaam!\n")
	fmt.Printf("comparative test of concurrent counter mutators using explicit CAS and atomic Addders\n")

	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())
	run(iters)
}

func run(iters int) {
	casDelta := clbAccess("access-with-CAS", iters, CASAdder, CASSubtracter)
	atomicDelta := clbAccess("access-with-Atomic", iters, AtomicAdder, AtomicSubtracter)

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
	fmt.Println("\n ------------------------------")
	fmt.Printf("report: %s access faster by %d nsecs (%d nsec/mutation-op)\n", info, diff, perMOP)
}

func clbAccess(id string, iters int, tasks ...mutatorTask) int64 {
	fmt.Printf("\n--- %s\n", id)

	var delta int64
	var wcnt = len(tasks)
	if wcnt == 0 {
		fmt.Printf("NOP %s with no tasks provided\n")
		return delta
	}

	done := make(chan string, wcnt)
	var v clb

	v.data[0] = 1 // help out the subtracter
	// begin
	start0 := time.Now().UnixNano()
	for _, task := range tasks {
		go task(&v, 0, iters, done)
	}

	var acks int
	for acks < wcnt {
		fmt.Printf("ack (%s)\n", <-done)
		acks++
	}
	delta = time.Now().UnixNano() - start0
	close(done)
	// end

	fmt.Printf("\n\tdelta: %d v.data[0]:%d [%s]\n", delta, v.data[0], id)

	return delta
}

/// using atomic adders ////////////////////////////////////

func AtomicAdder(p *clb, idx, n int, done chan string) {
	ptr := &(p.data[idx])
	tries := 0
	for i := 0; i < n; i++ {
		atomic.AddUint64(ptr, uint64(1))
		tries++ // unneessary ; keeping timings measures ~ fair
	}
	done <- fmt.Sprintf("AtomicAdder        (%d)", tries)
}
func AtomicSubtracter(p *clb, idx, n int, done chan string) {
	ptr := &(p.data[idx])
	tries := 0

	for atomic.LoadUint64(ptr) == 0 {
	}
	for i := 0; i < n; i++ {
		atomic.AddUint64(ptr, ^uint64(0))
		tries++ // unneessary ; keeping timings measures ~ fair
	}
	done <- fmt.Sprintf("AtomicSubtracter   (%d)", tries)
}

/// using atomic CAS /////////////////////////////////////

func CASAdder(p *clb, idx, n int, done chan string) {
	ptr := &(p.data[idx])
	tries := 0
	for i := 0; i < n; i++ {
		for {
			tries++
			v0 := atomic.LoadUint64(ptr)
			v := v0 + 1
			if atomic.CompareAndSwapUint64(ptr, v0, v) {
				break
			}
			runtime.Gosched()
		}
	}
	done <- fmt.Sprintf("CASAdder           (%d)", tries)
}

func CASSubtracter(p *clb, idx, n int, done chan string) {
	ptr := &(p.data[idx])
	tries := 0

	for atomic.LoadUint64(ptr) == 0 {
	}
	for i := 0; i < n; i++ {
		for {
			tries++
			v0 := atomic.LoadUint64(ptr)
			v := v0 - 1
			if atomic.CompareAndSwapUint64(ptr, v0, v) {
				break
			}
			runtime.Gosched()
		}
	}
	done <- fmt.Sprintf("CASSubtractrer     (%d)", tries)
}