// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2019 Intel Corporation

package eaa

import "github.com/smart-edge-open/edgeservices/pkg/util"

// CertsInfo describes paths for certs used in configuration
type CertsInfo struct {
	CaRootKeyPath     string `json:"CaRootKeyPath"`
	CaRootPath        string `json:"CaRootPath"`
	ServerCertPath    string `json:"ServerCertPath"`
	ServerKeyPath     string `json:"ServerKeyPath"`
	CommonName        string `json:"CommonName"`
	KafkaCAPath       string `json:"KafkaCAPath"`
	KafkaUserCertPath string `json:"KafkaUserCertPath"`
	KafkaUserKeyPath  string `json:"KafkaUserKeyPath"`
}

// Config describes EAA JSON config file
type Config struct {
	TLSEndpoint        string        `json:"TlsEndpoint"`
	OpenEndpoint       string        `json:"OpenEndpoint"`
	ValidationEndpoint string        `json:"ValidationEndpoint"`
	HeartbeatInterval  util.Duration `json:"HeartbeatInterval"`
	Certs              CertsInfo     `json:"Certs"`
	KafkaBroker        string        `json:"KafkaBroker"`
}
