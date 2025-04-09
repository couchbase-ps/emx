package utility

import (
	"os"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type TLSConfig struct {
	TlsKeyPath         string
	TlsCertificatePath string
}

// Logger to generate the application logs
func Logger() (logger log.Logger) {

	// Creating logger
	logger = log.NewLogfmtLogger(os.Stdout)
	logger = level.NewFilter(logger, level.AllowInfo())
	logger = log.With(logger, "ts", log.DefaultTimestamp, "caller", log.DefaultCaller)

	return logger
}
