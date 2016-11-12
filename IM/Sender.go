package IM

import (
	"net/http"
	"io/ioutil"
	"fmt"
)
/*
* Send Text Message With following request format:
* Note: message with post method
* --------------headers:---------------
* "Content-Type":"application/octet-stream"
* "Target-Id":"xxxx"
* "Check-Code":"xxxxx"
* "Message-Type":"xxxx"
* "File-Name" : "xxxxx"  //if it's a file message
* "Pic-Suffix" : "xxx"   //if it's a picture message
* --------------body-------------------
* ::the content you want to send
*
*
*
* MessageType can be the the following types:
* TextMessage 、 PictureMessage 、 FileMessage
*/

func SendMessageHandleFunc(r *http.Request, validateFunc func(string) (string, error)) Message {
	defer r.Body.Close()

	var senderId string
	if checkCode, ok := r.Header["Check-Code"]; ok {
		if send, err := validateFunc(checkCode[0]); err != nil {
			fmt.Println(err)
			return nil
		} else {
			senderId = send
		}
	}

	if targetId, ok := r.Header["Target-Id"]; ok {
		if messageType, ok := r.Header["Message-Type"]; ok {
			body, _ := ioutil.ReadAll(r.Body)

			switch messageType[0] {
			case TextMessageType:
				m := NewTextMessage(string(body))
				fmt.Println("content:",string(body))
				m.SetTargetId(targetId[0])
				m.SetSenderId(senderId)
				return m
			case PictureMessageType:
				if suffix, ok := r.Header["Pic-Suffix"]; ok {
					m := NewPictureMessage(body, suffix[0])
					m.SetTargetId(targetId[0])
					m.SetSenderId(senderId)
					return m
				}
				fmt.Println("Picture Suffix Missed")
				break
			case FileMessageType:
				if fileName, ok := r.Header["File-Name"]; ok {
					m := NewFileMessage(body, fileName[0])
					m.SetTargetId(targetId[0])
					m.SetSenderId(senderId)
					return m
				}
				fmt.Println("Filename Missed")
				break
			}
		} else {
			fmt.Println("Message Type Missed")
		}
	} else {
		fmt.Println("TargetId Missed")
	}

	return nil
}
