package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

const (
	TotalEvents  = 10_000_000
	BufferSize   = 1024 * 16
	NumProducers = 4
	NumConsumers = 4
	BatchSize    = 16
)

type Order struct {
	ID    uint64
	Price float64
	Qty   uint32
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	fmt.Printf("CPU Cores: %d\n", runtime.NumCPU())
	fmt.Printf("Workload:  %d events\n", TotalEvents)
	fmt.Printf("Layout:    %d Producers / %d Consumers\n", NumProducers, NumConsumers)
	fmt.Printf("BatchSize: %d\n", BatchSize)
	fmt.Println("---------------------------------------------------------")

	runChannelBenchmark()

	runRingBufferBenchmark()
}

func runChannelBenchmark() {
	fmt.Print("Running Go Channel Benchmark...  ")

	ch := make(chan Order, BufferSize)
	var wg sync.WaitGroup

	start := time.Now()

	msgsPerProducer := TotalEvents / NumProducers
	wg.Add(NumProducers)
	for p := 0; p < NumProducers; p++ {
		go func() {
			defer wg.Done()
			for i := 0; i < msgsPerProducer; i++ {
				ch <- Order{ID: uint64(i), Price: 100.0, Qty: 1}
			}
		}()
	}

	var consumerWg sync.WaitGroup
	consumerWg.Add(NumConsumers)
	for c := 0; c < NumConsumers; c++ {
		go func() {
			defer consumerWg.Done()
			for range ch {
			}
		}()
	}

	wg.Wait()
	close(ch)
	consumerWg.Wait()

	duration := time.Since(start)
	ops := float64(TotalEvents) / duration.Seconds()
	fmt.Printf("Done in %v\n", duration)
	fmt.Printf(">> Channel Throughput:    %.0f ops/sec\n", ops)
	fmt.Println("---------------------------------------------------------")
}

func runRingBufferBenchmark() {
	fmt.Print("Running RingBuffer Batch Benchmark...  ")

	rb := Newbuffer(BufferSize)
	var wg sync.WaitGroup

	start := time.Now()

	msgsPerProducer := TotalEvents / NumProducers
	wg.Add(NumProducers)
	for p := 0; p < NumProducers; p++ {
		go func() {
			defer wg.Done()
			
			ids := make([]uint64, BatchSize)
			prices := make([]float64, BatchSize)
			qtys := make([]uint32, BatchSize)
			
			for k := 0; k < BatchSize; k++ {
				ids[k] = uint64(k)
				prices[k] = 100.0
				qtys[k] = 1
			}

			loops := msgsPerProducer / BatchSize
			
			for i := 0; i < loops; i++ {
				for rb.EnqueueBatch(ids, prices, qtys) == 0 {
					runtime.Gosched()
				}
			}
		}()
	}

	msgsPerConsumer := TotalEvents / NumConsumers
	consumerWg := sync.WaitGroup{}
	consumerWg.Add(NumConsumers)

	for c := 0; c < NumConsumers; c++ {
		go func() {
			defer consumerWg.Done()
			
			ids := make([]uint64, BatchSize)
			prices := make([]float64, BatchSize)
			qtys := make([]uint32, BatchSize)
			
			processed := 0
			for processed < msgsPerConsumer {
				n := rb.DequeueBatch(ids, prices, qtys)
				if n > 0 {
					processed += int(n)
				} else {
					runtime.Gosched()
				}
			}
		}()
	}

	wg.Wait()
	consumerWg.Wait()

	duration := time.Since(start)
	ops := float64(TotalEvents) / duration.Seconds()
	fmt.Printf("Done in %v\n", duration)
	fmt.Printf(">> RingBuffer Throughput: %.0f ops/sec\n", ops)
	fmt.Println("---------------------------------------------------------")
}