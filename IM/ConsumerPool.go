package IM

import (
	"time"
	"fmt"
	"sync"
	"sync/atomic"
	"errors"
	"runtime"
)

const (
	defaultBuffSize = 20
	defaultTimeout = time.Second * 3
)

/*
* A consumer pool receive message from only one message group
* and then cache or persistent the message for a target user
*/
type ConsumerPool interface {
	OnConsumerExpired(*Consumer) error				//If message is handled, return nil, else return some error to notify sender
	OnRequestedConsumerMiss(id string) (*Consumer, error) 	//If message is handled, set err to nil, else return some error to notify sender
	OnMessageTargetMiss(Message) error 				//If message is handled, return nil, else return some error to notify sender
	Consume(Message)						//Consume message
	Start(chan Message)						//Start consumer pool
	Stop()								//Stop consumer pool
	ReceiveMessages(string, bool) (*MessageReceiver, error)	//Get a receiver which is a broker between consumer and user
	StopReceiveMessages(string) error				//unregister a message consumer
}

/*
* Default consumer pool creator
*/
func NewConsumerPool(onConsumerExpired func(*Consumer) error,
	onRequestedConsumerMiss func(string)(*Consumer, error),
	onMessageTargetMiss func(Message, ConsumerPool) error) ConsumerPool {

	return &DefaultConsumerPool{
		stopFlag			: 0,
		consumers			: make(map[string]*Consumer),
		onConsumerExpiredCallback	: onConsumerExpired,
		onRequestedConsumerMissCallback : onRequestedConsumerMiss,
		onMessageTargetMissCallback	: onMessageTargetMiss,
	}
}

/*
* A base consumer pool implementation
*/
type DefaultConsumerPool struct {
	stopFlag uint32								//Set to 1 when the pool is force to stop
	running uint32								//Set 1 when consumer pool is running

	mutex sync.RWMutex							//mutex_

	consumers map[string]*Consumer					//consumers
	onConsumerExpiredCallback func(*Consumer) error				//Called when consumer is going to be delete
	onRequestedConsumerMissCallback func(string)(*Consumer, error)	//Called when requesting an not cached consumer
	onMessageTargetMissCallback func(Message, ConsumerPool) error		//Called when message target is not cached
	ticker time.Ticker							//Ticker to set expire
}
func (p *DefaultConsumerPool) OnConsumerExpired(c *Consumer) error {
	return p.onConsumerExpiredCallback(c)
}
func (p *DefaultConsumerPool) OnRequestedConsumerMiss(id string) (*Consumer, error) {
	return p.onRequestedConsumerMissCallback(id)
}
func (p *DefaultConsumerPool) OnMessageTargetMiss(m Message) error { return p.onMessageTargetMissCallback(m, p); }
func (p *DefaultConsumerPool) Consume(m Message) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if _, ok := p.consumers[m.TargetId()]; !ok {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					_, file, line, _ := runtime.Caller(0)
					fmt.Println(err, file, line)
				}
			}()
			err := p.OnMessageTargetMiss(m)
			m.Finish(err)
		}()
	} else {
		p.consumers[m.TargetId()].Consume(m)
	}
}
func (p *DefaultConsumerPool) Start(incoming chan Message) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				_, file, line, _ := runtime.Caller(0)
				fmt.Println(err, file, line)

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

	for _, c := range p.consumers {
		c.FinishMessages(errors.New("Consumer stoped"))
	}
}
func (p *DefaultConsumerPool) ReceiveMessages(id string, race bool) (*MessageReceiver, error) {

	if c, ok := p.consumers[id]; ok {
		if race {
			c.CloseReceiver()
			c.BindReceiver(NewMessageReceiver(p, c))
			return c.GetReceiver(), nil
		} else {
			return nil, errors.New("Target consumer is alread listened")
		}
	} else {
		var consumer *Consumer = nil
		done := make(chan int, 1)

		go func() {
			defer func() {
				if err := recover(); err != nil {
					_, file, line, _ := runtime.Caller(0)
					fmt.Println(err, file, line)
				}
			}()
			consumer, _ = p.OnRequestedConsumerMiss(id)

			done <- 1
		}()

		//routine finished or time out
		select {
		case <- done:
		case <- time.After(defaultTimeout):
		}

		if consumer != nil {
			//update consumer pool
			p.mutex.Lock()
			p.consumers[id] = consumer
			p.consumers[id].BindReceiver(NewMessageReceiver(p, consumer))
			p.mutex.Unlock()

			return p.consumers[id].GetReceiver(), nil
		} else {
			return nil, errors.New("Consumer initialization failed")
		}
	}
}
func (p *DefaultConsumerPool) StopReceiveMessages(id string) error {
	done := make(chan int, 1)
	go func() {
		p.OnConsumerExpired(p.consumers[id])

		done <- 1
	}()

	var err error

	select {
	case <- done:
		err = nil
	case <- time.After(defaultTimeout):
		err = errors.New("Consumer expired callback time out")
	}

	p.mutex.Lock()
	delete(p.consumers, id)
	p.mutex.Unlock()

	return err
}

