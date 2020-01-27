package dcgm

import (
	"fmt"
	"sync"
)

type publisher struct {
	publish        chan interface{}
	close          chan bool
	subscribers    []*subscriber
	subscriberLock sync.Mutex
}

type subscriber struct {
	read  chan interface{}
	close chan bool
}

func newPublisher() *publisher {
	pub := &publisher{
		publish: make(chan interface{}),
		close:   make(chan bool),
	}
	return pub
}

func (p *publisher) subscriberList() []*subscriber {
	p.subscriberLock.Lock()
	defer p.subscriberLock.Unlock()
	return p.subscribers[:]
}

func (p *publisher) add() *subscriber {
	p.subscriberLock.Lock()
	defer p.subscriberLock.Unlock()
	newSub := &subscriber{
		read:  make(chan interface{}),
		close: make(chan bool),
	}
	p.subscribers = append(p.subscribers, newSub)
	return newSub
}

func (p *publisher) remove(leaving *subscriber) error {
	p.subscriberLock.Lock()
	defer p.subscriberLock.Unlock()
	subscriberIndex := -1
	for i, sub := range p.subscribers {
		if sub == leaving {
			subscriberIndex = i
			break
		}
	}
	if subscriberIndex == -1 {
		return fmt.Errorf("Could not find subscriber")
	}
	go func() { leaving.close <- true }()
	p.subscribers = append(p.subscribers[:subscriberIndex], p.subscribers[subscriberIndex+1:]...)
	return nil
}

func (p *publisher) send(val interface{}) {
	p.publish <- val
}

func (p *publisher) broadcast() {
	for {
		select {
		case publishing := <-p.publish:
			for _, sub := range p.subscriberList() {
				go func(s *subscriber, val interface{}) {
					s.read <- val
				}(sub, publishing)
			}
		case <-p.close:
			return
		}
	}
}

func (p *publisher) closePublisher() {
	p.close <- true
}
