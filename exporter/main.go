package main

import (
	"exporter/exporter/couchbase"
	"exporter/exporter/utility"
	"flag"
	"net/http"
	"os"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const EMX_PORT = "9876"

func main() {

	clientCert := flag.String("clientCert", "", "Path to the client certificate file to authenticate this client with couchbase-server")
	clientKey := flag.String("clientKey", "", "Path to the client private key file to authenticate this client with couchbase-server")

	tlsCertPath := flag.String("tlsCert", "", "Path to the server certificate file for HTTPS")
	tlsKeyPath := flag.String("tlsKey", "", "Path to the server private key file for HTTPS")

	disableTLS := flag.Bool("disableTLS", false, "Include if TLS is to be disabled, will default to false enabling HTTPS only mode")

	var tlsConfig utility.TLSConfig

	flag.Parse()
	tlsConfig.TlsKeyPath = *clientKey
	tlsConfig.TlsCertificatePath = *clientCert

	// Instantiating the logger object
	logger := utility.Logger()
	// Triggering the couchbase emx stats metrics creation
	couchbase.CreateCouchbaseEMXStatsMetrics(logger, tlsConfig)
	var port string = ""
	port = os.Getenv("EMX_PORT")
	if port == "" {
		port = EMX_PORT
	}

	if !*disableTLS {
		level.Info(logger).Log("Event", "TLS Enabled")

		var tlsKey = os.Getenv("EMX_TLS_KEY")
		if tlsKey == "" {
			tlsKey = *tlsKeyPath
		}
		var tlsCert = os.Getenv("EMX_TLS_CERT")
		if tlsCert == "" {
			tlsCert = *tlsCertPath
		}
		if tlsCert == "" || tlsKey == "" {
			level.Error(logger).Log("Error", "TLS enabled but no CERT or KEY file declared.")
			os.Exit(1)
		}

		level.Info(logger).Log("Event", "Exposing metrics at the endpoint '/metrics' on port '"+port+"'.")
		http.Handle("/metrics", promhttp.Handler())
		err := http.ListenAndServeTLS(":"+EMX_PORT, tlsCert, tlsKey, nil)
		if err != nil {
			level.Error(logger).Log("Error - failed to start HTTPS server", err)
			os.Exit(1)
		}

	} else {
		level.Info(logger).Log("Event", "TLS Disabled")

		level.Info(logger).Log("Event", "Exposing metrics at the endpoint '/metrics' on port '"+port+"'.")
		http.Handle("/metrics", promhttp.Handler())
		err := http.ListenAndServe(":"+port, nil)
		if err != nil {
			level.Error(logger).Log("Error - failed to start HTTP server", err)
			os.Exit(1)
		}
	}

	// Exposing the metrics endpoint of the exporter
	level.Info(logger).Log("Event", "Couchbase Enhanced Metrics Exporter started successfully")

}
