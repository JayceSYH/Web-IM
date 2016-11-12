package IM

import (
	"net/http"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
	"fmt"
)

type Router interface {
	Route(string, http.Handler)
	RouteFunc(string, func(http.ResponseWriter, *http.Request))
	Start(string, string)
}

type BeeGoRouter struct {}

func (r *BeeGoRouter) Route(url string, h http.Handler) {
	beego.Any(url, func(c *context.Context) {
		h.ServeHTTP(c.ResponseWriter.ResponseWriter, c.Request)
	})
}

func (r *BeeGoRouter) RouteFunc(url string, f func(http.ResponseWriter, *http.Request)) {
	beego.Any(url, func(c *context.Context) {
		f(c.ResponseWriter.ResponseWriter, c.Request)
	})
}

func (r *BeeGoRouter) Start(host string, port string) {
	beego.Run(host, port)
}

/*
* Now we use beego's router as our default router
*/
func NewRouter() Router {
	return &BeeGoRouter{}
}

func route(im *IM) {
	//route communication
	router := NewRouter()
	//router.RouteFunc(im.communicationPath, im.communication.Start)
	beego.Any(im.communicationPath + "/:checkCode", func(c *context.Context) {
		checkCode := c.Input.Param(":checkCode")
		im.communication.Start(c.ResponseWriter.ResponseWriter, c.Request, checkCode)
	})

	//route get file
	beego.Any("/" + DefaultFILERootPath + "/:hash/:fn", func(c *context.Context) {
		hash := c.Input.Param(":hash")

		if file, err := im.FetchFile(hash); err != nil {
			fmt.Println(err)
		} else {
			c.ResponseWriter.Write(file)
		}
	})

	//route message sender
	router.RouteFunc(im.senderPath, func(w http.ResponseWriter, r *http.Request) {
		m := SendMessageHandleFunc(r, im.Validate)
		if m != nil {
			im.SendMessage(m)
		}
	})

	//route user manage function
	router.RouteFunc(im.registerURL, im.UserManager.ServeRegister)
	router.RouteFunc(im.updateSecretURL, im.UserManager.ServeUpdateKey)

	beego.Run(im.host[7:])
}