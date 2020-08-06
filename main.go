package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/tarm/serial"
	"io"
	"net"
	"strconv"
	"strings"
)

type SerialConfig struct {
	baud   int
	device string
}

type GdbConfig struct {
	tcpPort string
}

type serialport io.ReadWriteCloser

func initializeSerialPort(device string, baud int) serialport {
	options := &serial.Config{
		Name: device,
		Baud: baud,
	}

	port, err := serial.OpenPort(options)

	if err != nil {
		fmt.Println("Could not open serial port!")
	}

	return port
}

func main() {
	baudratePtr := flag.Int("baudrate", 115200, "Serial baudrate speed")
	serialdevicePtr := flag.String("device", "", "Serial device, e.g. /dev/ttyUSB0")

	tcpPortPtr := flag.String("port", "8888", "GDB port")

	flag.Parse()

	serialConfig := SerialConfig{*baudratePtr, *serialdevicePtr}
	gdbConfig := GdbConfig{*tcpPortPtr}

	fmt.Println("baudrate: ", serialConfig.baud)
	fmt.Println("device: ", serialConfig.device)
	fmt.Println("tcp port", gdbConfig.tcpPort)

	gdbHexStr, _ := hex.DecodeString("d0febd272c01bfaf2801beaf25f0a0033001c4")

	if len(gdbHexStr)%2 != 0 {
		paddedHexStr := make([]byte, len(gdbHexStr)+1)
		copy(paddedHexStr, gdbHexStr[0:])
		gdbHexStr = paddedHexStr
	}

	for i := 0; i < len(gdbHexStr); i += 4 {
		converted := binary.LittleEndian.Uint32(gdbHexStr[i:])
		fmt.Printf("%#08x \n", converted)
	}

	port := initializeSerialPort(serialConfig.device, int(serialConfig.baud))

	listener, err := net.Listen("tcp", ":"+*tcpPortPtr)

	if err != nil {
		fmt.Println(err)
		return
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		handleGdbClient(conn, port)
	}

	defer port.Close()
}

func formatGdbPacket(message string) string {
	byteSlice := []byte(message)
	var checksumTotal int64

	for _, b := range byteSlice {
		checksumTotal += int64(b)
	}

	checksumTotal = checksumTotal % 256

	hexChecksum := strconv.FormatInt(checksumTotal, 16)

	if len(hexChecksum) < 2 {
		hexChecksum = "0" + hexChecksum
	}

	gdbFormattedPackage := "+$" + message + "#" + hexChecksum

	return gdbFormattedPackage
}

func acknowledgeWithOk(conn net.Conn) {
	conn.Write([]byte(formatGdbPacket("OK")))
}

func acknowledgeWithEmpty(conn net.Conn) {
	conn.Write([]byte("+$#00"))
}

func getPredefinedResponse(conn net.Conn, request string, requests []string, acknowledgementFunc func(net.Conn)) bool {
	var fulfilled bool

	for _, entry := range requests {
		fulfilled = strings.Contains(request, entry)

		if fulfilled {
			acknowledgementFunc(conn)
			break
		}
	}

	return fulfilled
}

func readMemory(serialPort serialport, request string) string {
	return sendSerialCommand(serialPort, formatGdbPacket(request))
}

func cleanupSerialData(data string) string {
	fmt.Println(data)
	startIndex := strings.Index(data, "$")

	if startIndex == -1 {
		fmt.Println("Not a valid gdb request package")
		return ""
	}

	endIndex := strings.Index(data, "#")

	if endIndex == -1 {
		fmt.Println("Not a valid gdb terminating request package")
		return ""
	}

	return data[startIndex+1 : endIndex]
}

func writeMemory(conn net.Conn, serialPort serialport, request string) string {
	return sendSerialCommand(serialPort, formatGdbPacket(request))
}

