package main

import (
	"flag"
	"log"
	"log/slog"
	"os"

	"github.com/bducha/mbwol/api"
	"github.com/bducha/mbwol/grub"
	"github.com/bducha/mbwol/tftp"
)

func main() {

	configFilePath := flag.String("config", "mbwol.json", "Path to the config file")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	httpPort := flag.Int("http-port", 8000, "HTTP port to listen on")

	flag.Parse()

	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}

	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(h))

	grub.InitHostConfigs(*configFilePath)

	go api.ListenAndServe(*httpPort)

	err := tftp.ListenAndServeTFTP()
	if err != nil {
		log.Fatalln("Error serving TFTP server : ", err)
	}
}