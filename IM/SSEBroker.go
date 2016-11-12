package IM

import (
	"net/http"
	"fmt"
	"errors"
	"reflect"
)

type SSEBroker struct {
	im *IM
	parses map[string]MessageParser
	race bool
}

func NewSSEBroker(im *IM) *SSEBroker {
	return &SSEBroker{
		im 			: im,
		parses			: make(map[string]MessageParser),
	}
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
			fmt.Printf("SSEBroker error: Fail to init receiver(%s):\n", mt)
			fmt.Println(err)
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
				fmt.Println("SSEBroker Error:", err)
			}

			close(finish)
		}()

		for {
			chose, _, ok := reflect.Select(cases)

			if !ok {
				break
			}


			ms := receivers[chose].WithdrawMessages()

			for _, frame := range b.parses[ms[0].Type()].ParseMessage(ms) {
				w.Write(frame.ToBytes())
				f.Flush()
			}
		}
	}()

	<-finish

	fmt.Println("connection closed")
	return nil
}