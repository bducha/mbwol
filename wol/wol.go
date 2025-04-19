package wol

import (
	"fmt"
	"log/slog"
	"net"
)

func SendMagicPacket(macAddress string, ipAddress string) error {
	mac, err := net.ParseMAC(macAddress)
	if err != nil {
		slog.Error("error parsing mac address", "error", err.Error(), "macAddress", macAddress)
		return fmt.Errorf("error parsing mac address : %s", err.Error())
	}

	// header
	packet := make([]byte, 102)
	for i := range 6 {
		packet[i] = 0xFF
	}
	// payload
	for i := 1; i <= 16; i++ {
		copy(packet[i*6:], mac)
	}
	conn, err := net.Dial("udp", fmt.Sprintf("%s:9", ipAddress))
	if err != nil {
		slog.Error("error connecting on ip address", "error", err.Error())
		return fmt.Errorf("error connecting on ip address : %s", err.Error())
	}
	defer conn.Close()
	_, err = conn.Write(packet)

	if err != nil {
		slog.Error("error sending magic packet", "error", err.Error())
		return fmt.Errorf("error sending magic packet : %s", err.Error())
	}
	return nil
}