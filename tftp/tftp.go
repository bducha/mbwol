package tftp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/bducha/mbwol/grub"
)

const (
	TFTP_PORT               = 69
	BLOCK_SIZE              = 512
	PACKET_SIZE             = 516
	OPCODE_RRQ              = 1
	OPCODE_DATA             = 3
	OPCODE_ACK              = 4
	OPCODE_ERROR            = 5
	OPCODE_OACK             = 6
	ERROR_CODE_ILLEGAL_TFTP = 4
)

type TFTPPacket struct {
	Opcode  int
	Payload []byte
}

type Connection struct {
	Conn *net.UDPConn
	BlockSize int
	DataLen int64
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
			go handleRRQ(buffer, clientAddr)
		default:
			slog.Warn("Unsupported opcode", "opcode", opcode)
		}
	}
}

func handleRRQ(packet []byte, clientAddr *net.UDPAddr) {
	parts := bytes.Split(packet[2:], []byte{0})
	filename := parts[0]
	slog.Debug("RRQ received", "filename", string(filename), "client", clientAddr.IP.String())
	slog.Debug("Parts", "parts", parts)
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		slog.Debug("Part", "part", string(part))
	}
	content := grub.GetConfigByIp(clientAddr.IP.String())
	slog.Debug("Retreived content", "content", content)
	data := []byte(content)
	block := 1
	udpConn, err := net.DialUDP("udp", nil, clientAddr)
	conn := Connection{
		Conn:      udpConn,
		BlockSize: BLOCK_SIZE,
		DataLen: 		int64(len(data)),
	}
	if err != nil {
		slog.Error("Error dialing UDP", "err", err.Error(), "client", clientAddr.String())
		return
	}
	defer udpConn.Close()

	// Handle options
	err = handleOptions(parts[2:], &conn)
	if err != nil {
		slog.Error("Error handling options", "err", err.Error())
		return
	}

	sendData(data, &conn, block)
}

func handleOptions(options [][]byte, conn *Connection) error {
	// No options
	if len(options[0]) == 0 {
		return nil
	}

	opts := make(map[string]string)

	oackPacket := []byte{0, byte(OPCODE_OACK)}

	for i, option := range options {
		if i%2 == 0 && string(option) != ""{
			slog.Debug("Option", "option", string(option))
			switch string(option) {
				case "tsize":
					slog.Debug("Tsize option", "tsize", string(options[i+1]))
					opts["tsize"] = strconv.FormatInt(conn.DataLen, 10)
			}
			if string(option) == "blksize" {
				blockSize := options[i+1]
				slog.Debug("Block size option", "blockSize", string(blockSize))
				opts["blksize"] = string(blockSize)
				
				size,_ := binary.Uvarint(blockSize)
				conn.BlockSize = int(size)
			}
		}
		// Ignore other options
	}

	for key, value := range opts {
		oackPacket = append(oackPacket, []byte(key)...)
		oackPacket = append(oackPacket, 0)
		oackPacket = append(oackPacket, []byte(value)...)
		oackPacket = append(oackPacket, 0)
	}

	_, err := conn.Conn.Write(oackPacket)
	if err != nil {
		slog.Error("Error sending data packet", "err", err.Error())
		return err
	}

	ackPacket, err := receivePacket(conn.Conn)
	if err != nil {
		slog.Error("Error receiving ACK packet:", "err", err)
		return err
	}
	if ackPacket.Opcode != OPCODE_ACK {
		slog.Error("Expected ACK packet", "receivedOpcode", ackPacket.Opcode)
		return err
	}
	slog.Debug("Received ACK")
	return nil
}


func sendData(data []byte, conn *Connection, blockNumber int) error {
	start := (blockNumber - 1) * conn.BlockSize
	end := start + conn.BlockSize
	if end > len(data) {
		end = len(data)
	}
	blockData := data[start:end]
	packet := createDataPacket(blockNumber, blockData)
	slog.Debug("Sending packet", "packet", packet)
	_, err := conn.Conn.Write(packet)
	if err != nil {
		slog.Error("Error sending data packet", "err", err.Error())
		return err
	}

	ackPacket, err := receivePacket(conn.Conn)
	if err != nil {
		slog.Error("Error receiving ACK packet:", "err", err)
		return err
	}
	if ackPacket.Opcode != OPCODE_ACK {
		slog.Error("Expected ACK packet", "receivedOpcode", ackPacket.Opcode)
		return err
	}
	slog.Debug("Received ACK")

	if len(data) == end {
		return nil
	}

	next, _ := binary.Uvarint(ackPacket.Payload[0:1])

	sendData(data, conn, int(next))
	return nil
}

func createDataPacket(blockNumber int, data []byte) []byte {
	opCode := []byte{0, byte(OPCODE_DATA)}
	blockNum := []byte{byte(blockNumber >> 8), byte(blockNumber)}
	packet := append(opCode, blockNum...)
	packet = append(packet, data...)
	return packet
}

func receivePacket(conn *net.UDPConn) (*TFTPPacket, error) {
	err := conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, 516)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		if err, ok := err.(net.Error); ok && err.Timeout() {
			// Handle timeout error
			return nil, errors.New("read operation timed out")
		}
		return nil, err
	}

	// Clear the deadline
	err = conn.SetReadDeadline(time.Time{})
	if err != nil {
		return nil, err
	}

	opcode := int(buffer[1])
	payload := buffer[2:n]
	return &TFTPPacket{Opcode: opcode, Payload: payload}, nil
}
