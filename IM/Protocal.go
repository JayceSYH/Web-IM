package IM

import "fmt"


const (
	Meta2ContentSep = "----------2918136232"

	Sender = "Sender"
	Group = "Group"
)


/*
* Frame define a piece of data sent to client
* When converted to bytes, frame has the following structure
 (Note: new line dose not mean a \n character, in sse, no \n or \r is allowed)
* MessageType\033
* MetaKey1:MetaContent1\033
* MetaKey2:MetaContent2\033
* ...\033
* Meta2ContentSep\033
* MessageContent
*
*
* FrameType has 3 type of value:
* TextMessage 、 PictureMessage 、 FileMessage
*/
type Frame struct {
	FrameType string
	Meta map[string]string
	FrameContent string
}

/*
* Convert a data frame to bytes
*/
func (f *Frame) ToBytes() []byte {
	meta := ""
	if len(f.Meta) == 0 {
		for k, v := range f.Meta {
			meta += fmt.Sprintf("%s:%s\033", k, v)
		}
	} else {
		meta = "\033"
	}
	frame := fmt.Sprintf("%s\033%s%s\033%s", f.FrameType, f.Meta, Meta2ContentSep, f.FrameContent)

	return []byte(frame)
}

func (f *Frame) AddMeta(key string, content string) {
	f.Meta[key] = content
}

/*
* Create a new frame
*/
func NewFrame(ftype string, fcontent string) *Frame {
	return &Frame{
		FrameType	: ftype,
		FrameContent	: fcontent,
	}
}