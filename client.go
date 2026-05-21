package reverkit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/iami317/hubur"
	"github.com/iami317/shttp"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type Client struct {
	ctx    context.Context
	config *ClientConfig
	// group_id: *UnitGroup
	groupUnitCallbackMap sync.Map
	groupToDelete        remoteFetchEventRequest
}

// 创建一个客户端
func NewClient(ctx context.Context, config *ClientConfig) (*Client, error) {
	newConf := *config
	config = &newConf
	//复制config
	client := &Client{
		ctx:    ctx,
		config: config,
	}
	//设置rmi地址
	err := client.fillRMIAddr()
	if err != nil {
		return nil, err
	}
	//带tocken访问服务器查看是否存活
	err = client.healthCheck()
	if err != nil {
		return nil, err
	}
	logger.Verbosef("remote reverse server check passed")
	//定时任务不断去服务器请求，查看服务器返回的信息中有没有自己缓存的事件
	go client.fetchEvent()
	//定时任务，每十秒查看一次map中的事件组是否超时，超时就删除
	go client.gcExpiredGroup()
	return client, nil
}

func (r *Client) Config() *ClientConfig {
	return r.config
}

// 定时任务每隔一秒请求去请求一下服务器，检查有没有事件请求
func (r *Client) fetchEvent() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			//logger.Debug("fetch and callback cancelled")
			return
		case <-ticker.C:
			err := r.remoteFetchEvent()
			if err != nil {
				logger.Error(err)
			}
		}
	}
}

// 生成新的 unit 加入group
func (r *Client) NewUnitWithGroup(group *UnitGroup) *Unit {
	unit := &Unit{client: r, id: hubur.RandLower(4), group: group}
	group.Join(unit)
	return unit
}

// 生成一个组，存入client，生成一个事件存入组中，并返回
func (r *Client) NewUnit() *Unit {
	group := r.NewUnitGroup()
	return r.NewUnitWithGroup(group)
}

// 新建一个事件组，超时时间为60秒，id为四位随机数，并放到client中
func (r *Client) NewUnitGroup() *UnitGroup {
	g := &UnitGroup{units: sync.Map{}, id: hubur.RandLower(4), expireAt: time.Now().Add(defaultUnitExpireTime)}
	r.groupUnitCallbackMap.Store(g.id, g)
	return g
}

// Resolve groupId to a UnitGroup if exists, or nil
func (r *Client) Resolve(groupId string) *UnitGroup {
	value, ok := r.groupUnitCallbackMap.Load(groupId)
	if !ok {
		return nil
	}
	return value.(*UnitGroup)
}

// 定时任务，每十秒查看一次map中的事件组是否超时，超时就删除
func (r *Client) gcExpiredGroup() {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			r.groupUnitCallbackMap.Range(func(key, value interface{}) bool {
				group := value.(*UnitGroup)
				if now.After(group.expireAt) {
					r.groupUnitCallbackMap.Delete(key)
				}
				return true
			})
		}
	}
}

// 设置rmi服务地址（需要加上ip和端口），传进来的地址为url.parse可以解析的
func (r *Client) fillRMIAddr() error {
	addr := r.config.RMIServerAddr
	if addr == "" && r.config.HTTPBaseURL != "" {
		u, err := url.Parse(r.config.HTTPBaseURL)
		if err != nil {
			return err
		}
		port := u.Port()
		if port == "" {
			if u.Scheme == "http" {
				port = "80"
			} else {
				port = "443"
			}
		}
		addr = net.JoinHostPort(u.Hostname(), port)
	}
	r.config.RMIServerAddr = addr
	return nil
}

// 带tocken访问服务器查看是否存活
func (r *Client) healthCheck() error {
	healthUrl := fmt.Sprintf("%s/_/api/health_check", r.config.HTTPBaseURL)
	req, err := http.NewRequest(http.MethodGet, healthUrl, nil)
	req.Header.Set("x-token", r.config.Token)
	req.Header.Set("content-type", "application/json")
	client, _ := shttp.NewDefaultClient(nil)
	resp, err := client.Do(r.ctx, &shttp.Request{
		RawRequest: req,
	})
	if err != nil {
		return fmt.Errorf("health check failed for remote reverse server, error: %v", err)
	}

	respData := &ResponseBase{}
	body := resp.GetBody()
	err = json.Unmarshal(body, respData)
	if err != nil || respData.Code != CodeSuccess {
		return fmt.Errorf("invalid response from remote reverse server, response is: %s", string(body))
	}

	return nil
}

var (
	errCallbackNotSet = errors.New("callback is not set")
)

// 判断给定事件是否在事件组中，并且Callback函数不为空，则回连成功
func (r *Client) localCallUnitCallback(event *Event) error {
	//从当前的client的sync.map中使用事件组的id读取事件组
	g, groupExists := r.groupUnitCallbackMap.Load(event.GroupID)
	//如果存在
	if groupExists {
		group := g.(*UnitGroup)
		//从事件组中读取指定事件id的事件
		u, unitExists := group.units.Load(event.UnitId)
		if unitExists {
			unit := u.(*Unit)
			unit.Lock()
			defer unit.Unlock()
			if unit.Callback != nil {
				//证明服务器接收到了这个事件，也就是当前客户端连接过服务器
				logger.Infof("callback called %#v", event)
				err := unit.Callback(event)
				if err != nil {
					return err
				} else {
					//callbackCallde设置为1
					atomic.StoreInt32(&group.callbackCalled, 1)
				}
			} else {
				return errCallbackNotSet
			}
			r.groupUnitCallbackMap.Delete(event.GroupID)
		} else {
			// 这个 warnf 和下面的一般应该不会出现，先忽略
			logger.Warnf("unit not found for event %#v", event)
		}
	} else {
		logger.Warnf("group not found for event %#v", event)
	}
	return nil
}

type fetchEventResponse struct {
	ResponseBase
	Event []*Event `json:"data"`
}

type remoteFetchEventRequest struct {
	GroupToDelete []string `json:"group_to_delete"`
}

// 从服务器获取所有事件，然后遍历事件与本地事件组做对比，如果有相同的，添加到删除列表中，对应组的callback设置为1
func (r *Client) remoteFetchEvent() error {
	fetchUrl := fmt.Sprintf("%s/_/api/fetch", r.config.HTTPBaseURL)
	data, err := json.Marshal(r.groupToDelete)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, fetchUrl, bytes.NewReader(data))
	req.Header.Set("x-token", r.config.Token)
	req.Header.Set("content-type", "application/json")
	client, _ := shttp.NewDefaultClient(nil)
	resp, err := client.Do(r.ctx, &shttp.Request{
		RawRequest: req,
	})
	if err != nil {
		return err
	}
	//从服务器获取所有的事件
	body := fetchEventResponse{Event: make([]*Event, 0)}
	err = json.Unmarshal(resp.GetBody(), &body)
	if err != nil {
		return err
	}
	if body.Code != CodeSuccess {
		return fmt.Errorf("invalid response code %v", body.Code)
	}
	r.groupToDelete.GroupToDelete = make([]string, 0, 10)
	for _, item := range body.Event {
		err := r.localCallUnitCallback(item)
		if err != nil {
			if err != errCallbackNotSet {
				logger.Error(err)
			}
		}

		r.groupToDelete.GroupToDelete = append(r.groupToDelete.GroupToDelete, item.GroupID) //将当前组放入删除的列表中
	}
	return nil
}
