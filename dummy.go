package main

import (
	"flag"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/beacon/faas/pkg/ipvs"
	"k8s.io/utils/exec"
)

// Options for program
type Options struct {
	MasterDevice string
	IP           string
	Port         int

	Backends string
}

func main() {
	var opts Options
	flag.StringVar(&opts.MasterDevice, "masterDevice", "eth0", "Master device name")
	flag.StringVar(&opts.IP, "ip", "", "IP address to bind")
	flag.IntVar(&opts.Port, "port", 80, "Port of service")
	flag.StringVar(&opts.Backends, "backends", "", "Backends to")
	flag.Parse()
	dev := ipvs.NewDevice(ipvs.DummyDeviceName, opts.MasterDevice)
	if err := dev.EnsureDevice(); err != nil {
		log.Fatalln("failed to ensure device:", err)
	}
	addrs, err := dev.ListBindAddress()
	if err != nil {
		log.Fatalln("failed to list addrs:", err)
	}

	exists, err := dev.EnsureAddressBind(opts.IP)
	if exists {
		log.Println("IP already bound with device")
	} else if err != nil {
		log.Fatalln("failed to bind address:", err)
	}
	execer := exec.New()
	ipvsInterface := ipvs.New(execer)

	targetVS := &ipvs.VirtualServer{
		Address:   net.ParseIP(opts.IP),
		Protocol:  "tcp",
		Port:      uint16(opts.Port),
		Scheduler: "wrr",
	}
	vs, err := ipvsInterface.GetVirtualServer(targetVS)
	if err != nil {
		log.Println("failed to get virtual server:", err, "try to add one")
		if err := ipvsInterface.AddVirtualServer(targetVS); err != nil {
			log.Fatalln("failed to add virtual server:", err)
		}
		vs = targetVS
	}
	backends := strings.Split(opts.Backends, ",")
	realServers := make([]*ipvs.RealServer, len(backends))

	prevServers, err := ipvsInterface.GetRealServers(vs)
	if err != nil {
		log.Fatalln("failed to get previous real servers:", err)
	}
	for _, rs := range prevServers {
		if err := ipvsInterface.DeleteRealServer(vs, rs); err != nil {
			log.Fatalln("failed to delete real servers:", err)
		}
	}

	for i, s := range backends {
		p := strings.Split(s, ":")
		ip := net.ParseIP(p[0])
		port, err := strconv.Atoi(p[1])
		if err != nil {
			log.Fatalf("invalid port when parsing backend %s:%v", s, err)
		}
		realServers[i] = &ipvs.RealServer{
			Address: ip,
			Port:    uint16(port),
			Weight:  50,
		}
		if err := ipvsInterface.AddRealServer(vs, realServers[i]); err != nil {
			log.Fatalf("failed to add real server %s:%v", s, err)
		}
	}

	for addr := range addrs {
		log.Println("Bind address:", addr)
	}
}
