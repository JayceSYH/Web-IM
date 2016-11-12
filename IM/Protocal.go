package IM

import "fmt"

/*
* Frame define a piece of data sent to client
* When converted to bytes, frame has the following structure
* MessageType\033\033the stuff you gonna send\033\033other stuff...\033\033the last stuff
*
*
* FrameType has 3 type of value:
* TextMessage 、 PictureMessage 、 FileMessage
*/
type Frame struct {
	FrameType string
	Sender string
	FrameContent string
}

/*
* Convert a data frame to bytes
*/
func (f *Frame) ToBytes() []byte {
	frame := fmt.Sprintf("data:%s\033\033%s\033\033%s\n\n", f.FrameType, f.Sender, f.FrameContent)

	return []byte(frame)
}

/*
* Create a new frame
*/
func NewFrame(ftype string, fcontent string, fsender string) *Frame {
	return &Frame{
		FrameType	: ftype,
		FrameContent	: fcontent,
		Sender		: fsender,
	}
}