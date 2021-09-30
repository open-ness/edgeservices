// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2019 Intel Corporation

package eaa_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"math/big"
	mrand "math/rand"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"

	"github.com/gorilla/websocket"
	"github.com/smart-edge-open/edgeservices/pkg/eaa"
	evapb "github.com/smart-edge-open/edgeservices/pkg/eva/internal_pb"

	"github.com/smart-edge-open/edgeservices/common/log"
)

// EaaCommonName Common Name that EAA uses for TLS connection
const (
	EaaCommonName = "eaa.openness"
	TestCertsDir  = "testdata/certs/"
)

// To pass configuration file path use ginkgo pass-through argument
// ginkgo -r -v -- -cfg=myconfig.json
var cfgPath string

func init() {
	flag.StringVar(&cfgPath, "cfg", "", "EAA TestSuite configuration file path")
}

func TestEaa(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Eaa Suite")
}

// MsgBrokerBackend configures which Message Broker backend will be used
type MsgBrokerBackend struct {
	Type string `json:"Type"`
	URL  string `json:"URL,omitempty"`
}

// Available Message Broker backends
const (
	KafkaBackend      = "kafka"
	GochannelsBackend = "gochannels"
)

type EAATestSuiteConfig struct {
	Dir                 string           `json:"Dir"`
	TLSEndpoint         string           `json:"TlsEndpoint"`
	ValidationEndpoint  string           `json:"ValidationEndpoint"`
	ApplianceTimeoutSec int              `json:"Timeout"`
	MsgBrokerBackend    MsgBrokerBackend `json:"MsgBrokerBackend"`
}

// test suite config with default values
var cfg = EAATestSuiteConfig{"../../", "localhost:48080",
	"localhost:42555", 2, MsgBrokerBackend{GochannelsBackend, ""}}

func readConfig(path string) {
	if path != "" {
		By("Configuring EAA test suite with: " + path)
		cfgData, err := ioutil.ReadFile(path)
		if err != nil {
			Fail("Failed to read suite configuration file!")
		}
		err = json.Unmarshal(cfgData, &cfg)
		if err != nil {
			Fail("Failed to unmarshal suite configuration file!")
		}
	}
}

func GetCertTempl() x509.Certificate {
	src := mrand.NewSource(time.Now().UnixNano())
	sn := big.NewInt(int64(mrand.New(src).Uint64()))

	certTempl := x509.Certificate{
		SerialNumber: sn,
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now().Add(-1 * time.Second),
		NotAfter:              time.Now().Add(1 * time.Minute),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		BasicConstraintsValid: true,
	}

	return certTempl
}

// Function is not used anymore, but can be used as helper:
// func generateCerts() {

// 	By("Generating certs")
// 	err := os.MkdirAll(tempdir+"/certs/eaa", 0755)
// 	Expect(err).ToNot(HaveOccurred(), "Error when creating temp directory")

// 	cmd := exec.Command("openssl", "req", "-x509", "-nodes", "-newkey",
// 		"rsa:2048", "-keyout", "server.key", "-out", "server.crt", "-days",
// 		"3650", "-subj", "/C=TT/ST=Test/L=Test/O=Test/OU=Test/CN=localhost")

// 	cmd.Dir = tempdir + "/certs/eaa"
// 	err = cmd.Run()
// 	Expect(err).ToNot(HaveOccurred(), "Error when generating .key .crt")

// 	cmd = exec.Command("openssl", "x509", "-in", "server.crt", "-out",
// 		"rootCA.pem", "-outform", "PEM")

// 	cmd.Dir = tempdir + "/certs/eaa"
// 	err = cmd.Run()
// 	Expect(err).ToNot(HaveOccurred(), "Error when converting .crt to .pem")

// 	cmd = exec.Command("openssl", "req", "-new", "-key", "server.key",
// 		"-out", "server.csr", "-subj",
// 		"/C=TT/ST=Test/L=Test/O=Test/OU=Test/CN=localhost")

// 	cmd.Dir = tempdir + "/certs/eaa"
// 	err = cmd.Run()
// 	Expect(err).ToNot(HaveOccurred(), "Error when generating .csr")
// }

type FakeIPAppLookupServiceServerImpl struct{}

