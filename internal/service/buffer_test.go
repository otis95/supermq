package service

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestBuffer(t *testing.T) {
	var (
		buf, _       = newBuffer(defaultBufferSize)
		wg           sync.WaitGroup
		producerCost int64
		consumerCost int64
		consumerSize int
	)
	wg.Add(2)
	// producer
	go func() {
		var (
			source = [1024 * 1024 * 512]byte{}
			r      = bytes.NewReader(source[:])
		)

		startT := time.Now().UnixNano()
		_, err := buf.ReadFrom(r)
		endT := time.Now().UnixNano()
		buf.Close()

		fmt.Printf("err: %s\n", err.Error())
		producerCost = endT - startT
		wg.Done()
	}()

	// consumer
	go func() {
		var (
			source = make([]byte, 0, 1024*1024*512)
			w      = bytes.NewBuffer(source)
		)

		startT := time.Now().UnixNano()
		_, err := buf.WriteTo(w)
		endT := time.Now().UnixNano()

		fmt.Printf("err: %s\n", err.Error())
		consumerCost = endT - startT
		consumerSize = w.Len()
		wg.Done()
	}()

	wg.Wait()
	fmt.Printf("[Producer] wait=%d ,throughput=%.4f(B/ns), cost=%d(ns)\n", buf.pwait, float64(1024*1024*512)/float64(producerCost), producerCost)
	fmt.Printf("[Consumer] wait=%d ,throughput=%.4f(B/ns), size=%d ,cost=%d(ns)\n", buf.cwait, float64(consumerSize)/float64(consumerCost), consumerSize, consumerCost)
}
