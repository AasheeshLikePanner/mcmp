

# Lock-Free MPMC Ring Buffer in Go

A **lock-free, multi-producer / multi-consumer (MPMC) ring buffer** implemented **purely in Go**, from scratch.

This implementation is designed for **high-throughput, low-latency systems** where Go channels become a bottleneck. It avoids locks entirely, performs **zero heap allocations**, and scales aggressively with batch size.

---

## Why this exists

Go channels are great for correctness and simplicity, but they trade performance for safety:

* Locks under the hood
* Scheduler involvement
* Allocation pressure in hot paths

For workloads like:

* Market data ingestion
* Event pipelines
* Trading systems
* Telemetry / logging
* Real-time streaming

…channels quickly become the limiting factor.

This ring buffer is built to remove that ceiling.

---

## Key properties

* **Lock-free** (CAS + memory ordering only)
* **Multi-producer / multi-consumer**
* **Zero allocations per operation**
* **Cache-line padded** to avoid false sharing
* **Batch-aware API** for linear scaling
* **Pure Go** — no cgo, no unsafe tricks

---

## Performance

Measured on a modern multi-core machine.

### Memory

```
0 allocs/op
```

### Throughput

```
Batch size 16 → ~78M ops/sec
Batch size 32 → ~146M ops/sec
```

That’s **4x+ faster than Go channels**, and performance continues to scale as batch size increases.

---

## Architecture overview

At a high level:

* The ring buffer size is a **power of two**
* Indices are masked instead of modulo
* Each slot has a **cycle state** to track ownership
* Producers and consumers coordinate using **atomic CAS**
* Cache lines are padded to prevent false sharing between:

  * write index
  * read index
  * slot metadata

### Slot lifecycle

1. Producer waits until the slot’s cycle matches the expected write cycle
2. Producer claims the slot via CAS
3. Data is written
4. Cycle is advanced to signal readiness
5. Consumer waits until cycle signals availability
6. Consumer reads data
7. Cycle is advanced to mark the slot reusable

No locks. No queues. No condition variables.

---

## API

### Create a buffer

```go
rb := Newbuffer(1024) // must be power of two
```

### Single enqueue / dequeue

```go
ok := rb.Enqueue(id, price, qty)
ok := rb.Dequeue(&id, &price, &qty)
```

### Batch operations (recommended)

```go
written := rb.EnqueueBatch(ids, prices, qtys)
read    := rb.DequeueBatch(ids, prices, qtys)
```

Batch APIs are where this structure really shines — fewer CAS operations, better cache locality, higher throughput.

---

## Output examples

Below are example outputs from benchmark runs and visualizations of throughput scaling.

<img width="904" height="346" alt="image" src="https://github.com/user-attachments/assets/e5b3a400-4c3e-4da0-9990-be56ad953586" />
<img width="904" height="346" alt="image" src="https://github.com/user-attachments/assets/980d6f6b-8b11-4ff5-b3b4-b23ef6aa834e" />

---

## When you should use this

Use this if you:

* Need **extreme throughput**
* Care about **tail latency**
* Control buffer sizing
* Are comfortable with lock-free concurrency semantics

Do **not** use this if you:

* Need unbounded queues
* Prefer simplicity over performance
* Are unfamiliar with concurrent memory models

This is a performance tool, not a general abstraction.

---

## Notes & constraints

* Buffer capacity **must be a power of two**
* No blocking semantics — enqueue/dequeue return immediately
* Backpressure must be handled by the caller
* Fairness is not guaranteed (by design)

---

## Inspiration

The design is inspired by classic ring buffers used in:

* LMAX Disruptor
* High-frequency trading systems
* Low-latency message queues

Adapted carefully to Go’s memory model and atomic primitives.

---

If you want, I can also:

* Add a **benchmark section** with `go test -bench`
* Add **architecture diagrams**
* Refactor this into a reusable package (non-`main`)

Just tell me how far you want to take ite