var responseFromEva = "testapp"

func (*FakeIPAppLookupServiceServerImpl) GetApplicationByIP(
	ctx context.Context,
	ipAppLookupInfo *evapb.IPApplicationLookupInfo) (
	*evapb.IPApplicationLookupResult, error) {

	log.Info("FakeIPAppLookupServiceServerImpl GetApplicationByIP for: " +
		ipAppLookupInfo.GetIpAddress())

	var result evapb.IPApplicationLookupResult
	result.AppID = responseFromEva
	return &result, nil
}

func fakeAppidProvider() error {

	lApp, err := net.Listen("tcp", cfg.ValidationEndpoint)
	if err != nil {
		log.Errf("net.Listen error: %+v", err)
		return err
	}

	serverApp := grpc.NewServer()
	ipAppLookupService := FakeIPAppLookupServiceServerImpl{}
	evapb.RegisterIPApplicationLookupServiceServer(serverApp,
		&ipAppLookupService)

	go func() {
		log.Infof("Fake internal serving on %s", cfg.ValidationEndpoint)
		err = serverApp.Serve(lApp)
		if err != nil {
			log.Errf("Failed grpcServe(): %v", err)
			return
		}
	}()

	return err
}

func copyFile(src string, dst string) {
	srcFile, err := os.Open(src)
	Expect(err).ToNot(HaveOccurred(), "Copy file - error when opening "+src)
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	Expect(err).ToNot(HaveOccurred(), "Copy file - error when creating "+dst)
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	Expect(err).ToNot(HaveOccurred(), "Copy file - error when copying "+src+
		" to "+dst)

}

func copyFileWithMode(src string, dst string, fm os.FileMode) {
	copyFile(src, dst)

	err := os.Chmod(dst, fm)
	Expect(err).ToNot(HaveOccurred(), "Error when changing file mode"+dst)
}

func copyCerts(srcPath string) {
	if _, err := os.Stat(tempdir + "/certs/eaa"); os.IsNotExist(err) {
		err := os.MkdirAll(tempdir+"/certs/eaa", 0755)
		Expect(err).ToNot(HaveOccurred(), "Error when creating temp directory")
	}

	certFilePerm := os.FileMode(0644)
	certFiles := []string{"rootCA.pem", "server.pem"}
	for _, f := range certFiles {
		copyFileWithMode(srcPath+f, tempdir+"/certs/eaa/"+f, certFilePerm)
	}

	keyFilePerm := os.FileMode(0600)
	keyFiles := []string{"rootCA.key", "server.key"}
	for _, f := range keyFiles {
		copyFileWithMode(srcPath+f, tempdir+"/certs/eaa/"+f, keyFilePerm)
	}
}

func filesEqual(src string, dst string) bool {
	srcFile, err := ioutil.ReadFile(src)
	Expect(err).ToNot(HaveOccurred(), "Compare file - error reading file "+src)

	dstFile, err := ioutil.ReadFile(dst)
	Expect(err).ToNot(HaveOccurred(), "Compare file - error reading file "+dst)

	return bytes.Equal(srcFile, dstFile)
}

func compareCerts(srcPath string) {
	certFiles := []string{"rootCA.key", "rootCA.pem", "server.key", "server.pem"}
	for _, f := range certFiles {
		r := filesEqual(srcPath+f, tempdir+"/certs/eaa/"+f)
		Expect(r).To(BeTrue(), "Compare certs - compared files are different "+f)
	}
}

