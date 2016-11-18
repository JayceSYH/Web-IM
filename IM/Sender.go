package IM

import (
	"net/http"
	"io/ioutil"
	"github.com/labstack/gommon/log"
)
/*
* Send Text Message With following request format:
* Note: message with post method
* --------------headers:---------------
* "Content-Type":"application/octet-stream"
* "Target-Id":"xxxx"				//if it's a group message(Group-Id is set), format:"target1;target2"
* "Group-Id":"xxxx"				//set when you message is going to be delivered to several other users
* "Check-Code":"xxxxx"
* "Message-Type":"xxxx"
* "File-Name" : "xxxxx"  			//if it's a file message
* "Pic-Suffix" : "xxx"   			//if it's a picture message
* --------------body-------------------
* ::the content you want to send
*
*
*
* MessageType can be the the following types:
* TextMessage 、 PictureMessage 、 FileMessage
*/

func SendMessageHandleFunc(r *http.Request, validateFunc func(string) (*User, error)) Message {
	defer r.Body.Close()

	var senderId string
	if checkCode, ok := r.Header["Check-Code"]; ok {
		if u, err := validateFunc(checkCode[0]); err != nil {
			log.Print(err)
			return nil
		} else {
			senderId = u.id
		}
	}

	if targetId, ok := r.Header["Target-Id"]; ok {
		if messageType, ok := r.Header["Message-Type"]; ok {
			var group []string
			if group, ok = r.Header["Group-Id"]; !ok {
				group = make([]string, 0)
			}

			body, _ := ioutil.ReadAll(r.Body)

			switch messageType[0] {
			case TextMessageType:
				m := NewTextMessage(string(body))
				log.Print("content:",string(body))
				m.SetTargetId(targetId[0])
				m.SetSenderId(senderId)
				if len(group) > 0 {
					m.SetGroup(group[0])
				}
				return m
			case PictureMessageType:
				if suffix, ok := r.Header["Pic-Suffix"]; ok {
					m := NewPictureMessage(body, suffix[0])
					m.SetTargetId(targetId[0])
					m.SetSenderId(senderId)
					if len(group) > 0 {
						m.SetGroup(group[0])
					}
					return m
				}
				log.Print("Picture Suffix Missed")
				break
			case FileMessageType:
				if fileName, ok := r.Header["File-Name"]; ok {
					m := NewFileMessage(body, fileName[0])
					m.SetTargetId(targetId[0])
					m.SetSenderId(senderId)
					if len(group) > 0 {
						m.SetGroup(group[0])
					}
					return m
				}
				log.Print("Filename Missed")
				break
			}
		} else {
			log.Print("Message Type Missed")
		}
	} else {
		log.Print("TargetId Missed")
	}

	return nil
}
