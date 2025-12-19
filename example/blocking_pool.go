package main

type BlockingPool[T any] struct {
	pool chan T
}

func NewBlockingPool[T any](capacity int) BlockingPool[T] {
	return BlockingPool[T]{pool: make(chan T, capacity)}
}

func (p *BlockingPool[T]) Get() T    { return <-p.pool }
func (p *BlockingPool[T]) Put(obj T) { p.pool <- obj }