/*
* A user handler of a message consumer
* user receiveChan to receive messages
* when stop receive, call stop() function
*/
type MessageReceiver struct {
	ReceiveChan chan int			//This chan notify user that message is coming
	state uint32				//Is this receiver is in use
	stopCallback func()			//Called the receiver is closed
	withdrawCallback func() []Message	//Called when user withdraws messages
}

/*
* Stop receive message and expire consumer
*/
func (r *MessageReceiver) Stop() {
	r.stopCallback()
	if atomic.CompareAndSwapUint32(&r.state, 1, 0) {
		close(r.ReceiveChan)
	}
}
/*
* Unbind Message Consumer
*/
func (r *MessageReceiver) UnBind() {
	if atomic.CompareAndSwapUint32(&r.state, 1, 0) {
		close(r.ReceiveChan)
	}
}
/*
* withdraw messages buffered in consumer
*/
func (r *MessageReceiver) WithdrawMessages() []Message {
	return r.withdrawCallback()
}

/*
* Create a new message receiver
*/
func NewMessageReceiver(pool ConsumerPool, c *Consumer) *MessageReceiver {
	return &MessageReceiver{
		ReceiveChan	: make(chan int),
		state		: 1,
		stopCallback	: func() { pool.StopReceiveMessages(c.id) },
		withdrawCallback: func() []Message { return c.WithdrawMessages() },
	}
}

/*
* A consumer represents a message consumer,
* for instance, a consumer may be a user in some app
*/
type Consumer struct {
	notified uint32			//Is user is already notified
	id string			//Target id

	mutex sync.Mutex		//mutex_

	messages []Message		//messages cached by consumer
	receiver *MessageReceiver	//Pointer of a message receiver
}

func NewConsumer(id string, initBuffSize int) *Consumer {
	if (initBuffSize < 1) {
		initBuffSize = defaultBuffSize
	}
	return &Consumer{
		id         : id,
		messages   : make([]Message, 0, defaultBuffSize),
	}
}
func (c *Consumer) Consume(m Message) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.messages = append(c.messages, m)
	if atomic.LoadUint32(&c.notified) == 0 {
		atomic.StoreUint32(&c.notified, 1)
		c.receiver.ReceiveChan <- 1
	}
}
func (c *Consumer) WithdrawMessages() []Message {
	c.FinishMessages(nil)

	c.mutex.Lock()
	ret := c.messages
	c.messages = make([]Message, 0, defaultBuffSize)
	atomic.StoreUint32(&c.notified, 0)
	c.mutex.Unlock()

	return ret
}
func (c *Consumer) FinishMessages(err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, m := range c.messages {
		m.Finish(err)
	}
}
func (c *Consumer) BindReceiver(r *MessageReceiver) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.receiver = r
	if len(c.messages) > 0 {
		c.receiver.ReceiveChan <- 1
		atomic.StoreUint32(&c.notified, 1)
	}
}
func (c *Consumer) GetReceiver() *MessageReceiver {
	c.mutex.Lock()
	c.mutex.Unlock()

	return c.receiver
}
func (c *Consumer) CloseReceiver() {
	c.mutex.Lock()
	c.mutex.Unlock()

	atomic.StoreUint32(&c.notified, 0)
	c.receiver.UnBind()
	c.receiver = nil
}