package webroute

import (
	"net/http"
)

type Router struct {
	*http.Request
	http.ResponseWriter
}

func NewRouter() *Router {
	return &Router{}
}

func (r *Router) SetDefaultRoute() {

}

func Register() {

}

func ListenAndServe(addr string, i interface{}) {

}
