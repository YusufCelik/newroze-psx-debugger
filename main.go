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

var targetDescriptionXML string = `<?xml version="1.0"?>
<!DOCTYPE feature SYSTEM "gdb-target.dtd">
<target version="1.0">
<!-- Helping GDB -->
<architecture>mips:3000</architecture>
<osabi>none</osabi>
<!-- Mapping ought to be flexible, but there seems to be some
     hardcoded parts in gdb, so let's use the same mapping. -->
<feature name="org.gnu.gdb.mips.cpu">
  <reg name="r0" bitsize="32" regnum="0"/>
  <reg name="r1" bitsize="32" regnum="1"/>
  <reg name="r2" bitsize="32" regnum="2"/>
  <reg name="r3" bitsize="32" regnum="3"/>
  <reg name="r4" bitsize="32" regnum="4"/>
  <reg name="r5" bitsize="32" regnum="5"/>
  <reg name="r6" bitsize="32" regnum="6"/>
  <reg name="r7" bitsize="32" regnum="7"/>
  <reg name="r8" bitsize="32" regnum="8"/>
  <reg name="r9" bitsize="32" regnum="9"/>
  <reg name="r10" bitsize="32" regnum="10"/>
  <reg name="r11" bitsize="32" regnum="11"/>
  <reg name="r12" bitsize="32" regnum="12"/>
  <reg name="r13" bitsize="32" regnum="13"/>
  <reg name="r14" bitsize="32" regnum="14"/>
  <reg name="r15" bitsize="32" regnum="15"/>
  <reg name="r16" bitsize="32" regnum="16"/>
  <reg name="r17" bitsize="32" regnum="17"/>
  <reg name="r18" bitsize="32" regnum="18"/>
  <reg name="r19" bitsize="32" regnum="19"/>
  <reg name="r20" bitsize="32" regnum="20"/>
  <reg name="r21" bitsize="32" regnum="21"/>
  <reg name="r22" bitsize="32" regnum="22"/>
  <reg name="r23" bitsize="32" regnum="23"/>
  <reg name="r24" bitsize="32" regnum="24"/>
  <reg name="r25" bitsize="32" regnum="25"/>
  <reg name="r26" bitsize="32" regnum="26"/>
  <reg name="r27" bitsize="32" regnum="27"/>
  <reg name="r28" bitsize="32" regnum="28"/>
  <reg name="r29" bitsize="32" regnum="29"/>
  <reg name="r30" bitsize="32" regnum="30"/>
  <reg name="r31" bitsize="32" regnum="31"/>
  <reg name="lo" bitsize="32" regnum="33"/>
  <reg name="hi" bitsize="32" regnum="34"/>
  <reg name="pc" bitsize="32" regnum="37"/>
</feature>
<feature name="org.gnu.gdb.mips.cp0">
  <reg name="status" bitsize="32" regnum="32"/>
  <reg name="badvaddr" bitsize="32" regnum="35"/>
  <reg name="cause" bitsize="32" regnum="36"/>
  <reg name="dcic" bitsize="32" regnum="38"/>
  <reg name="bpc" bitsize="32" regnum="39"/>
  <reg name="tar" bitsize="32" regnum="40"/>
</feature>
<!-- We don't have an FPU, but gdb hardcodes one, and will choke
     if this section isn't present. -->
<feature name="org.gnu.gdb.mips.fpu">
  <reg name="f0" bitsize="32" type="ieee_single" regnum="41"/>
  <reg name="f1" bitsize="32" type="ieee_single"/>
  <reg name="f2" bitsize="32" type="ieee_single"/>
  <reg name="f3" bitsize="32" type="ieee_single"/>
  <reg name="f4" bitsize="32" type="ieee_single"/>
  <reg name="f5" bitsize="32" type="ieee_single"/>
  <reg name="f6" bitsize="32" type="ieee_single"/>
  <reg name="f7" bitsize="32" type="ieee_single"/>
  <reg name="f8" bitsize="32" type="ieee_single"/>
  <reg name="f9" bitsize="32" type="ieee_single"/>
  <reg name="f10" bitsize="32" type="ieee_single"/>
  <reg name="f11" bitsize="32" type="ieee_single"/>
  <reg name="f12" bitsize="32" type="ieee_single"/>
  <reg name="f13" bitsize="32" type="ieee_single"/>
  <reg name="f14" bitsize="32" type="ieee_single"/>
  <reg name="f15" bitsize="32" type="ieee_single"/>
  <reg name="f16" bitsize="32" type="ieee_single"/>
  <reg name="f17" bitsize="32" type="ieee_single"/>
  <reg name="f18" bitsize="32" type="ieee_single"/>
  <reg name="f19" bitsize="32" type="ieee_single"/>
  <reg name="f20" bitsize="32" type="ieee_single"/>
  <reg name="f21" bitsize="32" type="ieee_single"/>
  <reg name="f22" bitsize="32" type="ieee_single"/>
  <reg name="f23" bitsize="32" type="ieee_single"/>
  <reg name="f24" bitsize="32" type="ieee_single"/>
  <reg name="f25" bitsize="32" type="ieee_single"/>
  <reg name="f26" bitsize="32" type="ieee_single"/>
  <reg name="f27" bitsize="32" type="ieee_single"/>
  <reg name="f28" bitsize="32" type="ieee_single"/>
  <reg name="f29" bitsize="32" type="ieee_single"/>
  <reg name="f30" bitsize="32" type="ieee_single"/>
  <reg name="f31" bitsize="32" type="ieee_single"/>
  <reg name="fcsr" bitsize="32" group="float"/>
  <reg name="fir" bitsize="32" group="float"/>
</feature>
</target>`

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
	serialPort.Write([]byte(stringToPsxBytes(addr, true)))

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

func parseGdbRequest(conn net.Conn, serialPort serialport, request string) {
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

		targetDescriptionBinXML := []byte(targetDescriptionXML)

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
		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket("S05")))
	case string(request[0]) == "Z":
		bpAddr := parseBreakpointWrite(request)
		writeBreakpointToPsx(serialPort, bpAddr)
		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket("OK")))
	case string(request[0]) == "z":
		conn.Write([]byte("+"))
		conn.Write([]byte(formatGdbPacket("OK")))
	case string(request[0]) == "s":
		serialPort.Write([]byte("s"))
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

func handlePacket(conn net.Conn, serialPort serialport, packet string) {
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

func main() {
	serialdevicePtr := flag.String("device", "", "Serial device, e.g. /dev/ttyUSB0")
	tcpPortPtr := flag.String("port", "8888", "GDB port")

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

		handleGdbClient(conn, port)
	}

	defer port.Close()
}
