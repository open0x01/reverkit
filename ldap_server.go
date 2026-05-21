package reverkit

import (
	"fmt"
	"github.com/iami317/hubur"
	ldap "github.com/open0x01/reverkit/ldap"
	"regexp"
	"sync"
)

var pathRegexldap = regexp.MustCompile(fmt.Sprintf("%s/(?P<token>[^/]{0,32})/(?P<group>[^/]{0,32})/(?P<unit>[^/]{0,32})/$", internalAPIMark))

func handleLdapConn(token string, db *DB, internalGroupEventMap *sync.Map, conn *hubur.TimeoutConn) {
	s := ldap.NewServer()
	routes := ldap.NewRouteMux()
	routes.Bind(func(writer ldap.ResponseWriter, message *ldap.Message) {
		res := ldap.NewBindResponse(ldap.LDAPResultSuccess)
		writer.Write(res)
		return
	})
	routes.Search(func(writer ldap.ResponseWriter, message *ldap.Message) {
		r := message.GetSearchRequest()
		path := fmt.Sprintf("%s", r.BaseObject()) //i/63c3ab/8moh/fl4q/
		pathMatch := pathRegexldap.FindStringSubmatch(path)
		if len(pathMatch) != 4 {
			logger.Errorf("failed to split path %s", hubur.EscapeInvalidUTF8Byte([]byte(path)))
			return
		}
		_token := pathMatch[1]
		group := pathMatch[2]
		unit := pathMatch[3]
		if _token != generateHashedToken(token, group, unit) {
			logger.Errorf("invalid ldap token")
			return
		}
		logger.Infof("reverse ldap server received request [%s/%s], remoteAddr: %s", group, unit, conn.RemoteAddr().String())
		event := &Event{TimeStamp: hubur.TimeStampMs(),
			Request: hubur.EscapeInvalidUTF8Byte([]byte(path)), RemoteAddr: conn.RemoteAddr().String(),
			UnitId: unit, GroupID: group,
			EventType: EventTypeRMIVisit, EventSource: EventSourceInternal}
		err := db.storeEvent(event)
		if err != nil {
			return
		}
		internalGroupEventMap.Store(event.GroupID, event)
		return
	})
	s.Handle(routes)
	s.UseConnServer(conn)
}
