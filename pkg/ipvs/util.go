package ipvs

import (
	"net"

	extensionv1 "github.com/beacon/faas/api/v1"
)

// Layer 4 protocols
const (
	ProtocolTCP = "tcp"
	ProtocolUDP = "udp"
)

// ServiceToVirtualServers convert service to virtual server, also for index purpose
func ServiceToVirtualServers(svc *extensionv1.Service) []*VirtualServer {
	ip := net.ParseIP(svc.Spec.VirtualIP)
	vss := make([]*VirtualServer, len(svc.Spec.Ports))
	protocol := svc.Spec.Protocol
	if protocol == "" {
		protocol = ProtocolTCP
	}
	for i, svcPort := range svc.Spec.Ports {
		vss[i] = &VirtualServer{
			Address:   ip,
			Protocol:  protocol,
			Port:      uint16(svcPort.Port),
			Scheduler: svc.Spec.Scheduler,
		}
	}
	return vss
}
