package utils

import (
	"reflect"
	"sync"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type Executor func(interface{}, *gitlab.Client) (interface{}, error)
type Pipeline interface {
	Pipe(executor Executor) Pipeline
	Merge() <-chan interface{}
}

type pipeline struct {
	client    *gitlab.Client
	dataC     chan interface{}
	errC      chan error
	executors []Executor
}

func New(f func(chan interface{}), client *gitlab.Client) Pipeline {
	inC := make(chan interface{})
	go f(inC)
	return &pipeline{
		client:    client,
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
		p.dataC, p.errC = run(p.dataC, p.executors[i], p.client)
	}
	return p.dataC
}
func run(
	inC <-chan interface{},
	f Executor,
	client *gitlab.Client,
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
					res, err := f(v, client)
					if err != nil {
						errC <- err
						continue
					}

					resValue := reflect.ValueOf(res)
					if resValue.Kind() == reflect.Slice || resValue.Kind() == reflect.Array {
						for i := 0; i < resValue.Len(); i++ {
							outC <- resValue.Index(i).Interface()
						}
					} else {
						outC <- res
					}
				}
			}()
		}

		wg.Wait()

	}()
	return outC, errC
}
