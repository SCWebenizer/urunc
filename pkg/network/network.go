// Copyright 2023 Nubificus LTD.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package network

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/jackpal/gateway"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// FIXME: Discover the veth endpoint name instead of using default "eth0". See: https://github.com/nubificus/urunc/issues/14
const DefaultInterface = "eth0"

const DefaultTap = "tapX_urunc"

type UnikernelNetworkInfo struct {
	TapDevice string
	EthDevice Interface
}
type Manager interface {
	NetworkSetup() (*UnikernelNetworkInfo, error)
}

func NewNetworkManager(networkType string) (Manager, error) {
	switch networkType {
	case "static":
		return &StaticNetwork{}, nil
	case "dynamic":
		return &DynamicNetwork{}, nil
	default:
		return nil, fmt.Errorf("network manager %s not supported", networkType)

	}
}

type Interface struct {
	IP             string
	DefaultGateway string
	Mask           string
	Interface      string
}

func getTapIndex() (int, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return 0, err
	}
	tapCount := 0
	for _, iface := range ifaces {
		if strings.Contains(iface.Name, "tap") {
			tapCount++
		}
	}
	return tapCount, nil
}

func createTapDevice(name string, mtu int, ownerUID, ownerGID int) (netlink.Link, error) {
	tapLinkAttrs := netlink.NewLinkAttrs()
	tapLinkAttrs.Name = name
	tapLink := &netlink.Tuntap{
		LinkAttrs: tapLinkAttrs,

		// We want a tap device (L2) as opposed to a tun (L3)
		Mode: netlink.TUNTAP_MODE_TAP,

		// Firecracker does not support multiqueue tap devices at this time:
		// https://github.com/firecracker-microvm/firecracker/issues/750
		Queues: 1,

		Flags: netlink.TUNTAP_ONE_QUEUE | // single queue tap device
			netlink.TUNTAP_VNET_HDR, // parse vnet headers added by the vm's virtio_net implementation
	}

	err := netlink.LinkAdd(tapLink)
	if err != nil {
		return nil, fmt.Errorf("failed to create tap device: %w", err)
	}

	for _, tapFd := range tapLink.Fds {
		err = unix.IoctlSetInt(int(tapFd.Fd()), unix.TUNSETOWNER, ownerUID)
		if err != nil {
			return nil, fmt.Errorf("failed to set tap %s owner to uid %d: %w", name, ownerUID, err)
		}

		err = unix.IoctlSetInt(int(tapFd.Fd()), unix.TUNSETGROUP, ownerGID)
		if err != nil {
			return nil, fmt.Errorf("failed to set tap %s group to gid %d: %w", name, ownerGID, err)
		}
	}

	err = netlink.LinkSetMTU(tapLink, mtu)
	if err != nil {
		return nil, fmt.Errorf("failed to set tap device MTU to %d: %w", mtu, err)
	}

	return tapLink, nil
}

// ensureEth0Exists checks all network interface in current netns and returns
// nil if eth0 is present or ErrEth0NotFound if not
func ensureEth0Exists() error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	for _, iface := range ifaces {
		if iface.Name == DefaultInterface {
			return nil
		}
	}
	return errors.New("eth0 device not found")
}

func defaultInterfaceInfo() (Interface, error) {
	ief, err := net.InterfaceByName(DefaultInterface)
	if err != nil {
		return Interface{}, err
	}
	addrs, err := ief.Addrs()
	if err != nil {
		return Interface{}, err
	}
	ipAddress := ""
	mask := ""
	netMask := net.IPMask{}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			ipAddress = ipNet.IP.String()
			// hexadecimal notation
			mask = ipNet.Mask.String()
			netMask = ipNet.Mask
			break
		}
	}
	if mask == "" {
		return Interface{}, fmt.Errorf("failed to find mask for %q", DefaultInterface)
	}
	// convert to decimal notation
	decimalParts := make([]string, len(netMask))
	for i, part := range netMask {
		decimalParts[i] = fmt.Sprintf("%d", part)
	}
	mask = strings.Join(decimalParts, ".")
	if ipAddress == "" {
		return Interface{}, fmt.Errorf("failed to find IPv4 address for %q", DefaultInterface)
	}
	gateway, err := gateway.DiscoverGateway()
	if err != nil {
		return Interface{}, err
	}
	return Interface{
		IP:             ipAddress,
		DefaultGateway: gateway.String(),
		Mask:           mask,
		Interface:      DefaultInterface,
	}, nil
}

func addIngressQdisc(link netlink.Link) error {
	ingress := &netlink.Ingress{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    netlink.HANDLE_INGRESS,
		},
	}
	return netlink.QdiscAdd((ingress))
}

func addRedirectFilter(source netlink.Link, target netlink.Link) error {
	return netlink.FilterAdd(&netlink.U32{
		FilterAttrs: netlink.FilterAttrs{
			LinkIndex: source.Attrs().Index,
			Parent:    netlink.MakeHandle(0xffff, 0),
			Protocol:  unix.ETH_P_ALL,
		},
		Actions: []netlink.Action{
			&netlink.MirredAction{
				ActionAttrs: netlink.ActionAttrs{
					Action: netlink.TC_ACT_STOLEN,
				},
				MirredAction: netlink.TCA_EGRESS_REDIR,
				Ifindex:      target.Attrs().Index,
			},
		},
	})
}
