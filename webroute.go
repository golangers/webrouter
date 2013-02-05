package webroute

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
)

type RouteManager struct {
	*http.ServeMux
	mu             sync.RWMutex
	notFoundHandle http.Handler
	filterPrefix   string
	appendSuffix   string
	delimiterStyle string
}

type Header struct {
	Key     string
	Headers []string
}

var (
	CtHtmlHeader = Header{"Content-Type", []string{"text/html; charset=utf-8"}}
)

func Error(w http.ResponseWriter, error string, code int, headers ...Header) {
	lenHeader := len(headers)
	if lenHeader == 0 {
		http.Error(w, error, code)
		return
	}

	for _, header := range headers {
		w.Header().Del(header.Key)
		for _, h := range header.Headers {
			w.Header().Add(header.Key, h)
		}
	}

	w.WriteHeader(code)
	fmt.Fprintln(w, error)
}

func NewRouteManager(filterPrefix, appendSuffix, delimiterStyle string) *RouteManager {
	if filterPrefix == "" {
		filterPrefix == "Route"
	} else if filterPrefix == "@" {
		filterPrefix == ""
	}

	if delimiterStyle == "" {
		delimiterStyle = "_"
	}

	return &RouteManager{
		ServeMux:       http.NewServeMux(),
		filterPrefix:   filterPrefix,
		appendSuffix:   appendSuffix,
		delimiterStyle: delimiterStyle,
	}
}

func (rm *RouteManager) HandleObject(patternRoot string, i interface{}) {
	rm.mu.RLock()
	filterPrefix := rm.filterPrefix
	appendSuffix := rm.appendSuffix
	delimiterStyle := rm.delimiterStyle
	rm.mu.RUnlock()
	rcti := reflect.TypeOf(i)
	rcvi := reflect.ValueOf(i)

	for i := 0; i < rcti.NumMethod(); i++ {
		m := rcti.Method(i)
		mName := m.Name
		if pos := strings.Index(mName, filterPrefix); pos == 0 {
			pattern := patternRoot + "/"
			if mName != filterPrefix+"Default" {
				pattern += strings.Replace(mName[len(filterPrefix):], "_", delimiterStyle, -1) + appendSuffix
			}

			rm.Handle(strings.ToLower(pattern), func(rcvm reflect.Value) http.Handler {
				mt := rcvm.Type()
				mtni := mt.NumIn()
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					rvw, rvr := reflect.ValueOf(w), reflect.ValueOf(r)
					switch mtni {
					case 1:
						if mt.In(0) == rvr.Type() {
							rcvm.Call([]reflect.Value{rvr})
						} else {
							rcvm.Call([]reflect.Value{rvw})
						}
					case 2:
						if mt.In(0) == rvr.Type() {
							rcvm.Call([]reflect.Value{rvr, rvw})
						} else {
							rcvm.Call([]reflect.Value{rvw, rvr})
						}
					default:
						rcvm.Call([]reflect.Value{})
					}
				})
			}(rcvi.Method(i)))
		}
	}

}

func (rm *RouteManager) NotFoundHandler(error string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.notFoundHandle = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Error(w, error, http.StatusNotFound)
	})
}

func (rm *RouteManager) NotFoundHtmlHandler(error string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.notFoundHandle = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Error(w, error, http.StatusNotFound, CtHtmlHeader)
	})
}

func (rm *RouteManager) Handle(pattern string, handler http.Handler) {
	rm.ServeMux.Handle(pattern, handler)
}

func (rm *RouteManager) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	rm.Handle(pattern, http.HandlerFunc(handler))
}

func (rm *RouteManager) Handler(r *http.Request) (h http.Handler, pattern string) {
	h, pattern = rm.ServeMux.Handler(r)

	if pattern == "" {
		rm.mu.RLock()
		h = rm.notFoundHandle
		rm.mu.RUnlock()
	}

	return
}

func (rm *RouteManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI == "*" {
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h, _ := rm.Handler(r)
	h.ServeHTTP(w, r)
}

func (rm *RouteManager) ListenAndServe(addr string) error {
	rm.mu.RLock()
	server := &http.Server{
		Addr:    addr,
		Handler: rm,
	}
	rm.mu.RUnlock()

	return server.ListenAndServe()
}

func (rm *RouteManager) ListenAndServeTLS(addr, certFile, keyFile string) error {
	rm.mu.RLock()
	server := &http.Server{
		Addr:    addr,
		Handler: rm,
	}
	rm.mu.RUnlock()

	return server.ListenAndServeTLS(certFile, keyFile)
}
