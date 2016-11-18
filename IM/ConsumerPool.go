package IM

import (
	"sync"
	"sync/atomic"
	"runtime"
	"strings"
	"container/list"
	"errors"
	"github.com/labstack/gommon/log"
)

const (
	DefaultReceiverBufferSize = 20
)

/*
* A consumer pool receive message from only one message group
* and then dispatch these messages through consumers which runs in
* its own goroutine
*/
type ConsumerPool interface {
	OnNewReceiver(id string) 					//If message is handled, set err to nil, else return some error to notify sender
	OnMessageTargetMiss(Message, string) error 			//If message is handled, return nil, else return some error to notify sender
	Consume(Message)						//Consume message
	Start(chan Message)						//Start consumer pool
	Stop()								//Stop consumer pool
	Get() *Consumer							//Get a consumer
	Recycle(c *Consumer)						//Recycle consumer
	ReceiveMessages(string, bool) (*MessageReceiver, error)		//Get a receiver which is a broker between consumer and user
	CloseReceiver(*MessageReceiver)
	Receivers(id string) (*ReceiverList, bool)
}

/*
* Default consumer pool creator
*/
func NewConsumerPool(onNewReceiver func(string),
	onMessageTargetMiss func(Message, string) error) ConsumerPool {

	pool := &DefaultConsumerPool{
		stopFlag			: 0,
		onNewReceiver			: onNewReceiver,
		onMessageTargetMissCallback	: onMessageTargetMiss,
		restConsumers			: list.New(),

		receivers			: make(map[string]*ReceiverList),
	}
	return pool
}

/*
* A list of receivers with same id. This allows many receivers listen
* to the same consumer with a unique id
*/
type ReceiverList struct {
	receivers map[*MessageReceiver]uint8
	mutex sync.Mutex
}
/*
* Clear all receivers in the receiver list
*/
func (l *ReceiverList) ClearReceivers() {
	l.mutex.Lock()
	l.mutex.Unlock()

	for r := range l.receivers {
		r.Stop()
	}

	l.receivers = make(map[*MessageReceiver]uint8)
}
func (l *ReceiverList) AddReceivers(recs... *MessageReceiver) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for _, r := range recs {
		l.receivers[r] = 0
	}
}
func NewReceiverList(recs... *MessageReceiver) *ReceiverList {
	recl := &ReceiverList{
		receivers	: make(map[*MessageReceiver]uint8),
	}

	recl.AddReceivers(recs...)

	return recl
}
/*
* A base consumer pool implementation
*/
type DefaultConsumerPool struct {
	stopFlag uint32								//Set to 1 when the pool is force to stop
	running uint32								//Set 1 when consumer pool is running

	poolMutex sync.RWMutex							//mutex_
	mutex sync.RWMutex

	//consumers map[string]*Consumer					//consumers
	restConsumers *list.List
	receivers map[string]*ReceiverList
	onNewReceiver func(string)						//Called when a new receiver with a new id is registered
	onMessageTargetMissCallback func(Message, string) error			//Called when message target is not cached
}
func (p *DefaultConsumerPool) OnNewReceiver(id string) {
	p.onNewReceiver(id)
}
func (p *DefaultConsumerPool) OnMessageTargetMiss(m Message, id string) error { return p.onMessageTargetMissCallback(m, id); }
func (p *DefaultConsumerPool) Consume(m Message) {
	cos := p.Get()
	cos.Consume(m)
}
/*
* Get Receivers with the target id from pool
*/
func (p *DefaultConsumerPool) Receivers(id string) (*ReceiverList, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if r, ok := p.receivers[id]; ok {
		return r, true
	} else {
		return nil, false
	}
}
/*
* Obtain a new consumer instance from the pool
*/
func (p *DefaultConsumerPool) Get() *Consumer {
	p.poolMutex.RLock()
	defer p.poolMutex.RUnlock()

	if p.restConsumers.Len() == 0 {
		return NewConsumer(p)
	} else {
		cos := p.restConsumers.Back()
		p.restConsumers.Remove(cos)
		return cos.Value.(*Consumer)
	}
}
/*
* Recycle a consumer instance
*/
func (p *DefaultConsumerPool) Recycle(c *Consumer) {
	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()

	p.restConsumers.PushBack(c)
}
/*
* Close and unregister a receiver from the pool
*/
func (p *DefaultConsumerPool) CloseReceiver(r *MessageReceiver) {
	rs := p.receivers[r.id]

	delete(rs.receivers, r)

}
func (p *DefaultConsumerPool) Start(incoming chan Message) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				_, file, line, _ := runtime.Caller(0)
				log.Print(err, file, line)

				p.Start(incoming)
			}
		}()
		for {
			if atomic.LoadUint32(&p.stopFlag) == 1 {
				break
			}

			m := <- incoming
			p.Consume(m)
		}
	}()
}
func (p *DefaultConsumerPool) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	atomic.StoreUint32(&p.stopFlag, 1)
}
/*
* Obtain a message receiver instance from the pool, which is used to receive message
* with the target id
*/
func (p *DefaultConsumerPool) ReceiveMessages(id string, race bool) (*MessageReceiver, error) {
	if rs, ok := p.receivers[id]; ok {
		if race {
			rec := NewMessageReceiver(p, id)

			rs.ClearReceivers()
			rs.AddReceivers(rec)

			return rec, nil
		} else {
			rec := NewMessageReceiver(p, id)
			p.receivers[id].AddReceivers(rec)

			return rec, nil
		}
	} else {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.Print(err)
				}
			}()

			p.OnNewReceiver(id)
		}()

		rec := NewMessageReceiver(p, id)
		p.mutex.Lock()
		p.receivers[id] = NewReceiverList(rec)
		p.mutex.Unlock()

		return rec, nil
	}
}


