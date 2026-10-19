package airplay

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBundledRuntimeDiscovery(t *testing.T) {
	runtimeDir := os.Getenv("ARIADNE_TEST_UXPLAY_RUNTIME")
	ip := net.ParseIP(os.Getenv("ARIADNE_TEST_UXPLAY_IPV4")).To4()
	if runtimeDir == "" || ip == nil {
		t.Skip("set ARIADNE_TEST_UXPLAY_RUNTIME and ARIADNE_TEST_UXPLAY_IPV4 to test LAN discovery")
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: ip})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	s := newService(runtimeDir, filepath.Join(t.TempDir(), "投屏发现验证"))
	s.name = fmt.Sprintf("Ariadne 发现测试 %d", time.Now().UnixNano())
	t.Cleanup(func() { s.Stop() })
	if result := s.Start(); !result.OK {
		t.Fatalf("receiver failed: %+v", result)
	}
	query := make([]byte, 12)
	binary.BigEndian.PutUint16(query[4:], 1)
	for _, label := range []string{"_airplay", "_tcp", "local"} {
		query = append(query, byte(len(label)))
		query = append(query, label...)
	}
	// PTR query with the unicast-response bit, sent through the actual LAN.
	query = append(query, 0, 0, 12, 0x80, 1)
	target := &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}
	buffer := make([]byte, 9000)
	for attempt := 0; attempt < 10; attempt++ {
		if _, err := conn.WriteToUDP(query, target); err != nil {
			t.Fatal(err)
		}
		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		for {
			n, _, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
					break
				}
				t.Fatal(err)
			}
			if mdnsContainsReceiverAddress(buffer[:n], s.name, ip) {
				t.Logf("receiver advertised %q with LAN address %s", s.name, ip)
				return
			}
		}
	}
	text, _ := ReadLog(s.Status().LogPath)
	t.Fatalf("no mDNS response advertised this receiver with LAN address %s\n%s", ip, text)
}

func mdnsContainsReceiverAddress(packet []byte, receiver string, ip net.IP) bool {
	if len(packet) < 12 || packet[2]&0x80 == 0 || !bytes.Contains(packet, []byte(receiver)) {
		return false
	}
	skipName := func(offset int) int {
		for offset < len(packet) {
			size := int(packet[offset])
			offset++
			if size == 0 {
				return offset
			}
			if size&0xc0 == 0xc0 && offset < len(packet) {
				return offset + 1
			}
			if size > 63 || offset+size > len(packet) {
				return -1
			}
			offset += size
		}
		return -1
	}
	offset := 12
	for i := 0; i < int(binary.BigEndian.Uint16(packet[4:6])); i++ {
		offset = skipName(offset)
		if offset < 0 || offset+4 > len(packet) {
			return false
		}
		offset += 4
	}
	count := int(binary.BigEndian.Uint16(packet[6:8])) + int(binary.BigEndian.Uint16(packet[8:10])) + int(binary.BigEndian.Uint16(packet[10:12]))
	for i := 0; i < count; i++ {
		offset = skipName(offset)
		if offset < 0 || offset+10 > len(packet) {
			return false
		}
		recordType := binary.BigEndian.Uint16(packet[offset:])
		size := int(binary.BigEndian.Uint16(packet[offset+8:]))
		offset += 10
		if offset+size > len(packet) {
			return false
		}
		if recordType == 1 && size == net.IPv4len && bytes.Equal(packet[offset:offset+size], ip) {
			return true
		}
		offset += size
	}
	return false
}
