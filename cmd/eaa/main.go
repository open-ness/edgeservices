// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2019 Intel Corporation

package main

import (
	"os"

	// Imports required to run agent
	"github.com/smart-edge-open/edgeservices/pkg/eaa"
	"github.com/smart-edge-open/edgeservices/pkg/service"
)

// EdgeServices array contains function pointers to services start functions
var EdgeServices = []service.StartFunction{eaa.Run}

func main() {

	if !service.RunServices(EdgeServices) {
		os.Exit(1)
	}

	service.Log.Infof("Service stopped gracefully")
}