/*
* A user handler of a message target
* Use receiveChan to receive messages
* When stopping receiving, call stop() function
*/
type MessageReceiver struct {
	ReceiveChan chan Message		//This chan notify user that message is coming
	state uint32				//Is this receiver is in use
	id string

	consumePool ConsumerPool
}

/*
* Stop receive message and expire consumer
*/
func (r *MessageReceiver) Stop() {
	if atomic.CompareAndSwapUint32(&r.state, 1, 0) {
		close(r.ReceiveChan)
	}
}
/*
* Create a new message receiver
*/
func NewMessageReceiver(p ConsumerPool, id string) *MessageReceiver {
	r := &MessageReceiver{
		ReceiveChan	: make(chan Message, DefaultReceiverBufferSize),
		state		: 1,
		consumePool	: p,
		id		: id,
	}

	return r
}

/*
* A consumer is used to dispatch messages, and will be active when new task is coming
* It's block when no message is coming and will be recycled when finishing a task
*/
type Consumer struct {
	mutex sync.RWMutex
	consumerPool ConsumerPool

	messageChan chan Message
	running uint32
}

func NewConsumer(pool ConsumerPool) *Consumer {
	return &Consumer{
		consumerPool		: pool,
		messageChan		: make(chan Message),
	}
}
func (c *Consumer) Stop() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	close(c.messageChan)
}
/*
* Consume and dispatch a message
*/
func (c *Consumer) Consume(m Message) {
	if atomic.LoadUint32(&c.running) == 0 {
		atomic.StoreUint32(&c.running, 1)
		go func() {
			defer func () {
				if err := recover(); err != nil {
					log.Print(err)

					atomic.StoreUint32(&c.running, 0)
					c.consumerPool.Recycle(c)
				}
			}()

			for {
				message, open := <- c.messageChan

				if !open {
					break
				}

				c.dispatch(message)
				c.consumerPool.Recycle(c)
			}
		}()
	}

	c.messageChan <- m
}
/*
* Dispatch messages to receivers, and one message with multi targets can be
* delivered to many receivers
*/
func (c *Consumer) dispatch(m Message) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	var tids []string

	if m.IsGroupMessage() {
		targets := m.TargetId()
		tids = strings.Split(targets, ";")
	} else {
		tids = make([]string, 1)
		tids[0] = m.TargetId()
	}

	for _, tid := range tids {
		if rs, ok := c.consumerPool.Receivers(tid); ok {
			for r := range rs.receivers {
				select {
				case r.ReceiveChan <- m:
				default:
					log.Print("Message dropped because receiver chan is busy")
					m.Finish(errors.New("Message dropped because receiver chan is busy"))
				}
			}
		} else {
			go func() {
				defer func() {
					if err := recover(); err != nil {
						log.Print(err)
					}
				}()

				c.consumerPool.OnMessageTargetMiss(m, tid)
				log.Print("Message Target Miss")
			}()
		}
	}
}
