package webroute

import (
	"net/http"
	"path"
	"reflect"
	"strings"
	"sync"
)

type RouteManager struct {
	hosts          bool
	mu             sync.RWMutex
	router         map[string]*route
	notFoundHandle http.Handler
	filterPrefix   string
	appendSuffix   string
	delimiterStyle string
}

type route struct {
	pattern string
	h       http.Handler
}

func NewRouteManager(filterPrefix, appendSuffix, delimiterStyle string) *RouteManager {
	rm := &RouteManager{
		router: map[string]*route{},
	}

	rm.FilterPrefix(filterPrefix)
	rm.AppendSuffix(appendSuffix)
	rm.DelimiterStyle(delimiterStyle)

	return rm
}

func (rm *RouteManager) FilterPrefix(filterPrefix string) {
	if filterPrefix == "" {
		filterPrefix = "Route"
	} else if filterPrefix == "@" {
		filterPrefix = ""
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.filterPrefix = filterPrefix
}

func (rm *RouteManager) AppendSuffix(appendSuffix string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.appendSuffix = appendSuffix
}

func (rm *RouteManager) DelimiterStyle(delimiterStyle string) {
	if delimiterStyle == "" {
		delimiterStyle = "_"
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.delimiterStyle = delimiterStyle
}

func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)
	if p[len(p)-1] == '/' && np != "/" {
		np += "/"
	}
	return np
}

func callMethod(rcvm, rcvw, rcvr reflect.Value) (arv []reflect.Value) {
	mt := rcvm.Type()
	mtni := mt.NumIn()
	switch mtni {
	case 1:
		if mt.In(0) == rcvr.Type() {
			arv = rcvm.Call([]reflect.Value{rcvr})
		} else {
			arv = rcvm.Call([]reflect.Value{rcvw})
		}
	case 2:
		if mt.In(0) == rcvr.Type() {
			arv = rcvm.Call([]reflect.Value{rcvr, rcvw})
		} else {
			arv = rcvm.Call([]reflect.Value{rcvw, rcvr})
		}
	default:
		arv = rcvm.Call([]reflect.Value{})
	}

	return
}

/*
workflow:
1. rcvmi => reflect.ValueOf(router.Init)
2. rcvmbs => reflect.ValueOf(router.Before_) & reflect.ValueOf(router.Before_[method])
3. rcvm => reflect.ValueOf(router.[method])
4. rcvmas => reflect.ValueOf(router.After_[method] & reflect.ValueOf(router.After_)

rcvmi, rcvmb..., rcvma... Can return one result of bool, if result is ture mean return func
*/
func makeHandler(rcvm reflect.Value, rcvmi reflect.Value, rcvmbs []reflect.Value, rcvmas []reflect.Value) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rcvw, rcvr := reflect.ValueOf(w), reflect.ValueOf(req)
		if rcvmi.IsValid() {
			if arv := callMethod(rcvmi, rcvw, rcvr); len(arv) > 0 && arv[0].Bool() {
				return
			}
		}

		if len(rcvmbs) > 0 {
			for _, revmb := range rcvmbs {
				if arv := callMethod(revmb, rcvw, rcvr); len(arv) > 0 && arv[0].Bool() {
					return
				}
			}
		}

		if arv := callMethod(rcvm, rcvw, rcvr); len(arv) > 0 && arv[0].Bool() {
			return
		}

		if len(rcvmas) > 0 {
			for _, rcvma := range rcvmas {
				if arv := callMethod(rcvma, rcvw, rcvr); len(arv) > 0 && arv[0].Bool() {
					return
				}
			}
		}
	})
}

func (rm *RouteManager) match(p string) (h http.Handler, pattern string) {
	if r := rm.router[p]; r != nil {
		h, pattern = r.h, p
	}

	return
}

func (rm *RouteManager) handler(host, path string) (h http.Handler, pattern string) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if rm.hosts {
		h, pattern = rm.match(host + path)
	}

	if h == nil {
		h, pattern = rm.match(path)
	}

	if h == nil {
		pattern = ""
		if rm.notFoundHandle == nil {
			h = http.NotFoundHandler()
		} else {
			h = rm.notFoundHandle
		}
	}

	return
}

func (rm *RouteManager) Register(patternRoot string, i interface{}) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if patternRoot[0] != '/' {
		rm.hosts = true
	}

	filterPrefix := rm.filterPrefix
	appendSuffix := rm.appendSuffix
	delimiterStyle := rm.delimiterStyle
	rcvi := reflect.ValueOf(i)
	rcti := rcvi.Type()
	var rcvmi reflect.Value
	if _, hasInit := rcti.MethodByName("Init"); hasInit {
		rcvmi = rcvi.MethodByName("Init")
	}

	for i := 0; i < rcti.NumMethod(); i++ {
		mName := rcti.Method(i).Name
		if pos := strings.Index(mName, filterPrefix); pos == 0 {
			filterPrefixMname := mName[len(filterPrefix):]
			pattern := patternRoot
			if mName != filterPrefix+"Default" {
				pattern += strings.ToLower(strings.Replace(filterPrefixMname, "_", delimiterStyle, -1)) + appendSuffix
			}

			var rcvmbs []reflect.Value
			var rcvmas []reflect.Value
			if _, ok := rcti.MethodByName("Before_"); ok {
				rcvmbs = append(rcvmbs, rcvi.MethodByName("Before_"))
			}

			beforeMname := "Before_" + filterPrefixMname
			if _, ok := rcti.MethodByName(beforeMname); ok {
				rcvmbs = append(rcvmbs, rcvi.MethodByName(beforeMname))
			}

			afterMname := "After_" + filterPrefixMname
			if _, ok := rcti.MethodByName(afterMname); ok {
				rcvmas = append(rcvmas, rcvi.MethodByName(afterMname))
			}

			if _, ok := rcti.MethodByName("After_"); ok {
				rcvmas = append(rcvmas, rcvi.MethodByName("After_"))
			}

			rm.router[pattern] = &route{
				pattern: pattern,
				h:       makeHandler(rcvi.Method(i), rcvmi, rcvmbs, rcvmas),
			}
		}
	}
}

func (rm *RouteManager) Handle(pattern string, handler http.Handler) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if pattern[0] != '/' {
		rm.hosts = true
	}

	rm.router[pattern] = &route{
		pattern: pattern,
		h:       handler,
	}
}

func (rm *RouteManager) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	rm.Handle(pattern, http.HandlerFunc(handler))
}

func (rm *RouteManager) Handler(req *http.Request) (h http.Handler, pattern string) {
	if req.Method != "CONNECT" {
		if p := cleanPath(req.URL.Path); p != req.URL.Path {
			_, pattern = rm.handler(req.Host, p)
			return http.RedirectHandler(p, http.StatusMovedPermanently), pattern
		}
	}

	return rm.handler(req.Host, req.URL.Path)
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

func (rm *RouteManager) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.RequestURI == "*" {
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h, _ := rm.Handler(req)
	h.ServeHTTP(w, req)
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
