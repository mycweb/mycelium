package ringbuf

type RingBuf[T any] struct {
	buf        []T
	head, tail int
}

func New[T any](n int) RingBuf[T] {
	return RingBuf[T]{buf: make([]T, n)}
}

func (rb *RingBuf[T]) MaxLen() int {
	return len(rb.buf)
}

func (rb *RingBuf[T]) PushBack(val T) {
	tail := (rb.tail + 1)
	rb.buf[rb.tail%len(rb.buf)] = val
	rb.tail = tail
}

func (rb *RingBuf[T]) PushFront(val T) {
	head := (rb.head - 1)
	rb.buf[head%len(rb.buf)] = val
	rb.head = head
}

func (rb *RingBuf[T]) PopFront() T {
	val := rb.At(0)
	rb.head = (rb.head + 1)
	return val
}

func (rb *RingBuf[T]) PopBack() T {
	val := rb.buf[rb.Len()-1]
	rb.tail = (rb.tail - 1)
	return val
}

func (rb *RingBuf[T]) At(i int) T {
	if i > rb.Len() {
		panic(i)
	}
	return rb.buf[(rb.head+i)%len(rb.buf)]
}

func (rb *RingBuf[T]) Len() int {
	return rb.tail - rb.head
}
