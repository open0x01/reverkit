package ldapserver

import (
	"github.com/iami317/logx"
)

var Logger *logx.Logger

func init() {
	Logger = logx.New()
	Logger.SetLevel("Info")
}
