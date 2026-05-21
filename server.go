package reverkit

import (
	"context"
	"fmt"
	"github.com/iami317/hubur"
	"net"
	"strconv"
)

type Server struct {
	ctx               context.Context
	cancel            context.CancelFunc
	config            *ServerConfig
	db                *DB
	reverseHTTPServer *HTTPServer
}

func NewServer(config *ServerConfig) (*Server, error) {
	//需要一个db，没指定默认生成
	if config.DBFilePath == "" {
		var err error
		config.DBFilePath, err = hubur.GetTempFilePath()
		if err != nil {
			return nil, err
		}
	}
	//创建数据库
	db, err := newDB(config.DBFilePath)
	if err != nil {
		return nil, err
	}
	r := &Server{
		config: config,
		db:     db,
	}
	if err := r.prepareConfig(config); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Server) Start(ctx context.Context) error {
	newCtx, cancel := context.WithCancel(ctx)
	r.ctx = newCtx
	r.cancel = cancel
	var err error
	if r.config.HTTPServerConfig.Enabled {
		//新建一个httpserver（设置路由，和httpserver（设置了地址和路由））
		r.reverseHTTPServer, err = NewHTTPServer(ctx, r.config, r.db)
		if err != nil {
			return err
		}
		r.reverseHTTPServer.Start()
		listenOn := net.JoinHostPort(r.config.HTTPServerConfig.ListenIP, r.config.HTTPServerConfig.ListenPort)
		logger.Infof("reverse http server listened on: %s, token: %s", listenOn, r.config.Token)
	}
	return nil
}

func (r *Server) prepareConfig(c *ServerConfig) error {
	//检查config（ip端口tocken的有效性）
	err := httpServerCheckAndPrepare(c)
	if err != nil {
		return err
	}
	return nil
}

func (r *Server) Config() *ServerConfig {
	return r.config
}

func (r *Server) Close() error {
	defer r.cancel()
	var err error
	var errs []error
	if r.reverseHTTPServer != nil {
		err = r.reverseHTTPServer.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if r.db != nil {
		r.db.Close()
	}
	if len(errs) != 0 {
		return fmt.Errorf("close err: %s", errs)
	}
	return nil
}

// NewLocalTestServer 用于本地测试， 不要忘了 Server.Close
// dbFilePath 可以使用 hubur.GetTempFilePath 生成
func NewLocalTestServer(ctx context.Context, dbFilePath string) (*Server, *Client, error) {
	listenPort, err := hubur.GetFreePort()
	if err != nil {
		return nil, nil, err
	}
	serverConf := &ServerConfig{
		DBFilePath: dbFilePath,
		Token:      hubur.RandLower(8),
		HTTPServerConfig: HTTPServerConfig{
			Enabled:    true,
			ListenIP:   "127.0.0.1",
			ListenPort: strconv.Itoa(listenPort),
		},
	}
	server, err := NewServer(serverConf)
	if err != nil {
		return nil, nil, err
	}
	err = server.Start(ctx)
	if err != nil {
		return nil, nil, err
	}
	clientConf := &ClientConfig{
		Token:       serverConf.Token,
		HTTPBaseURL: fmt.Sprintf("http://127.0.0.1:%d", listenPort),
	}
	client, err := NewClient(ctx, clientConf)
	if err != nil {
		return nil, nil, err
	}
	return server, client, nil
}
