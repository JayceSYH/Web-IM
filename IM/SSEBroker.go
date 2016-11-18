package IM

import (
	"net/http"
	"errors"
	"reflect"
	"sync"
	"log"
)

type SSEBroker struct {
	mutex sync.Mutex

	im *IM
	parses map[string]MessageParser
	race bool

	filters[] MessageFilter
}

func NewSSEBroker(im *IM) *SSEBroker {
	return &SSEBroker{
		im 			: im,
		parses			: make(map[string]MessageParser),

		filters			: make([]MessageFilter, 0, 10),
	}
}

type MessageFilter interface {
	Filter([]Message) []Message;
}

type MessageParser interface {
	ParseMessage([]Message) []*Frame
}

type MessageFuncParser func([]Message) []*Frame

func (p MessageFuncParser) ParseMessage(ms []Message) []*Frame {
	return p(ms)
}

func (b *SSEBroker) AddParser(messageType string, parser MessageParser) {
	b.parses[messageType] = parser
}
func (b *SSEBroker) AddParseFunc(messageType string, f func([]Message) []*Frame) {
	b.parses[messageType] = MessageFuncParser(f)
}
func (b *SSEBroker) SetRace(r bool) {
	b.race = r
}
func (b *SSEBroker) AddFilter(filter MessageFilter) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.filters = append(b.filters, filter)
}
func (b *SSEBroker) ClearFilter() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.filters = make([]MessageFilter, 0)
}
func (b *SSEBroker) StartProxy(id string, w http.ResponseWriter) error {
	//check for sse
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return errors.New("Streaming unsupported!")
	}

	//Set Headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Add(`Access-Control-Allow-Headers`, `Content-Type`)
	w.Header().Set(`Content-Type`, `text/event-stream;charset=utf-8`)
	w.Header().Set("Connection", "keep-alive")

	//init receivers
	parserNum := len(b.parses)
	receivers := make([]*MessageReceiver, parserNum)

	count := 0
	for mt := range b.parses{
		if rec, err := b.im.ReceiveMessages(id, mt, b.race); err == nil {
			receivers[count] = rec
			count++
		} else {
			log.Printf("SSEBroker error: Fail to init receiver(%s):\n", mt)
			log.Print(err)
		}
	}

	if count == 0 {
		return errors.New("No usable receiver")
	}

	//init select cases
	cases := make([]reflect.SelectCase, parserNum)

	for i, r := range receivers {
		cases[i] = reflect.SelectCase{ Dir : reflect.SelectRecv, Chan : reflect.ValueOf(r.ReceiveChan) }
	}

	//init close notifier
	notify := w.(http.CloseNotifier).CloseNotify()
	finish := make(chan int)

	go func() {
		<-notify

		for _, r := range receivers {
			r.Stop()
		}
	}()

	//main loop
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Print("SSEBroker Error:", err)
			}

			close(finish)
		}()

		ms := make([]Message, 1)
		for {
			_, m, ok := reflect.Select(cases)

			if !ok {
				break
			}

			if message, ok := m.Interface().(Message); ok {
				ms[0] = message
			}

			if len(b.filters) != 0 {
				b.mutex.Lock()
				for _, filter := range b.filters {
					ms = filter.Filter(ms)
				}
				b.mutex.Unlock()
			}

			if len(ms) > 0 {
				for _, frame := range b.parses[ms[0].Type()].ParseMessage(ms) {
					w.Write(frame.ToBytes())
					f.Flush()
				}
			}
		}
	}()

	<-finish

	log.Print("connection closed")
	return nil
}