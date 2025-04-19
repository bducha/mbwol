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
	flag.Parse()

	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	grub.InitHostConfigs(*configFilePath)

	go api.ListenAndServe()

	err := tftp.ListenAndServeTFTP()
	if err != nil {
		log.Fatalln("Error serving TFTP server : ", err)
	}
}