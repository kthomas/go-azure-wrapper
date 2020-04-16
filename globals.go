package azurewrapper

import (
	"os"
	"sync"

	"github.com/kthomas/go-logger"
)

var (
	log             *logger.Logger
	awsDefaultVpcID string
	bootstrapOnce   sync.Once
)

func init() {
	bootstrapOnce.Do(func() {
		lvl := os.Getenv("AZURE_LOG_LEVEL")
		if lvl == "" {
			lvl = "INFO"
		}
		log = logger.NewLogger("azurewrapper", lvl, true)
	})
}
