package azurewrapper

import (
	"os"
	"sync"

	"github.com/kthomas/go-logger"
)

var (
	log           *logger.Logger
	bootstrapOnce sync.Once
)

func init() {
	bootstrapOnce.Do(func() {
		lvl := os.Getenv("AZURE_LOG_LEVEL")
		if lvl == "" {
			lvl = "INFO"
		}
		var endpoint *string
		if os.Getenv("SYSLOG_ENDPOINT") != "" {
			endpt := os.Getenv("SYSLOG_ENDPOINT")
			endpoint = &endpt
		}
		log = logger.NewLogger("azurewrapper", lvl, endpoint)
	})
}
