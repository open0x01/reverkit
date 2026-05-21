package reverkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/iami317/hubur"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"time"
)

func init() {
	// 修复部分 Windows 上 js content-type 返回 text/plain 的问题
	// https://github.com/labstack/echo/issues/1038#issuecomment-410294904
	_ = mime.AddExtensionType(".js", "application/javascript")
	_ = mime.AddExtensionType(".css", "text/css")
}

var (
	actionPrev = "Prev"
	actionNext = "Next"
)

func (s *HTTPServer) HandleUnitVisit(w http.ResponseWriter, r *http.Request, ps httprouter.Params, isPublic bool) {
	hashedToken := ps.ByName("token")
	groupID := ps.ByName("group")
	unitID := ""
	if !isPublic {
		unitID = ps.ByName("unit")
	}
	//先验证tocken
	if hashedToken != generateHashedToken(s.config.Token, groupID, unitID) {
		s.LoginRequired(w)
		return
	}

	reqStr, err := httputil.DumpRequest(r, true)
	if err != nil {
		reqStr = []byte("!!failed to dump request!!")
	}

	es := EventSourceInternal
	if isPublic {
		es = EventSourcePublic
	}
	remoteAddr := r.RemoteAddr
	if s.config.HTTPServerConfig.IPHeader != "" {
		ip := r.Header.Get(s.config.HTTPServerConfig.IPHeader)
		if ip != "" {
			remoteAddr = ip
		}
	}
	event := &Event{TimeStamp: hubur.TimeStampMs(),
		Request: string(reqStr), RemoteAddr: remoteAddr,
		UnitId: unitID, GroupID: groupID,
		EventType: EventTypeHTTPVisit, EventSource: es}
	err = s.db.storeEvent(event)
	if err != nil {
		logger.Error(err)
	}
	logger.Infof("reverse http server received request [%s/%s], remoteAddr: %s", groupID, unitID, remoteAddr)
	if isPublic {
		conf := s.db.getHTTPResponse(groupID)
		if conf != nil {
			for k := range globalAddHeader {
				w.Header().Del(k)
			}
			// 避免 http server 自动添加 Content-Type，如果用户不设置就没有这个 header
			w.Header()["Content-Type"] = nil
			for _, item := range conf.Header {
				w.Header().Set(item["key"], item["value"])
			}
			code, _ := strconv.Atoi(conf.StatusCode)
			w.WriteHeader(code)
			_, _ = fmt.Fprint(w, conf.Body)
			return
		}
	} else {
		s.internalGroupEventMap.Store(event.GroupID, event)
	}
	s.Success(w, nil)
}

func (s *HTTPServer) HandleInternalUnitVisit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	s.HandleUnitVisit(w, r, ps, false)
}

func (s *HTTPServer) HandlePublicUnitVisit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	s.HandleUnitVisit(w, r, ps, true)
}

func (s *HTTPServer) checkToken(r *http.Request) bool {
	return r.Header.Get("X-Token") == s.config.Token
}

func (s *HTTPServer) HandleFetchEvent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if !s.checkToken(r) {
		s.LoginRequired(w)
		return
	}

	data := remoteFetchEventRequest{GroupToDelete: make([]string, 0, 10)}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.Fail(w, err)
		return
	}
	err = json.Unmarshal(body, &data)
	if err != nil {
		s.Fail(w, err)
		return
	}
	for _, item := range data.GroupToDelete {
		s.internalGroupEventMap.Delete(item)
	}
	events := make([]*Event, 0, 10)
	s.internalGroupEventMap.Range(func(key, value interface{}) bool {
		events = append(events, value.(*Event))
		return true
	})
	s.Success(w, events)
}

func (s *HTTPServer) HandleHealthCheck(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if !s.checkToken(r) {
		s.LoginRequired(w)
		return
	}
	s.Success(w, nil)
}

