package webrouter

import (
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
)

type RouteManager struct {
	*http.ServeMux
	mu             sync.RWMutex
	injections     []injector
	releases       []releasor
	notFoundHandle http.Handler
	filterPrefix   string
	appendSuffix   string
	delimiterStyle byte
}

type filterMethod struct {
	method string
	param  []string
	rcvm   reflect.Value
}

func NewRouteManager(filterPrefix, appendSuffix string, delimiterStyle byte) *RouteManager {
	rm := &RouteManager{
		ServeMux:   http.NewServeMux(),
		injections: []injector{},
		releases:   []releasor{},
	}

	rm.FilterPrefix(filterPrefix)
	rm.AppendSuffix(appendSuffix)
	rm.DelimiterStyle(delimiterStyle)

	return rm
}

//if filterPrefix value is '@' that mean not to filter, but it is has hidden danger, so you kown what to do.
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

func (rm *RouteManager) DelimiterStyle(delimiterStyle byte) {
	if delimiterStyle == 0 {
		delimiterStyle = '-'
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.delimiterStyle = delimiterStyle
}

func (rm *RouteManager) Injector(name, follower string, priority uint, handler func(w http.ResponseWriter, r *http.Request)) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if hasSameInjector(rm.injections, name) {
		panic("multiple registrations injector for " + name)
		os.Exit(-1)
	}

	rm.injections = append(rm.injections, injector{
		name:     name,
		follower: follower,
		priority: int(priority),
		h:        http.HandlerFunc(handler),
	})
}

func (rm *RouteManager) SortInjector() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.injections = sortInjector(rm.injections)
}

func (rm *RouteManager) Releasor(name, leader string, lag uint, handler func(w http.ResponseWriter, r *http.Request)) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if hasSameReleasor(rm.releases, name) {
		panic("multiple registrations releasor for " + name)
		os.Exit(-1)
	}

	rm.releases = append(rm.releases, releasor{
		name:   name,
		leader: leader,
		lag:    int(lag),
		h:      http.HandlerFunc(handler),
	})
}

func (rm *RouteManager) SortReleasor() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.releases = sortReleasor(rm.releases)
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
1.  router.Init
2.  router.Before_
3.  router.Before_[method]
4.  router.Filter_Before
5.  router.Http_<http's Method>_[method]
6.  router.[method]
7.  router.Filter_After
8.  router.After_[method
9.  router.After_
10. router.Render
11. router.Destroy

router.Xxx Can return one result of bool, if result is ture mean return func
*/
func makeHandler(rcvm reflect.Value, rcvmbs []reflect.Value, rcvmfb []filterMethod, rcvmfa []filterMethod, rcvmas []reflect.Value) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var parsedForm bool
		rcvw, rcvr := reflect.ValueOf(w), reflect.ValueOf(req)

		for _, rcvmb := range rcvmbs {
			if arv := callMethod(rcvmb, rcvw, rcvr); len(arv) > 0 && arv[0].Bool() {
				return
			}
		}

		for _, fm := range rcvmfb {
			if fm.method != "" && fm.method != req.Method {
				continue
			}

			call := true
			if len(fm.param) > 0 {
				if !parsedForm {
					req.ParseForm()
					parsedForm = true
				}

				for _, param := range fm.param {
					if _, ok := req.Form[param]; !ok {
						call = false
						break
					}
				}
			}

			if call {
				if arv := callMethod(fm.rcvm, rcvw, rcvr); len(arv) > 0 && arv[0].Bool() {
					return
				}
			}
		}

		if arv := callMethod(rcvm, rcvw, rcvr); len(arv) > 0 && arv[0].Bool() {
			return
		}

		for _, fm := range rcvmfa {
			if fm.method != "" && fm.method != req.Method {
				continue
			}

			call := true
			if len(fm.param) > 0 {
				if !parsedForm {
					req.ParseForm()
					parsedForm = true
				}

				for _, p := range fm.param {
					if _, ok := req.Form[p]; !ok {
						call = false
						break
					}
				}
			}

			if call {
				if arv := callMethod(fm.rcvm, rcvw, rcvr); len(arv) > 0 && arv[0].Bool() {
					return
				}
			}
		}

		for _, rcvma := range rcvmas {
			if arv := callMethod(rcvma, rcvw, rcvr); len(arv) > 0 && arv[0].Bool() {
				return
			}
		}

	})
}

