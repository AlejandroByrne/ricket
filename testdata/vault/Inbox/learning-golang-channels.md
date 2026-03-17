---
tags: [learning, golang]
---

Go channels are typed conduits for communicating between goroutines. A channel send blocks until a receiver is ready (unbuffered) or until the buffer is full (buffered). Select lets you wait on multiple channel ops.

Key patterns:
- fan-out: one sender, many receivers
- fan-in: many senders, one receiver
- done channel: signal cancellation
- pipeline: chain of goroutines passing data

See [[Goroutines and Concurrency]] for the broader context.
