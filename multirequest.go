package main

import (
	"sync"
)

func RequestMultiple(fns ...func(ech chan<- error)) error {
	e := make(chan error, len(fns))
	var wg sync.WaitGroup
	wg.Add(len(fns))
	for i := range fns {
		go func(j int) {
			fns[j](e)
			wg.Done()
		}(i)
	}
	wg.Wait()
	select {
	case x, ok := <-e:
		if ok {
			return x
		}
	default:
	}
	return nil
}
