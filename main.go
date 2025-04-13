package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/bducha/mbwol/tftp"
)

func main() {

	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	err := tftp.ListenAndServeTFTP()
	if err != nil {
		log.Fatalln("Error serving TFTP server : ", err)
	}
}