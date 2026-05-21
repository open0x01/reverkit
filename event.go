package reverkit

import (
	"github.com/iami317/hubur"
	"github.com/iami317/logx"
)

var (
	logger = logx.Default
)

type eventType string
type eventSource string

const (
	EventTypeHTTPVisit eventType = "http"
	EventTypeRMIVisit  eventType = "rmi"
	EventTypeLDAPVisit eventType = "ldap"

	EventSourceInternal eventSource = "internal"
	EventSourcePublic   eventSource = "public"

	internalAPIMark        = "i"
	publicAPIMark          = "p"
	payloadTemplateAPIMark = "t"

	tempDBFilePrefix = "temp-reverse-db-"
)

type Event struct {
	ID          int64       `json:"id"`
	GroupID     string      `json:"group_id"`
	UnitId      string      `json:"unit_id"`
	TimeStamp   int64       `json:"time_stamp"`
	EventSource eventSource `json:"event_source"`
	EventType   eventType   `json:"event_type"`
	// 字符串，方便去序列化 http 传输等
	Request    string `json:"request"`
	RemoteAddr string `json:"remote_addr"`
}

func generateHashedToken(token, groupID, unitID string) string {
	return hubur.Sha256String(token + groupID + unitID)[:6]
}
