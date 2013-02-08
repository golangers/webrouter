package webrouter

import (
	"net/http"
)

var (
	DefauleRouter = NewRouteManager("", "", "")
)

func FilterPrefix(filterPrefix string) {
	DefauleRouter.FilterPrefix(filterPrefix)
}

func AppendSuffix(appendSuffix string) {
	DefauleRouter.AppendSuffix(appendSuffix)
}

func DelimiterStyle(delimiterStyle string) {
	DefauleRouter.DelimiterStyle(delimiterStyle)
}

func Register(patternRoot string, i interface{}) {
	DefauleRouter.Register(patternRoot, i)
}

func Handle(pattern string, handler http.Handler) {
	DefauleRouter.ServeMux.Handle(pattern, handler)
}

func HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	DefauleRouter.ServeMux.HandleFunc(pattern, handler)
}

func Handler(req *http.Request) (h http.Handler, pattern string) {
	return DefauleRouter.ServeMux.Handler(req)
}

func NotFoundHandler(error string) {
	DefauleRouter.NotFoundHandler(error)
}

func NotFoundHtmlHandler(error string) {
	DefauleRouter.NotFoundHtmlHandler(error)
}

func ServeHTTP(w http.ResponseWriter, req *http.Request) {
	DefauleRouter.ServeHTTP(w, req)
}

func Injector(name, follower string, priority uint, handler func(w http.ResponseWriter, r *http.Request)) {
	DefauleRouter.Injector(name, follower, priority, handler)
}

func SortInjector() {
	DefauleRouter.SortInjector()
}

func Releasor(name, leader string, lag uint, handler func(w http.ResponseWriter, r *http.Request)) {
	DefauleRouter.Releasor(name, leader, lag, handler)
}

func SortReleasor() {
	DefauleRouter.SortReleasor()
}

func ListenAndServe(addr string, handler http.Handler) error {
	if handler == nil {
		return http.ListenAndServe(addr, DefauleRouter)
	}

	return http.ListenAndServe(addr, handler)
}

func ListenAndServeTLS(addr, certFile, keyFile string, handler http.Handler) error {
	if handler == nil {
		return http.ListenAndServeTLS(addr, certFile, keyFile, DefauleRouter)
	}

	return http.ListenAndServeTLS(addr, certFile, keyFile, handler)
}
