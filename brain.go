package main

import (
	"runtime"
	"sync/atomic"
)

const (
	CacheLineSize = 64
)

type RingBuffer struct {
	capacity uint64
	mask     uint64
	_        [CacheLineSize]byte

	writeIndex uint64
	_          [CacheLineSize - 8]byte

	readIndex uint64
	_         [CacheLineSize - 8]byte

	cycleState []uint64
	ids        []uint64
	prices     []float64
	qtys       []uint32
}

func Newbuffer(capacity uint64) *RingBuffer {
	buffer := &RingBuffer{
		capacity:   capacity,
		mask:       capacity - 1,
		writeIndex: 0,
		readIndex:  0,
		cycleState: make([]uint64, capacity),
		ids:        make([]uint64, capacity),
		prices:     make([]float64, capacity),
		qtys:       make([]uint32, capacity),
	}

	for i := uint64(0); i < capacity; i++ {
		buffer.cycleState[i] = i
	}

	return buffer
}

func (rb *RingBuffer) Enqueue(id uint64, price float64, qty uint32) bool {
	var head uint64
	var offset uint64
	var cycleVal uint64
	var diff int64

	for {
		head = atomic.LoadUint64(&rb.writeIndex)
		offset = head & rb.mask
		cycleVal = atomic.LoadUint64(&rb.cycleState[offset])
		
		diff = int64(cycleVal) - int64(head)

		if diff == 0 {
			if atomic.CompareAndSwapUint64(&rb.writeIndex, head, head+1) {
				break
			}
		} else if diff < 0 {
			return false
		}
	}

	rb.ids[offset] = id
	rb.prices[offset] = price
	rb.qtys[offset] = qty
	atomic.StoreUint64(&rb.cycleState[offset], head+1)
	return true
}

func (rb *RingBuffer) EnqueueBatch(ids []uint64, prices []float64, qtys []uint32) uint64 {
	count := uint64(len(ids))
	var head uint64
	var offset uint64
	var cycleVal uint64
	var diff int64

	for {
		head = atomic.LoadUint64(&rb.writeIndex)
		
		offset = head & rb.mask
		cycleVal = atomic.LoadUint64(&rb.cycleState[offset])
		diff = int64(cycleVal) - int64(head)
		
		if diff < 0 {
			return 0 
		}
		if diff == 0 {
			tailOffset := (head + count - 1) & rb.mask
			tailCycle := atomic.LoadUint64(&rb.cycleState[tailOffset])
			
			if int64(tailCycle) - int64(head + count - 1) < 0 {
				return 0 
			}
			if atomic.CompareAndSwapUint64(&rb.writeIndex, head, head+count) {
				for i := uint64(0); i < count; i++ {
					idx := (head + i) & rb.mask
					
					rb.ids[idx] = ids[i]
					rb.prices[idx] = prices[i]
					rb.qtys[idx] = qtys[i]
					atomic.StoreUint64(&rb.cycleState[idx], head + i + 1)
				}
				return count
			}
		}
	}
}

func (rb *RingBuffer) DequeueBatch(ids []uint64, prices []float64, qtys []uint32) uint64 {
	var tail uint64
	var offset uint64
	var cycleVal uint64
	
	limit := uint64(len(ids))
	if limit == 0 { return 0 }

	for {
		tail = atomic.LoadUint64(&rb.readIndex)
		
		offset = tail & rb.mask
		cycleVal = atomic.LoadUint64(&rb.cycleState[offset])

		if int64(cycleVal) - int64(tail+1) < 0 {
			return 0 
		}

		tailOffset := (tail + limit - 1) & rb.mask
		tailCycle := atomic.LoadUint64(&rb.cycleState[tailOffset])
		if int64(tailCycle) - int64(tail + limit) < 0 {
			return 0 
		}

		if atomic.CompareAndSwapUint64(&rb.readIndex, tail, tail+limit) {
			for i := uint64(0); i < limit; i++ {
				currIndex := tail + i
				currOffset := currIndex & rb.mask
				
				for {
					c := atomic.LoadUint64(&rb.cycleState[currOffset])
					if c == currIndex + 1 { break }
					runtime.Gosched()
				}

				ids[i]    = rb.ids[currOffset]
				prices[i] = rb.prices[currOffset]
				qtys[i]   = rb.qtys[currOffset]
				
				atomic.StoreUint64(&rb.cycleState[currOffset], currIndex + rb.capacity)
			}
			return limit
		}
	}
}

func (rb *RingBuffer) Dequeue(id *uint64, price *float64, qty *uint32) bool {
	var tail uint64
	var offset uint64
	var cycleVal uint64
	var diff int64

	for {
		tail = atomic.LoadUint64(&rb.readIndex)
		offset = tail & rb.mask
		cycleVal = atomic.LoadUint64(&rb.cycleState[offset])

		diff = int64(cycleVal) - int64(tail+1)

		if diff == 0 {
			if atomic.CompareAndSwapUint64(&rb.readIndex, tail, tail+1) {
				break
			}
		} else if diff < 0 {
			return false
		}
	}

	*id = rb.ids[offset]
	*price = rb.prices[offset]
	*qty = rb.qtys[offset]

	atomic.StoreUint64(&rb.cycleState[offset], tail+rb.capacity)
	return true
}