func generateConfigs() {
	By("Generating configuration files")
	_ = os.MkdirAll(tempdir+"/configs", 0755)

	files, err := ioutil.ReadDir(cfg.Dir + "/configs/")
	Expect(err).ToNot(HaveOccurred(), "Error when reading configs directory")
	for _, f := range files {
		if f.Name() != "eaa.json" {
			copyFile(cfg.Dir+"/configs/"+f.Name(), tempdir+
				"/configs/"+f.Name())
		}
	}

	var kafkaBrokerURL string
	if cfg.MsgBrokerBackend.Type == KafkaBackend {
		kafkaBrokerURL = cfg.MsgBrokerBackend.URL
	} else {
		kafkaBrokerURL = ""
	}

	// custom config for EAA
	eaaCfg := []byte(`{
		"TlsEndpoint": "` + cfg.TLSEndpoint + `",
		"ValidationEndpoint": "` + cfg.ValidationEndpoint + `",
		"Certs": {
			"CaRootKeyPath": "` + tempConfCaRootKeyPath + `",
			"CaRootPath": "` + tempConfCaRootPath + `",
			"ServerCertPath": "` + tempConfServerCertPath + `",
			"ServerKeyPath": "` + tempConfServerKeyPath + `",
			"CommonName": "` + EaaCommonName + `"
		},
		"KafkaBroker": "` + kafkaBrokerURL + `"
	}`)

	err = ioutil.WriteFile(tempdir+"/configs/eaa.json", eaaCfg, 0644)
	Expect(err).ToNot(HaveOccurred(), "Error when creating eaa.json")
}

var (
	srvCtx    context.Context
	srvCancel context.CancelFunc
	eaaErr    error
)

func runEaa(stopIndication chan bool) error {

	By("Starting appliance")

	srvCtx, srvCancel = context.WithCancel(context.Background())
	_ = srvCancel
	eaaRunSuccess := make(chan bool)
	go func() {
		var eaaCtx eaa.Context
		err := eaa.InitEaaContext(tempdir+"/configs/eaa.json", &eaaCtx)
		if err != nil {
			log.Errf("InitEaaContext() exited with error: %#v", err)
			goto fail
		}

		switch cfg.MsgBrokerBackend.Type {
		case KafkaBackend:
			eaaCtx.MsgBrokerCtx, err = eaa.NewKafkaMsgBroker(&eaaCtx, "eaa_test_consumer", nil)
			if err != nil {
				log.Errf("Failed to create a Kafka Message Broker: %#v", err)
				goto fail
			}
		case GochannelsBackend:
			eaaCtx.MsgBrokerCtx = eaa.NewGoChannelMsgBroker(&eaaCtx)
		default:
			log.Errf("Unnown Message Broker Type: %v", cfg.MsgBrokerBackend.Type)
			goto fail
		}

		eaaErr = eaa.RunServer(srvCtx, &eaaCtx)
		if eaaErr != nil {
			log.Errf("RunServer() exited with error: %#v", eaaErr)
		}
		stopIndication <- true
		return

	fail:
		eaaRunSuccess <- false
		stopIndication <- true
	}()

	// Wait until appliance is ready before running any tests specs
	return waitForEAA(eaaRunSuccess)
}

func runClassicEaa(stopIndication chan bool, path string) error {

	By("Starting classic EAA")

	srvCtx, srvCancel = context.WithCancel(context.Background())
	_ = srvCancel
	eaaRunSuccess := make(chan bool)
	go func() {
		var eaaCtx eaa.Context
		err := eaa.InitEaaContext(path, &eaaCtx)
		if err != nil {
			log.Errf("InitEaaContext() exited with error: %#v", err)
			goto fail
		}
		eaaErr = eaa.Run(srvCtx, path)
		if eaaErr != nil {
			log.Errf("Run() exited with error: %#v", eaaErr)
			goto fail
		}
		stopIndication <- true
		return

	fail:
		eaaRunSuccess <- false
		stopIndication <- true
	}()

	// Wait until appliance is ready before running any tests specs
	return waitForEAA(eaaRunSuccess)
}

