package reverkit

import (
	"context"
	"github.com/iami317/hubur"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestRemoteMultiplex(t *testing.T) {
	assert := require.New(t)
	path, err := hubur.GetTempFilePath()
	assert.Nil(err)
	defer os.RemoveAll(path)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	remote, local, err := NewLocalTestServer(ctx, path)
	assert.Nil(err)
	assert.Nil(remote.Start(context.Background()))
	defer remote.Close()

	assert.Nil(err)
	runRMIClientTest(assert, local)
	runHTTPClientTest(assert, local)
}

func runHTTPClientTest(assert *require.Assertions, r *Client) {
	var callbackCalled int32 = 0
	callback := func(e *Event) error {
		atomic.AddInt32(&callbackCalled, 1)
		return nil
	}

	// 异步 callback 模式
	// register a url
	unit := r.NewUnit()
	visitURL := unit.GetVisitURL()
	unit.OnVisit(callback)

	var retry int32 = 3
	for retry > 0 {
		// trigger a connection
		resp, err := http.Get(visitURL)
		assert.Nil(err)

		_, err = ioutil.ReadAll(resp.Body)

		// validate token
		assert.Nil(err)

		time.Sleep(time.Second * 2)
		// 多次触发 callback 也应该只调用一次
		assert.Equal(atomic.LoadInt32(&callbackCalled), int32(1))
		retry -= 1
	}

	// 同步等待模式，然后超时
	syncUnit := r.NewUnit()
	err := syncUnit.group.Wait(3 * time.Second)
	assert.Error(err, "wait event timeout")

	// 同步等待模式
	syncUnit1 := r.NewUnit()
	visitURL = syncUnit1.GetVisitURL()
	go func() {
		time.Sleep(time.Second)
		_, err := http.Get(visitURL)
		assert.Nil(err)
	}()
	atomic.StoreInt32(&callbackCalled, 0)
	syncUnit1.OnVisit(callback)
	err = syncUnit1.group.Wait(5 * time.Second)
	assert.Nil(err)
}

func runRMIClientTest(assert *require.Assertions, r *Client) {
	var callbackCalled int32 = 0
	callback := func(e *Event) error {
		atomic.AddInt32(&callbackCalled, 1)
		return nil
	}

	unit := r.NewUnit()
	rmiURL := unit.GetRmiURL()
	unit.OnVisit(callback)

	var retry int32 = 3
	for retry > 0 {
		u, err := url.Parse(rmiURL)
		assert.Nil(err)
		conn, err := net.Dial("tcp", u.Host)
		assert.Nil(err)
		conn = hubur.NewTimeoutConn(conn, time.Second*5)
		_, _ = conn.Write([]byte("JRMIAAAAAA"))
		data := make([]byte, 100)
		_, _ = conn.Read(data)
		_, _ = conn.Write([]byte(u.Path))
		// 多次触发 callback 也应该只调用一次
		time.Sleep(time.Second * 3)
		assert.Equal(atomic.LoadInt32(&callbackCalled), int32(1))
		retry -= 1
		_ = conn.Close()
	}
}
