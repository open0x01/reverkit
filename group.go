package reverkit

import (
	"errors"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

var (
	errFetchEventTimeout = errors.New("fetch event timeout")

	ErrReverseHTTPServerDown = errors.New("reverse http server is down")
)

type Callback func(event *Event) error

const defaultUnitExpireTime = 60 * time.Second

type Unit struct {
	sync.Mutex
	client   *Client
	id       string
	group    *UnitGroup
	Callback Callback //成功匹配到回连目标时的回调函数
	Data     interface{}
}

type UnitGroup struct {
	id string
	// {unit_id: unit}
	units          sync.Map
	callbackCalled int32
	// 超时时候，将无法被回调
	expireAt time.Time
}

func (g *UnitGroup) Id() string {
	return g.id
}

func (g *UnitGroup) Join(unit *Unit) {
	g.units.Store(unit.id, unit)
}

func (g *UnitGroup) Wait(timeout time.Duration) error {
	if timeout > defaultUnitExpireTime {
		logger.Warnf("reverse fetch timeout %v is bigger than default timeout %v", timeout, defaultUnitExpireTime)
		timeout = defaultUnitExpireTime
	}
	for i := time.Duration(0); i <= timeout; i += time.Second {
		if atomic.LoadInt32(&g.callbackCalled) == 1 {
			return nil
		}
		time.Sleep(time.Second)
	}
	return errFetchEventTimeout
}

func (u *Unit) OnVisit(callback Callback) {
	u.Lock()
	defer u.Unlock()
	u.Callback = callback
}

// 获取访问反连平台的 url，注意，这个 url 后面可以任意追加，保持前缀不变即可匹配
func (u *Unit) GetVisitURL() string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/", u.client.config.HTTPBaseURL, internalAPIMark,
		generateHashedToken(u.client.config.Token, u.group.id, u.id), u.group.id, u.id)
}

func (u *Unit) GetEncodedVisitURL() string {
	v := u.GetVisitURL()
	return url.QueryEscape(v)
}

func (u *Unit) GetRmiURL() string {
	return fmt.Sprintf("rmi://%s/%s/%s/%s/%s/%s/%s/", u.client.config.RMIServerAddr, internalAPIMark, rmiDataStartMark,
		generateHashedToken(u.client.config.Token, u.group.id, u.id), u.group.id, u.id, rmiDataEndMark)
}

func (u *Unit) GetLdapURL() string {
	return fmt.Sprintf("ldap://%s/%s/%s/%s/%s/", u.client.config.RMIServerAddr, internalAPIMark,
		generateHashedToken(u.client.config.Token, u.group.id, u.id), u.group.id, u.id)
}

func (u *Unit) Wait(timeout time.Duration) error {
	return u.group.Wait(timeout)
}

func (u *Unit) GroupId() string {
	return u.group.id
}

func (u *Unit) UnitId() string {
	return u.id
}
