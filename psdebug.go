package main

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"go.bug.st/serial"
)

type serialport io.ReadWriteCloser

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

	gdbFormattedPackage := "$" + message + "#" + hexChecksum

	return gdbFormattedPackage
}

func acknowledgeWithOk(conn net.Conn) {
	conn.Write([]byte("+"))
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

func readSinglePsxReqister(serialPort serialport, num string) string {
	serialPort.Write([]byte("p"))
	serialPort.Write([]byte(stringToPsxBytes(num, true)))

	reader := bufio.NewReader(serialPort)

	reply, err := reader.ReadString('+')
	if err != nil {
		panic(err)
	}

	delimiterIndex := strings.Index(reply, "+")

	return reply[:delimiterIndex]
}

func writeSinglePsxReqister(serialPort serialport, input string) string {
	serialPort.Write([]byte("P"))
	serialPort.Write([]byte(stringToPsxBytes(input, false)))

	reader := bufio.NewReader(serialPort)

	reply, err := reader.ReadString('+')
	if err != nil {
		panic(err)
	}

	delimiterIndex := strings.Index(reply, "+")

	return reply[:delimiterIndex]
}

func readPsxRegisters(serialPort serialport) string {
	serialPort.Write([]byte("g"))
	reader := bufio.NewReader(serialPort)

	reply, err := reader.ReadString('+')
	if err != nil {
		panic(err)
	}

	delimiterIndex := strings.Index(reply, "+")

	startIndex := strings.Index(reply, "00000000")

	return reply[startIndex:delimiterIndex]
}

func parseGdbRequestArguments(buffer string) (string, string) {
	addrEnd := strings.Index(buffer, ",")

	addr := buffer[1:addrEnd]

	size := buffer[addrEnd+1:]

	return addr, size
}

func readPsxMemory(serialPort serialport, addr string, size string) string {
	serialPort.Write([]byte("m"))
	serialPort.Write([]byte(stringToPsxBytes(addr, true)))
	serialPort.Write([]byte(stringToPsxBytes(size, true)))

	reader := bufio.NewReader(serialPort)

	reply, err := reader.ReadString('+')
	if err != nil {
		panic(err)
	}

	delimiterIndex := strings.Index(reply, "+")

	return reply[:delimiterIndex]
}

func writePsxMemory(serialPort serialport, addr string, size string, data []byte) {
	serialPort.Write([]byte("M"))
	serialPort.Write([]byte(stringToPsxBytes(addr, true)))
	serialPort.Write([]byte(stringToPsxBytes(size, true)))

	var checksumTotal int64

	for _, b := range data {
		checksumTotal += int64(b)
	}

	checksumTotal = checksumTotal % 256

	hexChecksum := strconv.FormatInt(checksumTotal, 16)

	serialPort.Write(stringToPsxBytes(hexChecksum, true))
	serialPort.Write(data)

	reader := bufio.NewReader(serialPort)

	_, err := reader.ReadString('+')
	if err != nil {
		panic(err)
	}
}

func writeBreakpointToPsx(serialPort serialport, addr string) {
	serialPort.Write([]byte("Z"))

	reader := bufio.NewReader(serialPort)

	_, err := reader.ReadString('+')
	if err != nil {
		panic(err)
	}
}

func clearBreakPointFromPsx(serialPort serialport, addr string) {
	serialPort.Write([]byte("z"))

	reader := bufio.NewReader(serialPort)

	_, err := reader.ReadString('+')
	if err != nil {
		panic(err)
	}
}

func stringToPsxBytes(input string, bigendian bool) []byte {
	result := make([]byte, 4)

	for (8 - len(input)) > 0 {
		input = "0" + input
	}

	stringDecodedToBytes, err := hex.DecodeString(input)
	if err != nil {
		log.Fatal(err)
	}

	if bigendian {
		bigEndianInt := binary.BigEndian.Uint32(stringDecodedToBytes)

		binary.LittleEndian.PutUint32(result, bigEndianInt)
	} else {
		bigEndianInt := binary.LittleEndian.Uint32(stringDecodedToBytes)

		binary.LittleEndian.PutUint32(result, bigEndianInt)
	}

	return result
}

func parseMemoryWrite(buffer string) (string, string, []byte) {
	addrEnd := strings.Index(buffer, ",")

	addr := buffer[1:addrEnd]

	writeEnd := strings.Index(buffer, ":")

	size := buffer[addrEnd+1 : writeEnd]

	data, err := hex.DecodeString(buffer[writeEnd+1:])

	if err != nil {
		fmt.Println("Could not decode data string to hex")
	}

	return addr, size, data
}

func parseRegisterWrite(buffer string) string {
	startIndex := strings.Index(buffer, "=")

	addr := buffer[startIndex+1 : startIndex+9]

	return addr
}

func parseBreakpointWrite(buffer string) string {
	// example command: $Z0,80100018,4#a8

	startIndex := strings.Index(buffer, ",")

	addr := buffer[startIndex+1 : startIndex+9]

	return addr
}

func memoryInValidRange(addr string) bool {
	for (8 - len(addr)) > 0 {
		addr = "0" + addr
	}

	memHexBytes, err := hex.DecodeString(addr)

	if err != nil {
		fmt.Println("Could not decode hex address", addr)
	}

	memInt := binary.BigEndian.Uint32(memHexBytes)

	if memInt > 0x80010000 && memInt < 0x801FFF00 {
		return true
	}

	return false
}

func parseGdbRequest(conn net.Conn, serialPort serialport, cachedOpcodes map[string]string, request string) {
	var emptyResponsesFor = []string{"qTStatus", "vMustReplyEmpty", "qC", "vCont?", "qSymbol::"}
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
	case strings.Contains(request, "qXfer:features:read:target.xml"):
		xmlStartIndex := strings.Index(request, "xml")
		argOneIndex := xmlStartIndex + strings.Index(request[xmlStartIndex:], ":")
		argTwoIndex := xmlStartIndex + strings.Index(request[xmlStartIndex:], ",")

		argOneStr := request[argOneIndex+1 : argTwoIndex]
		argTwoStr := request[argTwoIndex+1:]

		for len(argOneStr) < 8 {
			argOneStr = "0" + argOneStr
		}

		for len(argTwoStr) < 8 {
			argTwoStr = "0" + argTwoStr
		}

		dataOffsetBytes, _ := hex.DecodeString(argOneStr)
		dataLengthBytes, _ := hex.DecodeString(argTwoStr)

		dataOffset := binary.BigEndian.Uint32(dataOffsetBytes)
		dataLength := binary.BigEndian.Uint32(dataLengthBytes)

		targetDescriptionBinXML := []byte(TargetDescriptionXML)

		conn.Write([]byte("+"))

		if len(targetDescriptionBinXML) > (int(dataOffset) + int(dataLength)) {
			conn.Write([]byte(formatGdbPacket("m" + string(targetDescriptionBinXML[dataOffset:dataOffset+dataLength]))))
		} else if len(targetDescriptionBinXML) > int(dataOffset) {
			conn.Write([]byte(formatGdbPacket("l" + string(targetDescriptionBinXML[dataOffset:]))))
		} else {
			break
		}
	case strings.Contains(request, "qXfer:threads:read::"):
		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket("l<?xml version=\"1.0\"?><threads></threads>")))
	case string(request[0]) == "g":
		conn.Write([]byte("+"))

		conn.Write([]byte(formatGdbPacket(readPsxRegisters(serialPort))))
	case string(request[0]) == "m":
		addr, size := parseGdbRequestArguments(request)

		if !memoryInValidRange(addr) {
			conn.Write([]byte("+"))
			conn.Write([]byte(formatGdbPacket("")))
			break
		}

		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket(readPsxMemory(serialPort, addr, size))))
	case string(request[0]) == "M":
		addr, size, data := parseMemoryWrite(request)
		writePsxMemory(serialPort, addr, size, data)
		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket("OK")))
	case string(request[0]) == "p":
		regNum := request[1:]

		conn.Write([]byte("+"))
		if regNum == "49" {
			conn.Write([]byte(formatGdbPacket("00000000")))
		}
		if regNum == "4a" {
			conn.Write([]byte(formatGdbPacket("00000000")))
		}

		if regNum == "26" {
			conn.Write([]byte("+"))
			conn.Write([]byte(formatGdbPacket(readSinglePsxReqister(serialPort, regNum))))
		}
		if regNum == "27" {
			conn.Write([]byte("+"))
			conn.Write([]byte(formatGdbPacket(readSinglePsxReqister(serialPort, regNum))))
		}
		if regNum == "28" {
			conn.Write([]byte("+"))
			conn.Write([]byte(formatGdbPacket(readSinglePsxReqister(serialPort, regNum))))
		}

	case string(request[0]) == "X":
		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket("")))
	case string(request[0]) == "P":
		regAddr := parseRegisterWrite(request)
		writeSinglePsxReqister(serialPort, regAddr)
		conn.Write([]byte("+"))
		acknowledgeWithOk(conn)
	case string(request[0]) == "c":
		serialPort.Write([]byte("c"))
		reader := bufio.NewReader(serialPort)

		_, err := reader.ReadString('%')
		if err != nil {
			panic(err)
		}

		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket("S05")))
	case string(request[0]) == "Z":
		bpAddr := parseBreakpointWrite(request)
		opcode := readPsxMemory(serialPort, bpAddr, "00000004")

		if _, ok := cachedOpcodes[bpAddr]; !ok {
			cachedOpcodes[bpAddr] = opcode
		}

		writePsxMemory(serialPort, bpAddr, "00000004", stringToPsxBytes("0000000d", true))

		writeBreakpointToPsx(serialPort, bpAddr)
		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket("OK")))
	case string(request[0]) == "z":
		bpAddr := parseBreakpointWrite(request)

		writePsxMemory(serialPort, bpAddr, "00000004", stringToPsxBytes(cachedOpcodes[bpAddr], false))

		clearBreakPointFromPsx(serialPort, bpAddr)
		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket("OK")))
	case string(request[0]) == "s":
		serialPort.Write([]byte("s"))
		reader := bufio.NewReader(serialPort)

		_, err := reader.ReadString('%')
		if err != nil {
			panic(err)
		}

		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket("S05")))
	case strings.Contains(request, "qSupported"):
		msg := "PacketSize=800;qXfer:features:read+"
		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket(msg)))
	case string(request[0]) == "?":
		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket("S00")))
	case strings.Contains(request, "qOffsets"):
		msg := "Text=0;Data=0;Bss=0"
		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket(msg)))
	case strings.Contains(request, "qAttached"):
		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket("1")))
	}
}

func handlePacket(conn net.Conn, serialPort serialport, cachedOpcodes map[string]string, packet string) {
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

	parseGdbRequest(conn, serialPort, cachedOpcodes, packet[startIndex+1:endIndex])
}

func handleGdbClient(conn net.Conn, serialPort serialport, cachedOpcodes map[string]string) {
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
			handlePacket(conn, serialPort, cachedOpcodes, request)
		}
	}
}

func main() {
	serialdevicePtr := flag.String("device", "", "Serial device, e.g. /dev/ttyUSB0")
	tcpPortPtr := flag.String("port", "8888", "GDB port")

	cachedOpcodes := make(map[string]string)

	flag.Parse()

	if len(*serialdevicePtr) == 0 {
		log.Fatal("Serial device/comport not specified, see -h for help")
	}

	fmt.Println("Device to PSX: ", *serialdevicePtr)
	fmt.Println("TCP port for GDB", *tcpPortPtr)

	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(*serialdevicePtr, mode)
	if err != nil {
		log.Fatal(err)
	}

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

		handleGdbClient(conn, port, cachedOpcodes)
	}

	defer port.Close()
}
