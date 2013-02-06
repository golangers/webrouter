package webroute

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
	DefauleRouter.Handle(pattern, handler)
}

func HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	DefauleRouter.HandleFunc(pattern, handler)
}

func Handler(req *http.Request) (h http.Handler, pattern string) {
	return DefauleRouter.Handler(req)
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

func ListenAndServe(addr string) error {
	return DefauleRouter.ListenAndServe(addr)
}

func ListenAndServeTLS(addr, certFile, keyFile string) error {
	return DefauleRouter.ListenAndServeTLS(addr, certFile, keyFile)
}
