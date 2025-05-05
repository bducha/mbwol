package main

import (
	"log"
	"log/slog"
	"os"
	"strconv"

	"github.com/bducha/mbwol/api"
	"github.com/bducha/mbwol/grub"
	"github.com/bducha/mbwol/tftp"
)

var (
	configFilePath = "mbwol.json"
	verbose        = false
	httpPort       = 8000
)

func main() {
	parseEnv()

	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}

	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(h))

	grub.InitHostConfigs(configFilePath)

	go api.ListenAndServe(httpPort)

	err := tftp.ListenAndServeTFTP()
	if err != nil {
		log.Fatalln("Error serving TFTP server : ", err)
	}
}

func parseEnv() {
	if c := os.Getenv("MBWOL_CONFIG_FILE"); c != "" {
		configFilePath = c
	}
	var err error
	if c := os.Getenv("MBWOL_HTTP_PORT"); c != "" {
		httpPort, err = strconv.Atoi(c)
		if err != nil {
			log.Fatalln("Error parsing HTTP_PORT : ", err)
		}
	}
	if c := os.Getenv("MBWOL_VERBOSE"); c != "" {
		verbose, err = strconv.ParseBool(c)
		if err != nil {
			log.Fatalln("Error parsing VERBOSE : ", err)
		}
	}
}