func sendSerialCommand(serialPort serialport, cmd string) string {
	tmpBuffer := make([]byte, 1)
	readBuffer := make([]byte, 4096)

	fmt.Println("Sending the following command", cmd)

	n, err := serialPort.Write([]byte(cmd))

	fmt.Println("Num bytes written", n)

	if err != nil {
		fmt.Println("Could not push read registers command to PSX", err)
	}

	for {
		_, err := serialPort.Read(tmpBuffer)

		if err != nil {
			fmt.Println("Could not read registers from PSX", err)
			break
		} else {
			readBuffer = append(readBuffer, tmpBuffer...)
			if string(tmpBuffer) == "#" {
				break
			}
		}
	}

	fmt.Printf("readbuffer is: %s", readBuffer)

	cleanedData := cleanupSerialData(string(readBuffer))
	fmt.Println("cleaned data is ", cleanedData)

	return formatGdbPacket(cleanedData)
}

func readRegisters(serialPort serialport) string {
	return sendSerialCommand(serialPort, "$g#67")
}

func readSingleRegister(serialPort serialport, request string) string {
	return sendSerialCommand(serialPort, formatGdbPacket(request))
}

func writeSingleRegister(conn net.Conn, serialPort serialport, request string) {
	_, err := serialPort.Write([]byte(formatGdbPacket(request)))

	if err != nil {
		fmt.Println("Could not push data to PSX", err)
	}

	acknowledgeWithOk(conn)

}

func parseGdbRequest(conn net.Conn, serialPort serialport, request string) {
	var emptyResponsesFor = []string{"qTStatus", "vMustReplyEmpty", "qC", "vCont?", "X", "qSymbol::"}
	var okResponseFor = []string{"Hg0", "Hg1", "Hc-1", "Hc0", "Hc1", "qThreadExtraInfo", "qfThreadInfo", "qsThreadInfo"}

	acknowledged := getPredefinedResponse(conn, request, emptyResponsesFor, acknowledgeWithEmpty)

	if acknowledged {
		return
	}

	acknowledged = getPredefinedResponse(conn, request, okResponseFor, acknowledgeWithOk)

	if acknowledged {
		return
	}

	switch {
	case string(request[0]) == "m":
		fmt.Println("Reading a mem addr", request)
		value := readMemory(serialPort, request)
		fmt.Println("going to write the following", value)
		conn.Write([]byte(value))
	case string(request[0]) == "M":
		value := writeMemory(conn, serialPort, request)
		conn.Write([]byte(value))
	case string(request[0]) == "g":
		registers := readRegisters(serialPort)
		conn.Write([]byte(registers))
	case string(request[0]) == "p":
		registerValue := readSingleRegister(serialPort, request)
		conn.Write([]byte(registerValue))
	case string(request[0]) == "c":
		fmt.Println("Continuing")
		value := sendSerialCommand(serialPort, "$c#22")
		conn.Write([]byte(value))
	case string(request[0]) == "P":
		writeSingleRegister(conn, serialPort, request)
	case strings.Contains(request, "qSupported"):
		msg := "PacketSize=50"
		conn.Write([]byte(formatGdbPacket(msg)))
	case string(request[0]) == "?":
		conn.Write([]byte(formatGdbPacket("S00")))
	case strings.Contains(request, "qOffsets"):
		msg := "Text=0;Data=0;Bss=0"
		conn.Write([]byte(formatGdbPacket(msg)))
	case strings.Contains(request, "qAttached"):
		conn.Write([]byte(formatGdbPacket("1")))
	}
}

func handlePacket(conn net.Conn, serialPort serialport, packet string) {
	fmt.Println("handlePacket: ", packet)
	if packet == "+" {
		return
	}

	startIndex := strings.Index(packet, "$")

	if startIndex == -1 {
		fmt.Println("Not a valid gdb request package")
		return
	}

	endIndex := strings.Index(packet, "#")

	if endIndex == -1 {
		fmt.Println("Not a valid gdb terminating request package")
		return
	}

	parseGdbRequest(conn, serialPort, packet[startIndex+1:endIndex])
}

func handleGdbClient(conn net.Conn, serialPort serialport) {
	defer conn.Close()

	var buff [2048]byte

	for {
		bytesRead, err := conn.Read(buff[0:])

		if err != nil {
			conn.Close()
			fmt.Println(err)
			return
		}

		request := string(buff[0:bytesRead])

		if bytesRead > 0 {

			handlePacket(conn, serialPort, request)
		}
	}
}
