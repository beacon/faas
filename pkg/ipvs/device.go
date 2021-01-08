package ipvs

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"

	"github.com/beacon/faas/pkg/utils/sets"
	"github.com/vishvananda/netlink"
)

// Device abstract netlink
type Device interface {
	EnsureDevice() error

	EnsureAddressBind(address string) (exist bool, err error)
	UnbindAddress(address string) error
	ListBindAddress() (sets.String, error)
}

// DummyDeviceName device for our ipvs
const DummyDeviceName = "beacon-ipvs"

type dummyDevice struct {
	name          string
	masterDevName string // outgoing device
}

// NewDevice create new device for ipvs bridge
func NewDevice(name, masterName string) Device {
	return &dummyDevice{
		name:          name,
		masterDevName: masterName,
	}
}

func (d *dummyDevice) EnsureDevice() error {
	_, err := netlink.LinkByName(d.name)
	if err == nil {
		// Already exists
		return nil
	}
	dev := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: d.name}}
	if err := netlink.LinkAdd(dev); err != nil {
		return fmt.Errorf("failed to add link %s:%v", d.name, err)
	}
	return nil
	// masterDev, err := netlink.LinkByName(d.masterDevName)
	// if err != nil {
	// 	return fmt.Errorf("failed to get master device by name %s:%v", d.masterDevName, err)
	// }

	// return netlink.LinkSetMaster(masterDev, dev)
}

// EnsureAddressBind checks if address is bound to the interface and, if not, binds it. If the address is already bound, return true.
func (d *dummyDevice) EnsureAddressBind(address string) (exist bool, err error) {
	dev, err := netlink.LinkByName(d.name)
	if err != nil {
		return false, fmt.Errorf("error get interface: %s, err: %v", d.name, err)
	}
	addr := net.ParseIP(address)
	if addr == nil {
		return false, fmt.Errorf("error parse ip address: %s", address)
	}
	if err := netlink.AddrAdd(dev, &netlink.Addr{IPNet: netlink.NewIPNet(addr)}); err != nil {
		// "EEXIST" will be returned if the address is already bound to device
		if err == unix.EEXIST {
			return true, nil
		}
		return false, fmt.Errorf("error bind address: %s to interface: %s, err: %v", address, d.name, err)
	}
	return false, nil
}

// UnbindAddress makes sure IP address is unbound from the network interface.
func (d *dummyDevice) UnbindAddress(address string) error {
	dev, err := netlink.LinkByName(d.name)
	if err != nil {
		return fmt.Errorf("error get interface: %s, err: %v", d.name, err)
	}
	addr := net.ParseIP(address)
	if addr == nil {
		return fmt.Errorf("error parse ip address: %s", address)
	}
	if err := netlink.AddrDel(dev, &netlink.Addr{IPNet: netlink.NewIPNet(addr)}); err != nil {
		if err != unix.ENXIO {
			return fmt.Errorf("error unbind address: %s from interface: %s, err: %v", address, d.name, err)
		}
	}
	return nil
}

func (d *dummyDevice) ListBindAddress() (sets.String, error) {
	dev, err := netlink.LinkByName(d.name)
	if err != nil {
		return nil, fmt.Errorf("error get interface: %s, err: %v", d.name, err)
	}
	addrs, err := netlink.AddrList(dev, 0)
	if err != nil {
		return nil, fmt.Errorf("error list bound address of interface: %s, err: %v", d.name, err)
	}
	ips := make(sets.String)
	for _, addr := range addrs {
		ips.Add(addr.IP.String())
	}
	return ips, nil
}
