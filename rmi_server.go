package reverkit

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/iami317/hubur"
	"net"
	"regexp"
	"strconv"
	"sync"
)

var (
	rmiDataEndMark   = []byte("fYqzYxJz")
	rmiDataStartMark = []byte("GykcaQgj")
	// i/StartMark/:token/:unit/:group/EndMark
	pathRegex = regexp.MustCompile(fmt.Sprintf("%s/%s/(?P<token>[^/]{0,32})/(?P<group>[^/]{0,32})/(?P<unit>[^/]{0,32})/%s$", internalAPIMark, rmiDataStartMark, rmiDataEndMark))
)

func handleRMIConn(token string, db *DB, internalGroupEventMap *sync.Map, conn *hubur.TimeoutConn) {
	defer func() {
		_ = conn.Close()
	}()
	b := make([]byte, 100)
	n, err := conn.Read(b)
	if err != nil {
		logger.Error(err)
		return
	}
	if n < 4 || string(b[:4]) != "JRMI" {
		logger.Errorf("unexpected data, %s", hubur.EscapeInvalidUTF8Byte(b[0:n]))
		return
	}
	var buf bytes.Buffer
	buf.Write([]byte{0x4e, 0x00}) // acknowledging support
	// then write client addr
	addr := conn.RemoteAddr().String()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		logger.Errorf("split host port error, %s", err)
		return
	}
	binary.BigEndian.PutUint16(b, uint16(len(host)))
	buf.Write([]byte{b[0], b[1]})
	buf.WriteString(host)
	buf.Write([]byte{0x00, 0x00})
	p, err := strconv.Atoi(port)
	if err != nil {
		logger.Error("convert port to int error")
		return
	}
	binary.BigEndian.PutUint16(b, uint16(p))
	buf.Write([]byte{b[0], b[1]})

	_, err = conn.Write(buf.Bytes())
	if err != nil {
		logger.Error("error when writing data to client")
		return
	}
	path, _ := conn.ReadUntil(rmiDataEndMark, 2048)
	pathMatch := pathRegex.FindStringSubmatch(string(path))
	if len(pathMatch) != 4 {
		logger.Errorf("failed to split path %s", hubur.EscapeInvalidUTF8Byte(path))
		return
	}
	_token := pathMatch[1]
	group := pathMatch[2]
	unit := pathMatch[3]
	if _token != generateHashedToken(token, group, unit) {
		logger.Errorf("invalid rmi token")
		return
	}
	logger.Infof("reverse rmi server received request [%s/%s], remoteAddr: %s", group, unit, conn.RemoteAddr().String())
	event := &Event{TimeStamp: hubur.TimeStampMs(),
		Request: hubur.EscapeInvalidUTF8Byte(path), RemoteAddr: addr,
		UnitId: unit, GroupID: group,
		EventType: EventTypeRMIVisit, EventSource: EventSourceInternal}
	err = db.storeEvent(event)
	if err != nil {
		logger.Error(err)
	}
	internalGroupEventMap.Store(event.GroupID, event)
}
