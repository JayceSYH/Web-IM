package IM

import (
	"net/http"
	"fmt"
)

const (
	DefaultFILERootPath = "IM_TEMP_FILE"
)

/*
* Communication controls the communication connection between client and server
* User need to define the onConnection function to set id parameter for identifying a message consumer
*/
type Communication struct {
	im *IM
	broker *SSEBroker
	imageProxy *FileProxy
	fileProxy *FileProxy
}

func NewCommunication(im *IM) *Communication {
	return &Communication{
		im		: im,
		broker		: NewSSEBroker(im),
		imageProxy	: NewFileProxy(DefaultFILERootPath, im.host),
		fileProxy	: NewFileProxy(DefaultFILERootPath, im.host),
	}
}

func (c *Communication) FetchFile(name string) ([]byte, error) {
	if f, err := c.imageProxy.FetchFile(name); err == nil {
		return f, nil
	} else if f, err := c.fileProxy.FetchFile(name); err == nil {
		return f, nil
	} else {
		return nil, err
	}
}

/*
* Parser func used to parse relative message
*/
func (c *Communication) parseText(ms []Message) []*Frame {
	fs := make([]*Frame, len(ms))
	for i, m := range ms {
		if s, ok := m.Content().(string); ok {
			fs[i] = NewFrame(TextMessageType, s, m.SenderId())
		} else { fs[i] = NewFrame(TextMessageType, "", m.SenderId()) }
	}

	return fs
}
func (c *Communication) parseImage(ms []Message) []*Frame {
	fs := make([]*Frame, len(ms))

	for i, m := range ms {
		if pm, ok := m.(*PictureMessage); ok {
			if bs, ok := pm.Content().([]byte); ok {
				url := c.imageProxy.AddDisposalFileWithRestfulAPI(bs, pm.Suffix())
				fs[i] = NewFrame(PictureMessageType, url, m.SenderId())
			} else { fs[i] = NewFrame(PictureMessageType, "", m.SenderId()) }
		} else { fs[i] = NewFrame(PictureMessageType, "", m.SenderId()) }
	}

	return fs
}
func (c *Communication) parseFile(ms []Message) []*Frame {
	fs := make([]*Frame, len(ms))

	for i, m := range ms {
		if pm, ok := m.(*FileMessage); ok {
			if bs, ok := pm.Content().([]byte); ok {
				url := c.fileProxy.AddDisposalNamedFileWithRestfulAPI(bs, pm.FileName())
				fs[i] = NewFrame(FileMessageType, url, m.SenderId())
			} else { fs[i] = NewFrame(PictureMessageType, "", m.SenderId()) }
		} else { fs[i] = NewFrame(PictureMessageType, "", m.SenderId()) }
	}

	return fs
}

/*
* Note:Due to route problem, serve function are implemented in file route.go
* The url format is:
* communicationURL/(checkCode)
*/
func (c *Communication) Start(w http.ResponseWriter, r *http.Request, checkCode string) {
	if id, err := c.im.Validate(checkCode); err != nil {
		fmt.Println(err)
		return
	} else {
		c.broker.AddParseFunc(TextMessageType, c.parseText)
		c.broker.AddParseFunc(PictureMessageType, c.parseImage)
		c.broker.AddParseFunc(FileMessageType, c.parseFile)
		c.broker.StartProxy(id, w)
	}
}
