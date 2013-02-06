package webrouter

import (
	"fmt"
	"net/http"
)

var (
	CtHtmlHeader = Header{"Content-Type", []string{"text/html; charset=utf-8"}}
)

type Header struct {
	Key     string
	Headers []string
}

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
