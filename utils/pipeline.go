package utils

import "sync"

type Executor func(interface{}) (interface{}, error)
type Pipeline interface {
	Pipe(executor Executor) Pipeline
	Merge() <-chan interface{}
}

type pipeline struct {
	dataC     chan interface{}
	errC      chan error
	executors []Executor
}

func New(f func(chan interface{})) Pipeline {
	inC := make(chan interface{})
	go f(inC)
	return &pipeline{
		dataC:     inC,
		errC:      make(chan error),
		executors: []Executor{},
	}
}

func (p *pipeline) Pipe(executor Executor) Pipeline {
	p.executors = append(p.executors, executor)
	return p
}
func (p *pipeline) Merge() <-chan interface{} {
	for i := 0; i < len(p.executors); i++ {
		p.dataC, p.errC = run(p.dataC, p.executors[i])
	}
	return p.dataC
}
func run(
	inC <-chan interface{},
	f Executor,
) (chan interface{}, chan error) {
	outC := make(chan interface{})
	errC := make(chan error)

	workerCount := 10
	var wg sync.WaitGroup

	go func() {
		defer close(outC)
		defer close(errC)

		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for v := range inC {
					res, err := f(v)
					if err != nil {
						errC <- err
						continue
					}
					outC <- res
				}
			}()
		}

		wg.Wait()

	}()
	return outC, errC
}
