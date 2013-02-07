package webrouter

import (
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

func NewRouteManager(filterPrefix, appendSuffix, delimiterStyle string) *RouteManager {
	rm := &RouteManager{
		ServeMux: http.NewServeMux(),
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

func (rm *RouteManager) DelimiterStyle(delimiterStyle string) {
	if delimiterStyle == "" {
		delimiterStyle = "_"
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.delimiterStyle = delimiterStyle
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
func makeHandler(rcvm reflect.Value, rcvmbs []reflect.Value, rcvmas []reflect.Value) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rcvw, rcvr := reflect.ValueOf(w), reflect.ValueOf(req)
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

//Priority: Init > Before_ > Filter_Before > Before_[method] > [method] > After_[method] > Filter_After > After_ > Render > Destroy
func (rm *RouteManager) Register(patternRoot string, i interface{}) {
	rm.mu.RLock()
	filterPrefix := rm.filterPrefix
	appendSuffix := rm.appendSuffix
	delimiterStyle := rm.delimiterStyle
	rm.mu.RUnlock()

	rootMname := filterPrefix + "Root"
	rcvi := reflect.ValueOf(i)
	rcti := rcvi.Type()

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
			var rcvmbs []reflect.Value
			var rcvmas []reflect.Value
			filterPrefixMname := mName[len(filterPrefix):]
			pattern := patternRoot
			if mName != rootMname {
				pattern += strings.ToLower(strings.Replace(filterPrefixMname, "_", delimiterStyle, -1)) + appendSuffix
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
						curMType, cmOk := fm[mName]

						if cmOk && curMType == "allow" {
							rcvmbs = append(rcvmbs, rcvi.MethodByName("Filter_"+fMname))
						} else if !cmOk && aOk && allType == "allow" {
							rcvmbs = append(rcvmbs, rcvi.MethodByName("Filter_"+fMname))
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
						curMType, cmOk := fm[mName]

						if cmOk && curMType == "allow" {
							rcvmas = append(rcvmas, rcvi.MethodByName("Filter_"+fMname))
						} else if !cmOk && aOk && allType == "allow" {
							rcvmas = append(rcvmas, rcvi.MethodByName("Filter_"+fMname))
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

			rm.ServeMux.Handle(pattern, makeHandler(rcvi.Method(i), rcvmbs, rcvmas))
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

func (rm *RouteManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI == "*" {
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h, _ := rm.Handler(r)
	h.ServeHTTP(w, r)
}
