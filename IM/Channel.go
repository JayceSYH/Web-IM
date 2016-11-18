package IM

import (
	"sync/atomic"
	"time"
	"errors"
	"log"
)

const (
	DefaultChannelBufferSize = 10
)

/*
* Channel type defines how to initiate a type of channel
*/
type ChannelNewer func () Channel

/*
* A channel handles a kind of message
*/
type Channel interface {
	HandleMessage(Message)			//Handle Method used to handle message in channel loop
	StartChannelLoop(*ChannelGroup)		//Start the target channel loop
	Metrics() interface{}			//Get metrics info
	IsStarted() bool			//If the channel is started
	Stop()					//Stop the channel
}


/*
* Manage a group of channels, a broker between channel and classifier
*/
type ChannelGroup struct {
	mt string 				//Message type of the group
	mc []MessageClassifier			//Message classifiers
	incomingMessage chan Message		//Incoming message chan
	sendingMessage chan Message		//Sending message chan
	channels [] Channel			//A group of channels handle the same type of message
	gm GroupManager				//A group manager of this channel group
}

/*
* A group manager manage a channel group, such as metric manage and so on
*/
type GroupManager interface {
	StartManage(*ChannelGroup)
}

/*
* Create a new channel group
* Params:
* t 			: type of the channel
* mt			: type of message to be handled
* channelBufferSize 	: max buffer size of the channel
*/
func NewChannelGroup(n ChannelNewer, mt string, channelNum uint32, channelBufferSize uint32) *ChannelGroup {
	if (channelNum < 0) {
		channelNum = 0
	}
	if (channelBufferSize < 1) {
		channelBufferSize = 1
	}

	g := ChannelGroup{
		mt              : mt,
		incomingMessage : make(chan Message, channelBufferSize),
		sendingMessage  : make(chan Message, channelBufferSize),
		channels        : make([] Channel, channelNum),
	}

	for i := range g.channels {
		g.channels[i] = n()
	}

	return &g
}

/*
* Register message classifiers
*/
func (g *ChannelGroup) SetMessageClassifiers(mcs []MessageClassifier) {
	g.mc = mcs
}

/*
* Start channels of the group and register channel group in message classifier
*/
func (g *ChannelGroup) StartChannels() {
	if len(g.mc) == 0 {
		panic("No Message Classifier Binded")
	}

	for _, c := range g.channels {
		c.StartChannelLoop(g)
	}

	//register channel group
	for _, mc := range g.mc {
		mc.OnChannelGroupRegister(g)
	}
}

/*
* Stop channels
*/
func (g *ChannelGroup) StopChannels() {
	for _, mc := range g.mc {
		mc.OnChannelGroupUnRegister(g)
	}

	for _, c := range g.channels {
		c.Stop()
	}

	for {
		select {
		case m := <-g.incomingMessage:
			m.Finish(errors.New("Channel stoped"))
		default:
			goto exit
		}
	}
	exit:
}

func (g *ChannelGroup) Metrics() []interface{} {
	metrics := make([]interface{}, len(g.channels))
	for i, c := range g.channels {
		metrics[i] = c.Metrics()
	}

	return metrics
}


/*
* Some types of channels are defined here
*/

/*******Base Channel*********/
type BaseChannel struct {
	stateFlag uint32
}

func (c *BaseChannel) Metrics() interface {} { return nil }
func (c *BaseChannel) IsStarted() bool { return atomic.LoadUint32(&c.stateFlag) == 1 }
func (c *BaseChannel) Stop() { atomic.StoreUint32(&c.stateFlag, 0) }
func (c *BaseChannel) HandleMessage(Message) {}
func (c *BaseChannel) StartChannelLoop(g *ChannelGroup)  {
	if atomic.LoadUint32(&c.stateFlag) == 1 {
		log.Print("Channel Already Started")
		return
	}

	atomic.StoreUint32(&c.stateFlag, 1)

	go func() {
		//Restart Channel when it's down
		defer func() {
			if err := recover(); err != nil {
				atomic.StoreUint32(&c.stateFlag, 0)
				log.Print(err)
				log.Print("Channel Restart Automatically...")
				c.StartChannelLoop(g)
			}
		}()
		for {
			if atomic.LoadUint32(&c.stateFlag) == 0 {
				break
			}

			message := <-g.incomingMessage

			message.OnReceived()

			/*
			* Handle messages
			*/
			c.HandleMessage(message)

			/*
			* Send message to message pool
			*/
			g.sendingMessage <- message
		}
	}()
}
func NewBaseChannel() Channel {
	return &BaseChannel{}
}




/******Metrics Channel*******/
type MetricsChannel struct {
	flow uint64
	createTime time.Time

	BaseChannel
}

type MetricState struct {
	flow uint64
	createTime time.Time
	runningTime time.Duration
}

func (c *MetricsChannel) HandleMessage(m Message) {
	atomic.AddUint64(&c.flow, 1)
}
func (c *MetricsChannel) Metrics() interface{} {
	flow := atomic.LoadUint64(&c.flow)
	start := c.createTime
	return MetricState{
		flow        : flow,
		createTime  : start,
		runningTime : time.Now().Sub(start),
	}
}


func NewMetricsChannel() Channel {
	return &MetricsChannel{
		flow       : 0,
		createTime : time.Now(),
	}
}