func findHttpMethod(rcti reflect.Type, rcvi reflect.Value) map[string][]filterMethod {
	rcvhm := make(map[string][]filterMethod)

	for i := 0; i < rcti.NumMethod(); i++ {
		mName := rcti.Method(i).Name
		if hpos := strings.Index(mName, "Http_"); hpos != -1 {
			hpMname := mName[hpos+len("Http_"):]
			if mpos := strings.Index(hpMname, "_"); mpos != -1 {
				httpMethod := hpMname[:mpos]
				//len("_") == 1
				objMethod := hpMname[mpos+1:]

				rcvhm[objMethod] = append(rcvhm[objMethod], filterMethod{
					method: httpMethod,
					rcvm:   rcvi.Method(i),
				})
			}
		}
	}

	return rcvhm
}

/*
NewPattern => new[delimiterStyle]pattern
delimiterStyle => -
NewPattern => new-pattern
*/
func makePattern(method string, delimiterStyle byte) string {
	var c byte
	bl := byte('a' - 'A')
	l := len(method)
	pattern := make([]byte, 0, l+8)
	for i := 0; i < l; i++ {
		c = method[i]
		if c >= 'A' && c <= 'Z' {
			c += bl
			if i > 0 {
				pattern = append(pattern, delimiterStyle)
			}
		}

		pattern = append(pattern, c)
	}

	return string(pattern)
}

