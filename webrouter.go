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
