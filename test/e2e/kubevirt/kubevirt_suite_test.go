// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package kubevirt_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/api/core/v1"
	k8yaml "sigs.k8s.io/yaml"

	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

const (
	TEST_VM_TEMPLATE_PATH = "./testvm.yaml"
	randomLength          = 6
)

var (
	vmTemplate = new(kubevirtv1.VirtualMachine)
	frame      *e2e.Framework
)

func TestKubevirt(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kubevirt Suite")
}

var _ = BeforeSuite(func() {
	defer GinkgoRecover()

	if common.CheckRunOverlayCNI() {
		Skip("overlay CNI is installed , ignore this suite")
	}
	if !common.CheckMultusFeatureOn() {
		Skip("multus is not installed , ignore this suite")
	}

	var err error
	frame, err = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{spiderpool.AddToScheme, kubevirtv1.AddToScheme})
	Expect(err).NotTo(HaveOccurred())

	// make sure we have macvlan net-attach-def resource
	multusInstance, err := frame.GetMultusInstance(common.MacvlanUnderlayVlan0, common.MultusNs)
	Expect(err).NotTo(HaveOccurred())
	Expect(multusInstance).NotTo(BeNil())

	readTestVMTemplate()
})

func readTestVMTemplate() {
	bytes, err := os.ReadFile(TEST_VM_TEMPLATE_PATH)
	Expect(err).NotTo(HaveOccurred())

	err = k8yaml.Unmarshal(bytes, vmTemplate)
	Expect(err).NotTo(HaveOccurred())
}
