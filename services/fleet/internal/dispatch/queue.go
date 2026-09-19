package dispatch

import (
	"container/heap"
	"time"
)

// QueueItem is one call awaiting a human decision in the operator work
// queue: status PENDING_APPROVAL or ESCALATED (DEC-021). The package holds
// no status logic itself — callers decide what belongs in the queue.
type QueueItem struct {
	CallID     string
	Severity   int
	EnqueuedAt time.Time
}

// Queue is the operator work queue: a max-heap over urgency, scored by
// PriorityScore (DEC-020). now is always passed in by the caller — the
// package never reads the system clock itself. The score is computed on every comparison,
// never stored.
//
// There is deliberately no Rescore/Reheapify: under a uniform linear
// AgingRate, pop order does not depend on now (see the ordering-invariance
// property test), so a periodic re-heapify would be a provable no-op. If
// aging is ever made per-severity or nonlinear, that invariance breaks and
// a Reheapify(now) tick on the sim clock becomes mandatory — see
// CLAUDE.md "Two things that look like bugs but are not".
type Queue struct {
	impl *priorityHeap
}

// NewQueue returns an empty operator work queue scored with cfg.
func NewQueue(cfg PriorityConfig) *Queue {
	h := &priorityHeap{cfg: cfg}
	heap.Init(h)
	return &Queue{impl: h}
}

// Len reports the number of calls currently waiting.
func (q *Queue) Len() int { return q.impl.Len() }

// Push adds item to the queue. now is snapshotted for this operation only.
func (q *Queue) Push(item QueueItem, now time.Time) {
	q.impl.now = now
	heap.Push(q.impl, item)
}

// Pop removes and returns the highest-urgency item at now. The second
// return is false if the queue is empty.
func (q *Queue) Pop(now time.Time) (QueueItem, bool) {
	if q.impl.Len() == 0 {
		return QueueItem{}, false
	}
	q.impl.now = now
	return heap.Pop(q.impl).(QueueItem), true
}

// Peek returns the highest-urgency item at now without removing it. The
// second return is false if the queue is empty.
func (q *Queue) Peek(now time.Time) (QueueItem, bool) {
	if q.impl.Len() == 0 {
		return QueueItem{}, false
	}
	q.impl.now = now
	return q.impl.items[0], true
}

// Remove deletes the item for callID, preserving the heap invariant. It
// reports whether an item was found and removed. Needed by DEC-019's
// invalidation and by assignment (DEC-021 c8b): this is the queue's only
// consumer.
func (q *Queue) Remove(callID string, now time.Time) bool {
	q.impl.now = now
	for i, item := range q.impl.items {
		if item.CallID == callID {
			heap.Remove(q.impl, i)
			return true
		}
	}
	return false
}

// priorityHeap is the unexported container/heap.Interface implementation.
// now is set by Queue immediately before each heap operation so every
// comparison within one operation shares a single instant.
type priorityHeap struct {
	items []QueueItem
	cfg   PriorityConfig
	now   time.Time
}

func (h *priorityHeap) Len() int { return len(h.items) }

func (h *priorityHeap) Less(i, j int) bool {
	si := PriorityScore(h.items[i].Severity, h.items[i].EnqueuedAt, h.now, h.cfg)
	sj := PriorityScore(h.items[j].Severity, h.items[j].EnqueuedAt, h.now, h.cfg)
	if si != sj {
		return si > sj // max-heap: higher urgency pops first
	}
	return h.items[i].CallID < h.items[j].CallID // stable tie-break
}

func (h *priorityHeap) Swap(i, j int) { h.items[i], h.items[j] = h.items[j], h.items[i] }

func (h *priorityHeap) Push(x any) { h.items = append(h.items, x.(QueueItem)) }

func (h *priorityHeap) Pop() any {
	old := h.items
	n := len(old)
	item := old[n-1]
	h.items = old[:n-1]
	return item
}
