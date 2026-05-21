package main

import (
	"context"
	"fmt"
	"github.com/iami317/logx"
	"github.com/iami317/reverkit"
	"github.com/urfave/cli"
	"os"
	"time"
)

func main() {
	app := &cli.App{}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "server-addr",
			Usage: "start a http reverse service, eg. 127.0.0.1:1111",
		},
		cli.StringFlag{
			Name:  "token",
			Usage: "reverse service token",
		},
	}
	app.Action = func(c *cli.Context) error {
		serverAddr := c.String("server-addr")
		token := c.String("token")
		if serverAddr == "" || token == "" {
			return fmt.Errorf("empty args, try --help")
		}

		clientConfig := &reverkit.ClientConfig{
			Token:       token,
			HTTPBaseURL: serverAddr,
		}
		return DummyTry(clientConfig)
	}
	if err := app.Run(os.Args); err != nil {
		logx.Fatal(err)
	}
}

func DummyTry(config *reverkit.ClientConfig) error {

	//新建一个client，并不断请求服务器查看连接记录，查看是否有自己的组记录
	client, err := reverkit.NewClient(context.Background(), config)
	if err != nil {
		return err
	}
	//生成一个组，存入client，生成一个事件存入组中，并返回
	unit := client.NewUnit()

	//设置callback属性
	unit.OnVisit(func(event *reverkit.Event) error {
		fmt.Println()
		fmt.Println("new event from reverse server")
		fmt.Printf("groupid: %s, unitid: %s\n", event.GroupID, event.UnitId)
		fmt.Printf("remote addr: %s\n", event.RemoteAddr)
		fmt.Printf("request: %s\n", event.Request)
		return nil
	})
	reverseURL := unit.GetVisitURL()
	re := unit.GetRmiURL()
	re1 := unit.GetLdapURL()
	logx.Infof("try request %s %s %s", reverseURL, re, re1)
	time.Sleep(time.Second)
	//clien:=&http.Client{}
	//tr := &http.Transport{
	//	TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	//}
	//u, err := url.Parse("http://172.16.95.1:8080")
	//if err != nil {
	//}
	//tr.Proxy = http.ProxyURL(u)
	//clien.Transport=tr
	//_, err = clien.Get(reverseURL)
	//if err != nil {
	//	return err
	//}
	return unit.Wait(time.Second * 50)
}
