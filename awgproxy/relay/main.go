// relay bypasses macOS socket-layer VPN filters (e.g. Cisco AnyConnect's
// NEFilterDataProvider) by relaying UDP through /dev/bpf via libpcap.
// Invoked by `awgproxy <config> --bypass`, not meant to be run standalone.
//
// Usage: sudo relay <listen_port> <dest_ip> <dest_port> <iface>
package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

const srcPort = 55333

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[relay] FATAL: "+format+"\n", args...)
	os.Exit(1)
}

func ownAddrs(ifaceName string) (net.IP, net.HardwareAddr, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, nil, err
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if ip4 := ipnet.IP.To4(); ip4 != nil {
			return ip4, iface.HardwareAddr, nil
		}
	}
	return nil, nil, fmt.Errorf("no IPv4 address on %s", ifaceName)
}

func gatewayIP(ifaceName string) (string, error) {
	out, err := exec.Command("netstat", "-rn", "-f", "inet").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[0] == "default" && fields[len(fields)-1] == ifaceName {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("no default gateway found for %s", ifaceName)
}

func gatewayMAC(ip string) (net.HardwareAddr, error) {
	_ = exec.Command("ping", "-c", "1", "-t", "1", ip).Run()
	out, err := exec.Command("arp", "-n", ip).Output()
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`([0-9a-fA-F]{1,2}(?::[0-9a-fA-F]{1,2}){5})`)
	m := re.FindString(string(out))
	if m == "" {
		return nil, fmt.Errorf("ARP failed for %s", ip)
	}
	return net.ParseMAC(m)
}

func buildFrame(srcMAC, dstMAC net.HardwareAddr, srcIP, dstIP net.IP, srcPort, dstPort int, payload []byte) ([]byte, error) {
	eth := layers.Ethernet{
		SrcMAC:       srcMAC,
		DstMAC:       dstMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := layers.IPv4{
		Version:  4,
		TTL:      64,
		Id:       0,
		Protocol: layers.IPProtocolUDP,
		SrcIP:    srcIP,
		DstIP:    dstIP,
	}
	udp := layers.UDP{
		SrcPort: layers.UDPPort(srcPort),
		DstPort: layers.UDPPort(dstPort),
	}
	if err := udp.SetNetworkLayerForChecksum(&ip); err != nil {
		return nil, err
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if err := gopacket.SerializeLayers(buf, opts, &eth, &ip, &udp, gopacket.Payload(payload)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "Usage: relay <listen_port> <dest_ip> <dest_port> <iface>")
		os.Exit(1)
	}
	listenPort, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fatalf("bad listen_port: %v", err)
	}
	destIP := os.Args[2]
	destPort, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fatalf("bad dest_port: %v", err)
	}
	ifaceName := os.Args[4]

	srcIP, srcMAC, err := ownAddrs(ifaceName)
	if err != nil {
		fatalf("cannot get address for %s: %v", ifaceName, err)
	}

	gwIP, err := gatewayIP(ifaceName)
	if err != nil {
		fatalf("%v", err)
	}
	gwMAC, err := gatewayMAC(gwIP)
	if err != nil {
		fatalf("%v", err)
	}

	inactive, err := pcap.NewInactiveHandle(ifaceName)
	if err != nil {
		fatalf("pcap setup failed: %v", err)
	}
	defer inactive.CleanUp()
	_ = inactive.SetSnapLen(65536)
	_ = inactive.SetPromisc(false)
	_ = inactive.SetImmediateMode(true)
	_ = inactive.SetTimeout(50 * time.Millisecond)

	handle, err := inactive.Activate()
	if err != nil {
		fatalf("pcap activate failed: %v", err)
	}
	defer handle.Close()

	filter := fmt.Sprintf("udp and src host %s and src port %d and dst port %d", destIP, destPort, srcPort)
	if err := handle.SetBPFFilter(filter); err != nil {
		fatalf("BPF filter failed: %v", err)
	}

	local, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: listenPort})
	if err != nil {
		fatalf("local socket failed: %v", err)
	}
	defer local.Close()

	dstIP := net.ParseIP(destIP).To4()

	var mu sync.Mutex
	var client *net.UDPAddr

	go func() {
		buf := make([]byte, 65535)
		for {
			n, addr, err := local.ReadFromUDP(buf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[relay] fwd ERROR: %v\n", err)
				return
			}
			mu.Lock()
			client = addr
			mu.Unlock()

			frame, err := buildFrame(srcMAC, gwMAC, srcIP, dstIP, srcPort, destPort, buf[:n])
			if err != nil {
				fmt.Fprintf(os.Stderr, "[relay] build frame: %v\n", err)
				continue
			}
			if err := handle.WritePacketData(frame); err != nil {
				fmt.Fprintf(os.Stderr, "[relay] write: %v\n", err)
			}
		}
	}()

	go func() {
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		for packet := range packetSource.Packets() {
			udpLayer := packet.Layer(layers.LayerTypeUDP)
			if udpLayer == nil {
				continue
			}
			udp, _ := udpLayer.(*layers.UDP)

			mu.Lock()
			dst := client
			mu.Unlock()
			if dst == nil {
				continue
			}
			if _, err := local.WriteToUDP(udp.Payload, dst); err != nil {
				fmt.Fprintf(os.Stderr, "[relay] rev write: %v\n", err)
			}
		}
	}()

	fmt.Fprintln(os.Stderr, "[relay] ready")
	select {}
}
