package IM

import (
	"time"
	"errors"
	"log"
)


/*
* Message types are strings define here
*/
const (
	TextMessageType = "TextMessage"
	PictureMessageType = "PictureMessage"
	FileMessageType = "FileMessage"
)


/*
* Message defines base element and convert method
*/
type Message interface {
	Id() uint64				//Get Message Id
	SetId(uint64)				//Set Message Id

	OnBinary() ([]byte, error)		//Convert message to binary to persistent
	OnReceived()				//Called when message is just received
	Wait(time.Duration) error		//Wait until message finish or time out

	ResetMessage(string, interface{})	//Reset message when recycle
	Finish(error)				//Finish a message
	Content() interface{}			//Get Message Content
	Type() string				//Get Message Type
	TargetId() string			//Get Message target id
	SenderId() string			//Get Message sender id
	SetTargetId(string)			//Set Message Type id
	SetSenderId(string)			//Set Sender

	IsGroupMessage() bool			//If This Message Is Group Message
	GroupName() string			//Group Name
	SetGroup(string)			//Set Group
}


/*
* A Simple Message instance
*/
type DefaultMessage struct {
	id uint64

	messageType string
	target string
	sender string

	isGroup bool
	groupName string

	errorChan chan error

	content interface{}
}

func (m *DefaultMessage) ResetMessage(messageType string, c interface{}) {
	m.messageType = messageType
	m.content = c
}
func (m *DefaultMessage) OnReceived() {}
func (m *DefaultMessage) Content() interface{} { return m.content }
func (m *DefaultMessage) Type() string { return m.messageType }
func (m *DefaultMessage) Id() uint64 { return m.id }
func (m *DefaultMessage) SetId(id uint64) { m.id = id }
func (m *DefaultMessage) TargetId() string { return m.target }
func (m *DefaultMessage) SenderId() string { return m.sender }
func (m *DefaultMessage) SetTargetId(id string) { m.target = id }
func (m *DefaultMessage) SetSenderId(id string) { m.sender = id }
func (t *DefaultMessage) Wait(d time.Duration) error {
	select {
	case err := <- t.errorChan :
		return err
	case <- time.After(d) :
		return errors.New("Message handle time out")
	}
}
func (t *DefaultMessage) Finish(e error) { t.errorChan <- e }
func (t *DefaultMessage) SetGroup(g string) {
	t.isGroup = true
	t.groupName = g
}
func (t *DefaultMessage) IsGroupMessage() bool {
	return t.isGroup
}
func (t *DefaultMessage) GroupName() string {
	return t.groupName
}


/*
* Some common types of messages are defined here
*/

//TextMessage
type TextMessage struct {
	DefaultMessage
}

func NewTextMessage(content string) Message {
	return &TextMessage{
		DefaultMessage{
			messageType        : TextMessageType,
			content                : content,
			errorChan        : make(chan error, 1),
		},
	}
}

func (t *TextMessage) OnReceived() { log.Printf("Message received(%d), target: %v\n", t.Id(), t.TargetId()) }
func (t *TextMessage) OnBinary() ([]byte, error) {
	if s, ok := t.Content().(string); ok {
		return []byte(s), nil
	} else {
		return []byte(""), errors.New("Invalid Text Message")
	}
}



//PictureMessage
type PictureMessage struct {
	DefaultMessage

	suffix string
}
func NewPictureMessage(content []byte, suffix string) *PictureMessage {
	return &PictureMessage{
		DefaultMessage : DefaultMessage {
			messageType	: PictureMessageType,
			content		: content,
			errorChan	: make(chan error, 1),
		},

		suffix		: suffix,
	}
}
func (m *PictureMessage) OnReceived() { log.Print("Picture message received") }
func (m *PictureMessage) OnBinary() ([]byte, error) {
	if bs, ok := m.content.([]byte); ok {
		return bs, nil
	} else {
		return nil, errors.New("Invalid Picture message")
	}
}
func (m *PictureMessage) Suffix() string { return m.suffix }


//FileMessage
type FileMessage struct {
	DefaultMessage

	filename string
}
func NewFileMessage(content []byte, filename string) *FileMessage {
	return &FileMessage{
		DefaultMessage : DefaultMessage {
			messageType	: FileMessageType,
			content		: content,
			errorChan	: make(chan error, 1),
		},

		filename		: filename,
	}
}
func (m *FileMessage) OnReceived() { log.Print("Picture message received") }
func (m *FileMessage) OnBinary() ([]byte, error) {
	if bs, ok := m.content.([]byte); ok {
		return bs, nil
	} else {
		return nil, errors.New("Invalid Picture message")
	}
}
func (m *FileMessage) FileName() string { return m.filename }


