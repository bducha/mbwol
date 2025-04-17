package tftp

import (
	"bytes"
	"fmt"
	"log/slog"
	"net"

	"github.com/bducha/mbwol/grub"
)

const (
	TFTP_PORT    = 69
	BLOCK_SIZE   = 512
	PACKET_SIZE  = 516
	OPCODE_RRQ   = 1
	OPCODE_DATA  = 3
	OPCODE_ACK   = 4
	OPCODE_ERROR = 5
	ERROR_CODE_ILLEGAL_TFTP = 4
)

type TFTPPacket struct {
	Opcode  int
	Payload []byte
}

func ListenAndServeTFTP() error {

	addr := &net.UDPAddr{Port: TFTP_PORT, IP: net.ParseIP("0.0.0.0")}
	conn, err := net.ListenUDP("udp", addr)

	if err != nil {
		return fmt.Errorf("error listening on UDP port : %s", err)
	}

	defer conn.Close()

	slog.Info("TFTP Server listening", "addr", conn.LocalAddr().String())

	for {
		buffer := make([]byte, PACKET_SIZE)
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			slog.Error("Error reading from UDP", "err", err.Error())
		}

		slog.Debug("Received packet", "from", clientAddr.String(), "size", n)
		opcode := int(buffer[1])
		switch opcode {
		case OPCODE_RRQ:
			handleRRQ(buffer, clientAddr)
		default:
			slog.Warn("Unsupported opcode", "opcode", opcode)
		}
	}
}


func handleRRQ(packet []byte, clientAddr *net.UDPAddr) {
	parts := bytes.Split(packet[2:], []byte{0})
	filename := parts[0]
	slog.Debug("RRQ received", "filename", string(filename), "client", clientAddr.IP.String())

	content := grub.GetConfigByIp(clientAddr.IP.String())
	slog.Debug("Retreived content", "content", content)
	data := []byte(content)
	block := 1
	conn, err := net.DialUDP("udp", nil, clientAddr)
	if err != nil {
		slog.Error("Error dialing UDP", "err", err.Error(), "client", clientAddr.String())
		return
	}
	defer conn.Close()

	for i:= 0; i < len(data); i += BLOCK_SIZE {
		end := i + BLOCK_SIZE
		if end > len(data) {
			end = len(data)
		}
		blockData := data[i:end]
		packet := createDataPacket(block, blockData)
		slog.Debug("Sending packet", "packet", packet)
		_, err = conn.Write(packet)
		if err != nil {
			slog.Error("Error sending data packet", "err", err.Error())
			return
		}

		ackPacket, err := receivePacket(conn)
		if err != nil {
			slog.Error("Error receiving ACK packet:", "err", err)
			return
		}
		if ackPacket.Opcode != OPCODE_ACK {
			slog.Error("Expected ACK packet", "receivedOpcode", ackPacket.Opcode)
			return
		}
		slog.Debug("Received ACK")
		block++
	}
}

func createDataPacket(blockNumber int, data []byte) []byte {
	opCode := []byte{0, byte(OPCODE_DATA)}
	blockNum := []byte{byte(blockNumber >> 8), byte(blockNumber)}
	packet := append(opCode, blockNum...)
	packet = append(packet, data...)
	return packet
}

func receivePacket(conn *net.UDPConn) (*TFTPPacket, error) {
	buffer := make([]byte, 516)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		return nil, err
	}
	opcode := int(buffer[1])
	payload := buffer[2:n]
	return &TFTPPacket{Opcode: opcode, Payload: payload}, nil
}