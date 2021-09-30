// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2019 Intel Corporation

package interfaceservice

import (
	"strings"

	pb "github.com/smart-edge-open/edgeservices/pkg/interfaceservice/pb"
	"github.com/pkg/errors"
)

// attachPortToOvs attaches given port to kube-ovn's bridge
func detachPortFromOvs(port pb.Port) error {
	name, err := getPortName(port.Pci)

	if err != nil {
		return err
	}

	output, err := Vsctl("ovs-vsctl", "del-port", strings.TrimSpace(name))
	if err == nil {
		log.Info("Removed OVS port: ", name)
	} else {
		log.Info("removing port: " + string(output))
		return errors.Wrapf(err, string(output))
	}

	return err
}
