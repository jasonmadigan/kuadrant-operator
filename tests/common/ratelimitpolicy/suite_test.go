//go:build integration

/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ratelimitpolicy

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	controllers "github.com/kuadrant/kuadrant-operator/internal/controller"
	"github.com/kuadrant/kuadrant-operator/internal/log"
	"github.com/kuadrant/kuadrant-operator/tests"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

// This test suite will be run on k8s env with GatewayAPI CRDs, Istio and Kuadrant CRDs installed

var k8sClient client.Client
var testEnv *envtest.Environment
var kuadrantInstallationNS string

func testClient() client.Client { return k8sClient }

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "RateLimitPolicy Controller Suite")
}

const (
	TestGatewayName   = "test-placed-gateway"
	TestHTTPRouteName = "toystore-route"
)

var _ = SynchronizedBeforeSuite(func() []byte {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: &[]bool{true}[0],
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	s := controllers.BootstrapScheme()

	controllers.SetupKuadrantOperatorForTest(s, cfg)

	k8sClient, err = client.New(cfg, client.Options{Scheme: s})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	ctx := context.Background()
	ns := tests.CreateNamespace(ctx, testClient())
	tests.ApplyKuadrantCR(ctx, testClient(), ns)

	data := controllers.MarshalConfig(cfg, controllers.WithKuadrantInstallNS(ns))

	return data
}, func(data []byte) {
	// Unmarshal the shared configuration struct
	var sharedCfg controllers.SharedConfig
	Expect(json.Unmarshal(data, &sharedCfg)).To(Succeed())

	// Create the rest.Config object from the shared configuration
	cfg := &rest.Config{
		Host: sharedCfg.Host,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: sharedCfg.TLSClientConfig.Insecure,
			CertData: sharedCfg.TLSClientConfig.CertData,
			KeyData:  sharedCfg.TLSClientConfig.KeyData,
			CAData:   sharedCfg.TLSClientConfig.CAData,
		},
	}

	kuadrantInstallationNS = sharedCfg.KuadrantNS

	// Create new scheme for each client
	s := controllers.BootstrapScheme()

	// Set the shared configuration
	var err error
	k8sClient, err = client.New(cfg, client.Options{Scheme: s})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	tests.GatewayClassName = os.Getenv("GATEWAYAPI_PROVIDER")
	Expect(tests.GatewayClassName).NotTo(BeZero(), "Please make sure GATEWAYAPI_PROVIDER is set correctly.")
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	By("tearing down the test environment")
	tests.DeleteNamespace(context.Background(), k8sClient, kuadrantInstallationNS)
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func TestMain(m *testing.M) {
	logger := log.NewLogger(
		log.SetLevel(log.DebugLevel),
		log.SetMode(log.ModeDev),
		log.WriteTo(GinkgoWriter),
	).WithName("ratelimitpolicy_controller_test")
	log.SetLogger(logger)
	os.Exit(m.Run())
}