func waitForEAA(eaaRunSuccess chan bool) error {
	go func() {
		accessCertTempl := GetCertTempl()
		accessCertTempl.Subject.CommonName = AccessName
		accessCert, accessCertPool := generateSignedClientCert(
			&accessCertTempl)

		client := createHTTPClient(accessCert, accessCertPool)
		for {
			_, err := client.Get("https://" + cfg.TLSEndpoint)
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		eaaRunSuccess <- true
	}()

	select {
	case ok := <-eaaRunSuccess:
		if ok {
			By("Appliance ready")
			return nil
		}
		return errors.New("starting appliance - run fail")
	case <-time.After(time.Duration(cfg.ApplianceTimeoutSec) * time.Second):
		return errors.New("starting appliance - timeout")
	}
}

func stopEaa(stopIndication chan bool) {
	By("Stopping appliance")
	srvCancel()
	<-stopIndication
	Expect(eaaErr).ShouldNot(HaveOccurred())
}

func GenerateTLSCert(cTempl, cParent *x509.Certificate, pub,
	prv interface{}) tls.Certificate {

	sClientCertDER, err := x509.CreateCertificate(rand.Reader,
		cTempl, cParent, pub.(*ecdsa.PrivateKey).Public(), prv)
	Expect(err).ShouldNot(HaveOccurred())

	sClientCert, err := x509.ParseCertificate(sClientCertDER)
	Expect(err).ShouldNot(HaveOccurred())

	sClientCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: sClientCert.Raw,
	})
	derKey, err := x509.MarshalECPrivateKey(pub.(*ecdsa.PrivateKey))
	Expect(err).ShouldNot(HaveOccurred())

	clientKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: derKey,
	})

	clientTLSCert, err := tls.X509KeyPair(sClientCertPEM,
		clientKeyPEM)
	Expect(err).ShouldNot(HaveOccurred())

	return clientTLSCert
}

func generateSignedClientCert(certTempl *x509.Certificate) (tls.Certificate,
	*x509.CertPool) {
	src := mrand.NewSource(time.Now().UnixNano())
	sn := big.NewInt(int64(mrand.New(src).Uint64()))
	certTempl.SerialNumber = sn

	// create cert pool with rootCA
	certPool := x509.NewCertPool()
	c, err := ioutil.ReadFile(tempConfCaRootPath)
	Expect(err).ShouldNot(HaveOccurred())
	ok := certPool.AppendCertsFromPEM(c)
	Expect(ok).To(BeTrue())

	// generate key for client
	clientPriv, err := ecdsa.GenerateKey(elliptic.P256(),
		rand.Reader)
	Expect(err).ShouldNot(HaveOccurred())

	// use root key generated by EAA
	serverPriv, err := tls.LoadX509KeyPair(
		tempConfCaRootPath,
		tempConfCaRootKeyPath)
	Expect(err).ShouldNot(HaveOccurred())

	prvCert, err := x509.ParseCertificate(
		serverPriv.Certificate[0])
	Expect(err).ShouldNot(HaveOccurred())

	clientTLSCert := GenerateTLSCert(certTempl, prvCert,
		clientPriv, serverPriv.PrivateKey)

	return clientTLSCert, certPool
}

func createHTTPClient(clientTLSCert tls.Certificate,
	certPool *x509.CertPool) *http.Client {
	// create client with certificate signed above
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      certPool,
				Certificates: []tls.Certificate{clientTLSCert},
				ServerName:   EaaCommonName,
			},
		}}

	return client
}

func createWebSocDialer(clientTLSCert tls.Certificate,
	certPool *x509.CertPool) *websocket.Dialer {
	// create socket with certificate signed above
	socket := &websocket.Dialer{
		TLSClientConfig: &tls.Config{
			RootCAs:      certPool,
			Certificates: []tls.Certificate{clientTLSCert},
			ServerName:   EaaCommonName,
		},
	}

	return socket
}

var (
	tempdir                string
	tempConfCaRootKeyPath  string
	tempConfCaRootPath     string
	tempConfServerCertPath string
	tempConfServerKeyPath  string
)

var _ = BeforeSuite(func() {
	readConfig(cfgPath)

	var err error
	tempdir, err = ioutil.TempDir("", "eaaTestBuild")
	if err != nil {
		Fail("Unable to create temporary build directory")
	}

	copyCerts(TestCertsDir)
	compareCerts(TestCertsDir)

	tempConfCaRootKeyPath = tempdir + "/" + "certs/eaa/rootCA.key"
	tempConfCaRootPath = tempdir + "/" + "certs/eaa/rootCA.pem"
	tempConfServerCertPath = tempdir + "/" + "certs/eaa/server.pem"
	tempConfServerKeyPath = tempdir + "/" + "certs/eaa/server.key"

	generateConfigs()

	err = fakeAppidProvider()
	Expect(err).ToNot(HaveOccurred(), "Unable to start fake AppID provider")
})

var _ = AfterSuite(func() {

	defer os.RemoveAll(tempdir) // cleanup temporary build directory

})
