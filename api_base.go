package reverkit

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"sync"
)

const (
	CodeSuccess       = 0
	CodeGenericFailed = 1
	CodeLoginRequired = 2
)

type ResponseBase struct {
	Code int `json:"code"`
}

type Response struct {
	ResponseBase
	Data interface{} `json:"data"`
}

func (r Response) json() ([]byte, error) {
	return json.Marshal(r)
}

type HTTPServer struct {
	Server *http.Server
	Router *httprouter.Router

	ctx                   context.Context
	config                *ServerConfig
	db                    *DB
	internalGroupEventMap sync.Map
}

var globalAddHeader = map[string]string{
	"X-XSS-Protection":       "1; mode=block",
	"X-Frame-Options":        "SAMEORIGIN",
	"X-Content-Type-Options": "nosniff",
	"Content-Security-Policy": "default-src 'self'; script-src 'self' 'unsafe-inline'; " +
		"object-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self'; " +
		"media-src 'self'; frame-src 'self'; font-src 'self' data:; connect-src 'self'",
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for k, v := range globalAddHeader {
		w.Header().Add(k, v)
	}
	s.Router.ServeHTTP(w, r)
}

func (s *HTTPServer) response(w http.ResponseWriter, data interface{}, code int) {
	jsonData, err := Response{ResponseBase: ResponseBase{Code: code}, Data: data}.json()
	if err != nil {
		w.WriteHeader(500)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	_, _ = fmt.Fprint(w, string(jsonData))
}

func (s *HTTPServer) Success(w http.ResponseWriter, data interface{}) {
	s.response(w, data, CodeSuccess)
}

func (s *HTTPServer) Fail(w http.ResponseWriter, data interface{}) {
	s.response(w, data, CodeGenericFailed)
}

func (s *HTTPServer) LoginRequired(w http.ResponseWriter) {
	s.response(w, "Login Required", CodeLoginRequired)
}
