// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package kubevirt_test

import (
	"context"
	"fmt"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = FDescribe("test kubevirt", Label("kubevirt"), func() {
	var (
		virtualMachine *kubevirtv1.VirtualMachine
		ctx            context.Context
		namespace      string
	)

	BeforeEach(func() {
		ctx = context.TODO()

		// make sure the vm has the macvlan annotation.
		virtualMachine = vmTemplate.DeepCopy()
		anno := virtualMachine.Spec.Template.ObjectMeta.GetAnnotations()
		anno[common.MultusDefaultNetwork] = common.MacvlanUnderlayVlan0
		virtualMachine.Spec.Template.ObjectMeta.SetAnnotations(anno)

		// create namespace
		//namespace = "ns" + utilrand.String(randomLength)
		namespace = "kube-system"
		//GinkgoWriter.Printf("create namespace %v. \n", namespace)
		//err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
		//Expect(err).NotTo(HaveOccurred())

		//DeferCleanup(func() {
		//	if CurrentSpecReport().Failed() {
		//		GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
		//		return
		//	}
		//
		//	GinkgoWriter.Printf("delete namespace %v. \n", namespace)
		//	Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())
		//})
	})

	It("Succeed to keep static IP for kubevirt VM/VMI after restarting the VM/VMI pod", Label("F00001"), func() {
		// 1. create a kubevirt vm with passt network mode
		virtualMachine.Spec.Template.Spec.Networks = []kubevirtv1.Network{
			{
				Name: "default",
				NetworkSource: kubevirtv1.NetworkSource{
					Pod: &kubevirtv1.PodNetwork{},
				},
			},
		}
		virtualMachine.Spec.Template.Spec.Domain.Devices.Interfaces = []kubevirtv1.Interface{
			{
				Name: "default",
				InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
					Passt: &kubevirtv1.InterfacePasst{},
				},
			},
		}
		virtualMachine.Name = fmt.Sprintf("%s-%s", virtualMachine.Name, utilrand.String(randomLength))
		virtualMachine.Namespace = namespace
		GinkgoWriter.Printf("try to create kubevirt VM: %v \n", virtualMachine)
		err := frame.CreateResource(virtualMachine)
		Expect(err).NotTo(HaveOccurred())

		// 2. wait for the vmi to be ready and record the vmi corresponding vmi pod IP
		vmi, err := waitVMIUntilRunning(virtualMachine.Namespace, virtualMachine.Name, time.Minute*3)
		Expect(err).NotTo(HaveOccurred())

		vmInterfaces := make(map[string][]string)
		for _, vmNetworkInterface := range vmi.Status.Interfaces {
			ips := vmNetworkInterface.IPs
			sort.Strings(ips)
			vmInterfaces[vmNetworkInterface.Name] = ips
		}
		GinkgoWriter.Printf("original VMI NIC allocations: %v \n", vmInterfaces)

		// 3. restart the vmi object and compare the new vmi pod IP whether is same with the previous-recorded IP
		GinkgoWriter.Printf("try to restart VMI %s/%s", vmi.Namespace, vmi.Name)
		err = frame.KClient.Delete(ctx, vmi)
		Expect(err).NotTo(HaveOccurred())
		vmi, err = waitVMIUntilRunning(virtualMachine.Namespace, virtualMachine.Name, time.Minute*5)
		Expect(err).NotTo(HaveOccurred())

		tmpVMInterfaces := make(map[string][]string)
		for _, vmNetworkInterface := range vmi.Status.Interfaces {
			ips := vmNetworkInterface.IPs
			sort.Strings(ips)
			tmpVMInterfaces[vmNetworkInterface.Name] = ips
		}
		GinkgoWriter.Printf("new VMI NIC allocations: %v \n", tmpVMInterfaces)
		Expect(vmInterfaces).Should(Equal(tmpVMInterfaces))
	})

	PIt("Succeed to keep static IP for the kubevirt VM live migration", Label("F00002"), func() {
		// 1. create a kubevirt vm with masquerade mode (At present, it seems like the live migration only supports masquerade mode)
		virtualMachine.Spec.Template.Spec.Networks = []kubevirtv1.Network{
			{
				Name: "default",
				NetworkSource: kubevirtv1.NetworkSource{
					Pod: &kubevirtv1.PodNetwork{},
				},
			},
		}
		virtualMachine.Spec.Template.Spec.Domain.Devices.Interfaces = []kubevirtv1.Interface{
			{
				Name: "default",
				InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
					Masquerade: &kubevirtv1.InterfaceMasquerade{},
				},
			},
		}
		virtualMachine.Name = fmt.Sprintf("%s-%s", virtualMachine.Name, utilrand.String(randomLength))
		virtualMachine.Namespace = namespace
		GinkgoWriter.Printf("try to create kubevirt VM: %v \n", virtualMachine)
		err := frame.CreateResource(virtualMachine)
		Expect(err).NotTo(HaveOccurred())

		// 2. record the vmi corresponding vmi pod IP
		_, err = waitVMIUntilRunning(virtualMachine.Namespace, virtualMachine.Name, time.Minute*10)
		Expect(err).NotTo(HaveOccurred())

		var podList corev1.PodList
		err = frame.KClient.List(ctx, &podList, client.MatchingLabels{
			kubevirtv1.VirtualMachineNameLabel: virtualMachine.Name,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(podList.Items).To(HaveLen(1))
		originalPodName := podList.Items[0].Name
		originalPodIPs := podList.Items[0].Status.PodIPs
		GinkgoWriter.Printf("original virt-launcher pod '%s/%s' IP allocations: %v \n", namespace, originalPodName, originalPodIPs)

		// 3. create a vm migration
		vmim := &kubevirtv1.VirtualMachineInstanceMigration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-migration", virtualMachine.Name),
				Namespace: virtualMachine.Namespace,
			},
			Spec: kubevirtv1.VirtualMachineInstanceMigrationSpec{
				VMIName: virtualMachine.Name,
			},
		}
		GinkgoWriter.Printf("try to create VirtualMachineInstanceMigration: %v \n", vmim)
		err = frame.KClient.Create(ctx, vmim)
		Expect(err).NotTo(HaveOccurred())

		// 4. wait for the completion of the migration and compare the new vmi pod IP whether is same with the previous-recorded IP
		Eventually(func() error {
			tmpPod, err := frame.GetPod(originalPodName, virtualMachine.Namespace)
			if nil != err {
				return err
			}
			if tmpPod.Status.Phase == corev1.PodSucceeded {
				return nil
			}
			return fmt.Errorf("virt-launcher pod %s/%s phase is %s, the vm is still in live migration phase", tmpPod.Namespace, tmpPod.Name, tmpPod.Status.Phase)
		}).WithTimeout(time.Minute * 10).WithPolling(time.Second * 5).Should(BeNil())
		GinkgoWriter.Printf("virt-launcher pod %s/%s is completed\n", namespace, originalPodName)

		var newPodList corev1.PodList
		err = frame.KClient.List(ctx, &newPodList, client.MatchingLabels{
			kubevirtv1.VirtualMachineNameLabel: virtualMachine.Name,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(newPodList.Items).To(HaveLen(2))
		for _, tmpPod := range newPodList.Items {
			if tmpPod.Name == originalPodName {
				continue
			}
			GinkgoWriter.Printf("the new migration virt-launcher pod %s/%s IP allocations: %v \n", tmpPod.Namespace, tmpPod.Name, tmpPod.Status.PodIPs)
			Expect(tmpPod.Status.PodIPs).To(Equal(originalPodIPs))
		}
	})
})

func waitVMIUntilRunning(namespace, name string, timeout time.Duration) (*kubevirtv1.VirtualMachineInstance, error) {
	tick := time.Tick(timeout)
	var vmi kubevirtv1.VirtualMachineInstance

	for {
		select {
		case <-tick:
			//GinkgoWriter.Printf("VMI %s/%s is still in phase %s \n", namespace, name, vmi.Status.Phase)
			GinkgoWriter.Printf("=======++++++++out of time, VMI: %+v \n", vmi)
			var podList corev1.PodList
			e := frame.KClient.List(context.TODO(), &podList)
			if e == nil {
				for index := range podList.Items {
					GinkgoWriter.Printf("-------------pod Name: %s \n", podList.Items[index].Name)
					_, ok := podList.Items[index].GetLabels()[kubevirtv1.VirtualMachineNameLabel]
					if ok {
						GinkgoWriter.Printf("============++++++++++++++ VM Pod: %s \n", podList.Items[index].String())
					}
				}
			} else {
				GinkgoWriter.Printf("???????????????????????Bad: %v \n", e)
			}

			vmiEvents, err := frame.GetEvents(context.TODO(), "VirtualMachineInstance", name, namespace)
			Expect(err).NotTo(HaveOccurred())
			for _, item := range vmiEvents.Items {
				GinkgoWriter.Printf("==========+++++++++++++++++============== vmi events: %v \n", item)
			}

			vmEvents, err := frame.GetEvents(context.TODO(), "VirtualMachine", name, namespace)
			Expect(err).NotTo(HaveOccurred())
			for _, item := range vmEvents.Items {
				GinkgoWriter.Printf("***************************************** vm events: %v \n", item)
			}

			return nil, fmt.Errorf("time out to wait VMI %s/%s running, error: %v", namespace, name, e)

		default:
			err := frame.GetResource(types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			}, &vmi)
			if nil != err {
				if errors.IsNotFound(err) {
					time.Sleep(time.Second * 5)
					continue
				}

				return nil, err
			}
			if vmi.Status.Phase == kubevirtv1.Running {
				return &vmi, nil
			}
			time.Sleep(time.Second * 5)
		}
	}
}
