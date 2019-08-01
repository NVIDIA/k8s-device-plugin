package main

import "sync"

// Stor define a storage interface for the data
type Stor interface {
}

type memStor struct {
	data map[string]string
	lock sync.RWMutex
}

func NewMemStor() Stor {
	ms = &memStor{
		data: map[string]string{},
		lock: sync.RWMutex{},
	}
}

func (ms *memStor) Add(origin, new string) {
	ms.lock.Lock()
	defer ms.lock.Unlock()
	ms.data[origin] = new
}

func (ms *memStor) Delete(origin, new string) {
	ms.lock.Lock()
	defer ms.lock.Unlock()
	delete(ms.data, origin)
}