func (s *HTTPServer) Start() {
	//logger.Verbosef("starting reverse http server")
	lis, err := net.Listen("tcp", s.Server.Addr)
	if err != nil {
		logger.Errorf("can't start reverse server, %s", err)
		return
	}
	//定时任务，删除在syncmap中的过期事件
	go s.gcExpiredEventMap()
	go func() {
		err = s.Server.Serve(&rmiHTTPListener{Listener: lis, db: s.db, token: s.config.Token, internalGroupEventMap: &s.internalGroupEventMap})
		if err != http.ErrServerClosed {
			logger.Errorf("can't start reverse server, %s", err)
		}
	}()
}

type CommonResponseComponent struct {
	StatusCode string              `json:"statusCode"`
	Header     []map[string]string `json:"header"`
	Body       string              `json:"body"`
}

type HTTPResponseConfig struct {
	CommonResponseComponent
	GroupID string `json:"groupID"`
}

func (s *HTTPServer) Close() error {
	err := s.Server.Close()
	if err != nil {
		logger.Errorf("reverse server shutdown error: [%s]", err)
		return err
	}
	return nil
}

// 定时任务，删除在syncmap中的过期事件
func (s *HTTPServer) gcExpiredEventMap() {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			now := hubur.TimeStampSecond()
			s.internalGroupEventMap.Range(func(key, value interface{}) bool {
				event := value.(*Event)
				if event.TimeStamp < now-600 {
					s.internalGroupEventMap.Delete(key)
				}
				return true
			})
		}
	}
}

func NewHTTPServer(ctx context.Context, config *ServerConfig, db *DB) (*HTTPServer, error) {
	s := &HTTPServer{ctx: ctx, config: config, db: db}

	// todo server max header and body
	s.Router = httprouter.New()
	internalVisitURL := fmt.Sprintf("/%s/:token/:group/:unit/*others", internalAPIMark)
	s.Router.HEAD(internalVisitURL, s.HandleInternalUnitVisit)
	s.Router.GET(internalVisitURL, s.HandleInternalUnitVisit)
	s.Router.POST(internalVisitURL, s.HandleInternalUnitVisit)
	s.Router.PUT(internalVisitURL, s.HandleInternalUnitVisit)
	s.Router.DELETE(internalVisitURL, s.HandleInternalUnitVisit)

	publicVisitURL := fmt.Sprintf("/%s/:token/:group/*others", publicAPIMark)
	s.Router.HEAD(publicVisitURL, s.HandlePublicUnitVisit)
	s.Router.GET(publicVisitURL, s.HandlePublicUnitVisit)
	s.Router.POST(publicVisitURL, s.HandlePublicUnitVisit)
	s.Router.PUT(publicVisitURL, s.HandlePublicUnitVisit)
	s.Router.DELETE(publicVisitURL, s.HandlePublicUnitVisit)

	s.Router.POST("/_/api/fetch", s.HandleFetchEvent)
	s.Router.GET("/_/api/health_check", s.HandleHealthCheck)

	s.Router.GET("/", func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		writer.Write([]byte("It Works"))
	})

	s.Server = &http.Server{
		Addr:              net.JoinHostPort(config.HTTPServerConfig.ListenIP, config.HTTPServerConfig.ListenPort),
		Handler:           s,
		ReadTimeout:       time.Second * 10,
		ReadHeaderTimeout: time.Second * 5,
		WriteTimeout:      time.Second * 5,
		IdleTimeout:       time.Second * 30,
	}
	return s, nil
}

func httpServerCheckAndPrepare(config *ServerConfig) error {
	if !config.HTTPServerConfig.Enabled {
		return nil
	}

	port := config.HTTPServerConfig.ListenPort
	if port == "" {
		newPort, err := hubur.GetFreePort()
		if err != nil {
			return err
		}
		config.HTTPServerConfig.ListenPort = strconv.Itoa(newPort)
	}
	if config.HTTPServerConfig.ListenIP == "" {
		return errors.New("reverse server listen ip can not be empty")
	}

	if config.Token == "" {
		return fmt.Errorf("server token can not be empty, please set a random string, suggestion: \"%s\"", hubur.RandLower(6))
	}
	return nil
}
