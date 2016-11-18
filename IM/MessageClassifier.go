package IM

import (
	"sync"
	"errors"
	"log"
)

const (
	defaultClassifierNum = 1
)

/*
* Message classifier classifies different types of messages,
* and sends them to different channel group
*/
type MessageClassifier interface {
	OnChannelGroupRegister(*ChannelGroup)
	OnChannelGroupUnRegister(*ChannelGroup)
	Classify(Message)
}

func NewMessageClassifier() MessageClassifier {
	return &DefaultMessageClassifier{
		incoming	: make(chan Message),
		channelGroups	: make(map[string]*ChannelGroup),
	}
}

/*
* A default message classifier implement base function of a Classifier
*/
type DefaultMessageClassifier struct {
	mutex sync.RWMutex

	incoming chan Message
	channelGroups map[string]*ChannelGroup
}

func (c *DefaultMessageClassifier) OnChannelGroupRegister(g *ChannelGroup) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.channelGroups[g.mt] = g
}
func (c *DefaultMessageClassifier) OnChannelGroupUnRegister(g *ChannelGroup) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.channelGroups, g.mt)
}
func (c *DefaultMessageClassifier) Classify(m Message) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if g, ok := c.channelGroups[m.Type()]; ok {
		select {
		case g.incomingMessage <- m:
		default:
			log.Print("message discarded(%s)", m.Type())
			m.Finish(errors.New("message discarded(channel group is busy)"))
		}
	}else {
		m.Finish(errors.New("no channel defined for such message"))
	}
}
