// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2019 Intel Corporation

package interfaceservice

import (
	"context"
	"os/exec"

	"github.com/pkg/errors"

	"github.com/golang/protobuf/ptypes/empty"
	pb "github.com/smart-edge-open/edgeservices/pkg/interfaceservice/pb"
)

var (
	// KernelNetworkDevicesProvider stores function providing
	// functionality to get network interfaces
	KernelNetworkDevicesProvider = getKernelNetworkDevices

	// Vsctl stores function which executes ovs-vsctl command with given args
	Vsctl = vsctl
	// Devbind stores function which executes dpdk-devbind.py command with given args
	Devbind = devbind

	// ReattachDpdkPorts function pointer called before server is up and running
	ReattachDpdkPorts = reattachDpdkPorts
)

// InterfaceService provides service for managing physical
// network interfaces in kube-ovn mode.
// It exposes Get method which provides information about interfaces.
// It also exposes Attach and Detach methods which can be used
// to configure those interfaces.
type InterfaceService struct{}

// Get fetches all ports from the server.
func (*InterfaceService) Get(ctx context.Context,
	e *empty.Empty) (*pb.Ports, error) {
	log.Info("InterfaceService Get: received request")

	updateDPDKDevbindOutput()

	ports, strPorts, err := getPorts()
	if err != nil {
		log.Errf("Failed to get ports %s", err.Error())
		return nil, errors.Wrapf(err, "Failed to get ports %s", err.Error())
	}

	for _, s := range strPorts {
		log.Info(s)
	}

	log.Info("InterfaceService Get: sending a response with success")

	return &pb.Ports{Ports: ports}, err
}

// Attach triggers operation of attaching an interface to provided bridge.
// It requires full definition of Ports.
func (*InterfaceService) Attach(ctx context.Context,
	ports *pb.Ports) (*empty.Empty, error) {
	log.Info("InterfaceService Attach: received request")

	updateDPDKDevbindOutput()

	for _, port := range ports.Ports {
		if err := validatePort(*port); err != nil {
			log.Errf("Port validation failed: %s", err.Error())
			return &empty.Empty{}, err
		}
		if err := attachPortToOvs(*port); err != nil {
			log.Errf("Attaching port failed: %s", err.Error())
			return &empty.Empty{}, err
		}
	}

	log.Info("InterfaceService Attach: sending a response with success")

	return &empty.Empty{}, nil
}

// Detach removes a port from a bridge. It requires PCI only.
func (*InterfaceService) Detach(ctx context.Context,
	ports *pb.Ports) (*empty.Empty, error) {
	log.Info("InterfaceService Detach: received request")

	updateDPDKDevbindOutput()

	for _, port := range ports.Ports {
		if err := validatePort(*port); err != nil {
			log.Errf("Port validation failed: %s", err.Error())
			return &empty.Empty{}, err
		}
		if err := detachPortFromOvs(*port); err != nil {
			log.Errf("Detach port from Ovs failed: %s", err.Error())
			return &empty.Empty{}, err
		}
		if err := bindDriver(*port); err != nil {
			log.Errf("Bind driver failed: %s", err.Error())
			return &empty.Empty{}, err
		}
	}

	log.Info("InterfaceService Detach: sending a response with success")

	return &empty.Empty{}, nil
}

// vsctl executes ovs-vsctl with given args, it returns combined output
func vsctl(args ...string) ([]byte, error) {
	// #nosec G204 - params are hardcoded
	return exec.Command("sudo", args...).
		CombinedOutput()
}

// devbind executes dpdk-devbind.py with given args, it returns combined output
func devbind(args ...string) ([]byte, error) {
	// #nosec G204 - params are hardcoded
	return exec.Command("sudo", args...).
		CombinedOutput()
}
