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

// NOTICE-3rd party copyrighted material.
// see inlined comments

package main

import (
	"flag"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
)

var (
	cpus = runtime.NumCPU()
	wcnt = cpus * 2
)

func main() {
	fmt.Printf("Salaam!\n")
	flag.IntVar(&cpus, "p", cpus, "number of cpus for GOMAXPROCS")
	flag.IntVar(&wcnt, "w", wcnt, "number of goroutines")
	flag.Parse()
	fmt.Printf("[%v] using cpus:%d wcnt:%d\n", time.Now(), cpus, wcnt)
	runtime.GOMAXPROCS(cpus)

	TestStoreLoadSeqCst32()
	TestCASLock()
}

type cline [8]uint64

func newCL() *cline {
	var cl cline
	return &cl
}

func TestCASLock() {
	var cl = newCL()

	// consistently, if wcnt is a power of 2 mulitple of even n, with GOMAXPROCS(n), it will livelock
	// but other variations will also block, e.g. cpu @ 3 and wcnt @ 9 although they may make more progress

	var done = make(chan struct{}, wcnt)
	var op = func(id uint64, cl *cline, done chan struct{}) {
		cl.put(id, id)
		done <- struct{}{}
	}

	for i := 1; i <= wcnt; i++ {
		go op(uint64(i), cl, done)
	}
	for i := 1; i <= wcnt; i++ {
		<-done
	}
	fmt.Println("TestCASLock - done")
}

func (clb *cline) put(key, value uint64) (removed bool, k0 uint64, idx uint8) {
	if key == 0 {
		panic("ERR - key is nil")
	}
	var xof = 0
	var index0, index1 uint64
	var state uint8
	// lock
	for {
		index0 = atomic.LoadUint64(&clb[xof+0])
		state = uint8(index0 & 0xFF00000000000000 >> 56)
		if state == 0 {
			index1 = index0 | 0x0100000000000000
			if atomic.CompareAndSwapUint64(&clb[xof+0], index0, index1) {
				fmt.Printf("locked %03d [%064b]\n", key, index1)
				break
			}
		}
		//	runtime.Gosched() // #fix#
	}
	/* including either of the following causes some sort of livelock
	 * runtime.Gosched()
	 * time.Sleep(n) // n > 0 ; (n = 0 is fine)
	 */
	//	runtime.Gosched()
	//	time.Sleep(1) // comment out and it is #fix#

	fmt.Printf("tryunlock %d\n", key) // lock owner *never* reaches this line ..

	if !atomic.CompareAndSwapUint64(&clb[xof+0], index1, index0) {
		panic(fmt.Errorf("BUG - can't cas index %d", key))
	}
	fmt.Printf("unlocked %d\n", key)
	return
}

// NOTICE-3RD PARTY
// Modified version of the eponymous function.
// original source: http://golang.org/src/sync/atomic/atomic_test.go#1263
//
// -- BEGIN --
// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// -- END --
//
// Original test uses 2 workers in context of 4 logical CPUs.
// Modified to use options, per other tests.
func TestStoreLoadSeqCst32() {
	if runtime.NumCPU() == 1 {
		fmt.Printf("Skipping test on %v processor machine", runtime.NumCPU())
		return
	}
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(cpus))
	N := int32(1e3)

	c := make(chan bool, wcnt)
	X := make([]int32, wcnt)
	ack := make([][3]int32, wcnt)
	for i, _ := range ack {
		ack[i] = [3]int32{-1, -1, -1}
	}
	for p := 0; p < wcnt; p++ {
		go func(me int) {
			he := wcnt - me - 1
			fmt.Printf("debug - me:%d he:%d\n", me, he)
			for i := int32(1); i < N; i++ {
				atomic.StoreInt32(&X[me], i)
				my := atomic.LoadInt32(&X[he])
				atomic.StoreInt32(&ack[me][i%3], my)
				for w := 1; atomic.LoadInt32(&ack[he][i%3]) == -1; w++ {
					// REVU: why yeild?
					//					if w%1000 == 0 {
					//					runtime.Gosched()
					//					}
				}
				his := atomic.LoadInt32(&ack[he][i%3])
				if (my != i && my != i-1) || (his != i && his != i-1) {
					panic(fmt.Errorf("invalid values: %d/%d (%d)", my, his, i))
				}
				if my != i && his != i {
					panic(fmt.Errorf("store/load are not sequentially consistent: %d/%d (%d)", my, his, i))
				}
				atomic.StoreInt32(&ack[me][(i-1)%3], -1)
			}
			c <- true
		}(p)
	}
	lim := wcnt
	for lim > 0 {
		<-c
		lim--
	}
	fmt.Println("TestStoreLoadSeqCst32 - done")
}