//Priority: Init > Before_ > Before_[method] > Filter_Before > Http_<http's Method>_[method] > [method] > Filter_After > After_[method] > After_ > Render > Destroy
func (rm *RouteManager) Register(patternRoot string, i interface{}) {
	rm.mu.RLock()
	filterPrefix := rm.filterPrefix
	appendSuffix := rm.appendSuffix
	delimiterStyle := rm.delimiterStyle
	rm.mu.RUnlock()

	defaultMname := filterPrefix + "Default"
	rcvi := reflect.ValueOf(i)
	rcti := rcvi.Type()

	var rcvhm map[string][]filterMethod
	rcvhm = findHttpMethod(rcti, rcvi)

	var rcvmi reflect.Value
	var hasInit bool
	if _, hasInit = rcti.MethodByName("Init"); hasInit {
		rcvmi = rcvi.MethodByName("Init")
	}

	var rcvmr reflect.Value
	var hasRender bool
	if _, hasRender = rcti.MethodByName("Render"); hasRender {
		rcvmr = rcvi.MethodByName("Render")
	}

	var rcvmd reflect.Value
	var hasDestroy bool
	if _, hasDestroy = rcti.MethodByName("Destroy"); hasDestroy {
		rcvmd = rcvi.MethodByName("Destroy")
	}

	var fbm []map[string]string
	if _, hasFilterBefore := rcti.MethodByName("Filter_Before"); hasFilterBefore {
		rcvmfb := rcvi.MethodByName("Filter_Before")
		fbres := rcvmfb.Call([]reflect.Value{})
		fbm = fbres[0].Interface().([]map[string]string)
	}

	var fam []map[string]string
	if _, hasFilterAfter := rcti.MethodByName("Filter_After"); hasFilterAfter {
		rcvmfa := rcvi.MethodByName("Filter_After")
		fares := rcvmfa.Call([]reflect.Value{})
		fam = fares[0].Interface().([]map[string]string)
	}

	for i := 0; i < rcti.NumMethod(); i++ {
		mName := rcti.Method(i).Name
		if pos := strings.Index(mName, filterPrefix); pos == 0 {
			var (
				rcvmbs, rcvmas []reflect.Value
				rcvmfb, rcvmfa []filterMethod
			)

			filterPrefixMname := mName[len(filterPrefix):]
			pattern := patternRoot
			if mName != defaultMname {
				pattern += makePattern(filterPrefixMname, delimiterStyle) + appendSuffix
			}

			if hasInit {
				rcvmbs = append(rcvmbs, rcvmi)
			}

			if _, ok := rcti.MethodByName("Before_"); ok {
				rcvmbs = append(rcvmbs, rcvi.MethodByName("Before_"))
			}

			if len(fbm) > 0 {
				for _, fm := range fbm {
					if fMname, ok := fm["_FILTER"]; ok {
						allType, aOk := fm["_ALL"]
						curMType, cmOk := fm[filterPrefixMname]

						switch {
						case cmOk && curMType == "allow":
							fallthrough
						case !cmOk && aOk && allType == "allow":
							var httpParams []string
							if fm["_PARAM"] != "" {
								httpParams = strings.Split(fm["_PARAM"], "&")
							}

							if fm["_METHOD"] == "" {
								rcvmfb = append(rcvmfb, filterMethod{
									param: httpParams,
									rcvm:  rcvi.MethodByName("Filter_" + fMname),
								})
							} else {
								httpMethods := strings.Split(fm["_METHOD"], "|")
								for _, httpMethod := range httpMethods {
									rcvmfb = append(rcvmfb, filterMethod{
										method: httpMethod,
										param:  httpParams,
										rcvm:   rcvi.MethodByName("Filter_" + fMname),
									})
								}
							}
						}
					}
				}
			}

			beforeMname := "Before_" + filterPrefixMname
			if _, ok := rcti.MethodByName(beforeMname); ok {
				rcvmbs = append(rcvmbs, rcvi.MethodByName(beforeMname))
			}

			afterMname := "After_" + filterPrefixMname
			if _, ok := rcti.MethodByName(afterMname); ok {
				rcvmas = append(rcvmas, rcvi.MethodByName(afterMname))
			}

			if len(fam) > 0 {
				for _, fm := range fam {
					if fMname, ok := fm["_FILTER"]; ok {
						allType, aOk := fm["_ALL"]
						curMType, cmOk := fm[filterPrefixMname]

						switch {
						case cmOk && curMType == "allow":
							fallthrough
						case !cmOk && aOk && allType == "allow":
							var httpParams []string
							if fm["_PARAM"] != "" {
								httpParams = strings.Split(fm["_PARAM"], "&")
							}

							if fm["_METHOD"] == "" {
								rcvmfa = append(rcvmfa, filterMethod{
									param: httpParams,
									rcvm:  rcvi.MethodByName("Filter_" + fMname),
								})
							} else {
								httpMethods := strings.Split(fm["_METHOD"], "|")
								for _, httpMethod := range httpMethods {
									rcvmfa = append(rcvmfa, filterMethod{
										method: httpMethod,
										param:  httpParams,
										rcvm:   rcvi.MethodByName("Filter_" + fMname),
									})
								}
							}
						}
					}
				}
			}

			if _, ok := rcti.MethodByName("After_"); ok {
				rcvmas = append(rcvmas, rcvi.MethodByName("After_"))
			}

			if hasRender {
				rcvmas = append(rcvmas, rcvmr)
			}

			if hasDestroy {
				rcvmas = append(rcvmas, rcvmd)
			}

			rcvmfb = append(rcvmfb, rcvhm[filterPrefixMname]...)
			rm.ServeMux.Handle(pattern, makeHandler(rcvi.Method(i), rcvmbs, rcvmfb, rcvmfa, rcvmas))
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

func (rm *RouteManager) Handler(r *http.Request) (h http.Handler, pattern string) {
	h, pattern = rm.ServeMux.Handler(r)
	if pattern == "" && rm.notFoundHandle != nil {
		h = rm.notFoundHandle
	}

	return
}

//processing order: injector > handler > releasor
func (rm *RouteManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI == "*" {
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	rm.mu.RLock()
	defer rm.mu.RUnlock()

	for _, injection := range rm.injections {
		injection.h.ServeHTTP(w, r)
	}

	h, _ := rm.Handler(r)
	h.ServeHTTP(w, r)

	for _, release := range rm.releases {
		release.h.ServeHTTP(w, r)
	}
}
