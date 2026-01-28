// a parallel operation queue.
package operationq

import (
	"container/list"
)

//go:generate counterfeiter -o fake_operationq/fake_operation.go . Operation

// The Operation interface is implemented externally, by the user of the queue.
type Operation interface {
	// Identifier for the operation's queue. Operations with the same key will be
	// executed in the order in which they were pushed. Operations with different
	// keys will be executed concurrently.
	Key() string

	// Work to execute when the operation is popped off of the queue.
	Execute()
}

//go:generate counterfeiter -o fake_operationq/fake_queue.go . Queue

// Queue executes operations, parallelized by operation key.
type Queue interface {
	// Enqueue an operation for execution.
	Push(Operation)
}

type buffer interface {
	Push(Operation) bool
	Pop() (Operation, bool)
	Len() int
}

type slidingBuffer struct {
	buffer   *list.List
	capacity int
}

func newSlidingBuffer(capacity int) buffer {
	return &slidingBuffer{
		buffer:   list.New(),
		capacity: capacity,
	}
}

func (b *slidingBuffer) Push(op Operation) bool {
	if b.capacity == 0 {
		return false
	}

	b.buffer.PushBack(op)
	if b.buffer.Len() > b.capacity {
		b.buffer.Remove(b.buffer.Front())
	}
	return true
}

func (b *slidingBuffer) Pop() (Operation, bool) {
	elem := b.buffer.Front()
	if elem == nil {
		return nil, false
	}

	b.buffer.Remove(elem)
	return elem.Value.(Operation), true
}

func (b *slidingBuffer) Len() int {
	return b.buffer.Len()
}

type multiQueue struct {
	queues       map[string]buffer
	pushChan     chan Operation
	completeChan chan string
	capacity     int
}

// NewSlidingQueue returns a queue that will buffer up to `capacity` operations
// per key. When capacity is exceeded, older operations are dequeued to make room.
func NewSlidingQueue(capacity int) Queue {
	q := &multiQueue{
		queues:       make(map[string]buffer),
		pushChan:     make(chan Operation),
		completeChan: make(chan string),
		capacity:     capacity,
	}
	go q.run()
	return q
}

func (q *multiQueue) run() {
	for {
		select {
		case queueKey := <-q.completeChan:
			queue := q.queues[queueKey]
			if queue.Len() == 0 {
				delete(q.queues, queueKey)
			} else {
				op, ok := queue.Pop()
				if ok {
					go q.execute(op)
				}
			}

		case op := <-q.pushChan:
			if queue, ok := q.queues[op.Key()]; ok {
				queue.Push(op)
			} else {
				q.queues[op.Key()] = newSlidingBuffer(q.capacity)
				go q.execute(op)
			}
		}
	}
}

func (q *multiQueue) Push(o Operation) {
	q.pushChan <- o
}

func (q *multiQueue) execute(o Operation) {
	o.Execute()
	q.completeChan <- o.Key()
}
