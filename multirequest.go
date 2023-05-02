package main

import (
	"log"
	"sync"
)

func RequestMultiple(fns ...func() error) error {
	e := make(chan error, len(fns))
	var wg sync.WaitGroup
	wg.Add(len(fns))
	for i := range fns {
		go func(j int) {
			err := fns[j]()
			if err != nil {
				log.Println("function", j, "returned error", err)
				e <- err
			}
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
