package utils

import (
	"reflect"
	"sync"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type Executor func(interface{}, *gitlab.Client, int, ...any) (interface{}, error)
type Pipeline interface {
	Pipe(executor Executor, searchOption ...any) Pipeline
	Merge() <-chan interface{}
}

type pipeline struct {
	client    *gitlab.Client
	dataC     chan interface{}
	errC      chan error
	executors []Executor
	options   [][]any
}

func New(f func(chan interface{}), client *gitlab.Client, searchOption ...any) Pipeline {
	inC := make(chan interface{})
	go f(inC)
	return &pipeline{
		client:    client,
		dataC:     inC,
		errC:      make(chan error),
		executors: []Executor{},
		options:   [][]any{},
	}
}

func (p *pipeline) Pipe(executor Executor, searchOption ...any) Pipeline {
	p.executors = append(p.executors, executor)
	p.options = append(p.options, searchOption)
	return p
}
func (p *pipeline) Merge() <-chan interface{} {
	for i := 0; i < len(p.executors); i++ {
		p.dataC, p.errC = run(p.dataC, p.executors[i], p.client, p.options[i]...)
	}
	return p.dataC
}
func run(
	inC <-chan interface{},
	f Executor,
	client *gitlab.Client,
	searchOption ...any,
) (chan interface{}, chan error) {
	outC := make(chan interface{})
	errC := make(chan error)

	workerCount := 200
	var wg sync.WaitGroup

	go func() {
		defer close(outC)
		defer close(errC)

		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func(workerId int) {
				defer wg.Done()
				for v := range inC {
					res, err := f(v, client, i, searchOption...)
					if err != nil {
						errC <- err
						continue
					}

					resValue := reflect.ValueOf(res)
					if resValue.Kind() == reflect.Slice || resValue.Kind() == reflect.Array {
						for j := 0; j < resValue.Len(); j++ {
							outC <- resValue.Index(j).Interface()
						}
					} else {
						outC <- res
					}
				}
			}(i)
		}

		wg.Wait()

	}()
	return outC, errC
}
