package main

import (
	"context"
	"errors"
	"github.com/iami317/logx"
	"github.com/open0x01/reverkit"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"net"
	"os"
)

type reverseConfig struct {
	Token      string `yaml:"token" json:"token"`
	DBPath     string `yaml:"db_path" json:"db_path"`
	HTTPListen string `yaml:"http_listen" json:"http_listen"`
}

func main() {
	app := &cli.App{}
	app.Usage = "A reverse service for ai.*"
	app.Name = "reverkit"
	logx.SetLevel("debug")

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "http-listen",
			Usage: "start a http reverse service, eg. 127.0.0.1:1111",
		},
		cli.StringFlag{
			Name:  "token",
			Usage: "reverse service token",
		},
		cli.StringFlag{
			Name:  "dbpath",
			Usage: "set the db path",
		},
		cli.StringFlag{
			Name:  "config",
			Usage: "load config from `FILE`",
		},
	}
	app.Action = func(c *cli.Context) error {
		var config reverseConfig
		configPath := c.String("config")
		httpListen := c.String("http-listen")
		token := c.String("token")
		dbPath := c.String("dbpath")
		if configPath != "" {
			data, err := ioutil.ReadFile(configPath)
			if err != nil {
				return err
			}
			err = yaml.Unmarshal(data, &config)
			if err != nil {
				return err
			}
		}
		if httpListen != "" {
			config.HTTPListen = httpListen
		}
		if token != "" {
			config.Token = token
		}
		if dbPath != "" {
			config.DBPath = dbPath
		}

		if config.HTTPListen == "" {
			return errors.New("http listen can not be empty")
		}
		if config.Token == "" {
			return errors.New("token can not be empty")
		}
		if config.DBPath == "" {
			return errors.New("db path can not be empty")
		}

		//设置反连平台的config
		conf := reverkit.NewDefaultConfig()

		// 拆分指定的平台的ip和端口
		host, port, err := net.SplitHostPort(config.HTTPListen)
		if err != nil {
			return err
		}
		//设置http反连平台
		conf.HTTPServerConfig.Enabled = true
		conf.HTTPServerConfig.ListenIP = host
		conf.HTTPServerConfig.ListenPort = port
		conf.Token = config.Token
		conf.DBFilePath = config.DBPath

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		//声明server对象
		s, err := reverkit.NewServer(conf)
		if err != nil {
			return err
		}

		//启动server
		err = s.Start(ctx)
		if err != nil {
			return err
		}
		<-ctx.Done()
		defer s.Close()
		return nil
	}
	if err := app.Run(os.Args); err != nil {
		logx.Error(err.Error())
		os.Exit(1)
	}
}
