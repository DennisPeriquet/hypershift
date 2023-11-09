package kubevirt

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	suppconfig "github.com/openshift/hypershift/support/config"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	capikubevirt "sigs.k8s.io/cluster-api-provider-kubevirt/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	hyperv1 "github.com/openshift/hypershift/api/v1beta1"
)

const (
	namespace              = "my-ns"
	poolName               = "my-pool"
	clusterName            = "my-cluster"
	infraId                = "infraId123"
	hostedClusterNamespace = "hostedClusterNamespace"
	bootImageName          = "https://rhcos.mirror.openshift.com/art/storage/releases/rhcos-4.12/412.86.202209302317-0/x86_64/rhcos-412.86.202209302317-0-openstack.x86_64.qcow2.gz"
	imageHash              = "imageHash"
)

func TestKubevirtMachineTemplate(t *testing.T) {
	testCases := []struct {
		name     string
		nodePool *hyperv1.NodePool
		hcluster *hyperv1.HostedCluster
		expected *capikubevirt.KubevirtMachineTemplateSpec
	}{
		{
			name: "happy flow",
			nodePool: &hyperv1.NodePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      poolName,
					Namespace: namespace,
				},
				Spec: hyperv1.NodePoolSpec{
					ClusterName: clusterName,
					Replicas:    nil,
					Config:      nil,
					Management:  hyperv1.NodePoolManagement{},
					AutoScaling: nil,
					Platform: hyperv1.NodePoolPlatform{
						Type: hyperv1.KubevirtPlatform,
						Kubevirt: generateKubevirtPlatform(
							memoryNPOption("5Gi"),
							coresNPOption(4),
							imageNPOption("testimage"),
							volumeNPOption("32Gi"),
						),
					},
					Release: hyperv1.Release{},
				},
			},
			hcluster: &hyperv1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-hostedcluster",
					Namespace: "clusters",
				},
				Spec: hyperv1.HostedClusterSpec{
					InfraID: "1234",
				},
			},

			expected: &capikubevirt.KubevirtMachineTemplateSpec{
				Template: capikubevirt.KubevirtMachineTemplateResource{
					Spec: capikubevirt.KubevirtMachineSpec{
						VirtualMachineTemplate: *generateNodeTemplate(
							memoryTmpltOpt("5Gi"),
							cpuTmpltOpt(4),
							storageTmpltOpt("32Gi"),
						),
					},
				},
			},
		},
		{
			name: "happy flow - QoS CLass Guaranteed",
			nodePool: &hyperv1.NodePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      poolName,
					Namespace: namespace,
				},
				Spec: hyperv1.NodePoolSpec{
					ClusterName: clusterName,
					Replicas:    nil,
					Config:      nil,
					Management:  hyperv1.NodePoolManagement{},
					AutoScaling: nil,
					Platform: hyperv1.NodePoolPlatform{
						Type: hyperv1.KubevirtPlatform,
						Kubevirt: generateKubevirtPlatform(
							memoryNPOption("5Gi"),
							coresNPOption(4),
							imageNPOption("testimage"),
							volumeNPOption("32Gi"),
							qosClassGuaranteedNPOption(),
						),
					},
					Release: hyperv1.Release{},
				},
			},
			hcluster: &hyperv1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-hostedcluster",
					Namespace: "clusters",
				},
				Spec: hyperv1.HostedClusterSpec{
					InfraID: "1234",
				},
			},

			expected: &capikubevirt.KubevirtMachineTemplateSpec{
				Template: capikubevirt.KubevirtMachineTemplateResource{
					Spec: capikubevirt.KubevirtMachineSpec{
						VirtualMachineTemplate: *generateNodeTemplate(
							storageTmpltOpt("32Gi"),
							guaranteedResourcesOpt(4, "5Gi"),
						),
					},
				},
			},
		},
		{
			name: "NetworkInterfaceMultiQueue is Disable",
			nodePool: &hyperv1.NodePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      poolName,
					Namespace: namespace,
				},
				Spec: hyperv1.NodePoolSpec{
					ClusterName: clusterName,
					Replicas:    nil,
					Config:      nil,
					Management:  hyperv1.NodePoolManagement{},
					AutoScaling: nil,
					Platform: hyperv1.NodePoolPlatform{
						Type: hyperv1.KubevirtPlatform,
						Kubevirt: generateKubevirtPlatform(
							memoryNPOption("5Gi"),
							coresNPOption(4),
							imageNPOption("testimage"),
							volumeNPOption("32Gi"),
							multiQueueNPOption(hyperv1.MultiQueueDisable),
						),
					},
					Release: hyperv1.Release{},
				},
			},
			hcluster: &hyperv1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-hostedcluster",
					Namespace: "clusters",
				},
				Spec: hyperv1.HostedClusterSpec{
					InfraID: "1234",
				},
			},

			expected: &capikubevirt.KubevirtMachineTemplateSpec{
				Template: capikubevirt.KubevirtMachineTemplateResource{
					Spec: capikubevirt.KubevirtMachineSpec{
						VirtualMachineTemplate: *generateNodeTemplate(
							memoryTmpltOpt("5Gi"),
							cpuTmpltOpt(4),
							storageTmpltOpt("32Gi"),
						),
					},
				},
			},
		},
		{
			name: "NetworkInterfaceMultiQueue is Enable",
			nodePool: &hyperv1.NodePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      poolName,
					Namespace: namespace,
				},
				Spec: hyperv1.NodePoolSpec{
					ClusterName: clusterName,
					Replicas:    nil,
					Config:      nil,
					Management:  hyperv1.NodePoolManagement{},
					AutoScaling: nil,
					Platform: hyperv1.NodePoolPlatform{
						Type: hyperv1.KubevirtPlatform,
						Kubevirt: generateKubevirtPlatform(
							memoryNPOption("5Gi"),
							coresNPOption(4),
							imageNPOption("testimage"),
							volumeNPOption("32Gi"),
							multiQueueNPOption(hyperv1.MultiQueueEnable),
						),
					},
					Release: hyperv1.Release{},
				},
			},
			hcluster: &hyperv1.HostedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-hostedcluster",
					Namespace: "clusters",
				},
				Spec: hyperv1.HostedClusterSpec{
					InfraID: "1234",
				},
			},

			expected: &capikubevirt.KubevirtMachineTemplateSpec{
				Template: capikubevirt.KubevirtMachineTemplateResource{
					Spec: capikubevirt.KubevirtMachineSpec{
						VirtualMachineTemplate: *generateNodeTemplate(
							memoryTmpltOpt("5Gi"),
							cpuTmpltOpt(4),
							storageTmpltOpt("32Gi"),
							networkInterfaceMultiQueueTmpltOpt(),
						),
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(PlatformValidation(tc.nodePool)).To(Succeed())

			bootImage := newCachedBootImage(bootImageName, imageHash, hostedClusterNamespace, false)
			bootImage.dvName = bootImageNamePrefix + "12345"
			result := MachineTemplateSpec(tc.nodePool, bootImage, tc.hcluster)
			g.Expect(result).To(Equal(tc.expected), "Comparison failed\n%v", cmp.Diff(tc.expected, result))
		})
	}
}

func TestCacheImage(t *testing.T) {
	nodePool := &hyperv1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      poolName,
			Namespace: namespace,
		},
		Spec: hyperv1.NodePoolSpec{
			ClusterName: clusterName,
			Replicas:    nil,
			Config:      nil,
			Management:  hyperv1.NodePoolManagement{},
			AutoScaling: nil,
			Platform: hyperv1.NodePoolPlatform{
				Type: hyperv1.KubevirtPlatform,
				Kubevirt: generateKubevirtPlatform(
					memoryNPOption("5Gi"),
					coresNPOption(4),
					imageNPOption("testimage"),
					volumeNPOption("32Gi"),
				),
			},
			Release: hyperv1.Release{},
		},
	}

	testCases := []struct {
		name              string
		nodePool          *hyperv1.NodePool
		existingResources []client.Object
		asserFunc         func(Gomega, []v1beta1.DataVolume, string, *cachedBootImage)
		errExpected       bool
		dvNamePrefix      string
	}{
		{
			name:         "happy flow - no existing PVC",
			nodePool:     nodePool,
			errExpected:  false,
			dvNamePrefix: bootImageNamePrefix,
			asserFunc:    assertDV,
		},
		{
			name:        "happy flow - PVC already exists",
			nodePool:    nodePool,
			errExpected: false,
			existingResources: []client.Object{
				&v1beta1.DataVolume{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: bootImageNamePrefix,
						Name:         bootImageNamePrefix + "already-exists",
						Namespace:    hostedClusterNamespace,
						Annotations: map[string]string{
							bootImageDVAnnotationHash: imageHash,
						},
						Labels: map[string]string{
							bootImageDVLabelRoleName: bootImageDVLabelRoleValue,
							bootImageDVLabelUID:      infraId,
						},
					},
				},
			},
			dvNamePrefix: bootImageNamePrefix + "already-exists",
			asserFunc:    assertDV,
		},
		{
			name:        "cleanup - different hash",
			nodePool:    nodePool,
			errExpected: false,
			existingResources: []client.Object{
				&v1beta1.DataVolume{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: bootImageNamePrefix,
						Name:         bootImageNamePrefix + "another-one",
						Namespace:    hostedClusterNamespace,
						Annotations: map[string]string{
							bootImageDVAnnotationHash: "otherImageHash",
							bootImageDVLabelUID:       infraId,
						},
						Labels: map[string]string{
							bootImageDVLabelRoleName: bootImageDVLabelRoleValue,
							bootImageDVLabelUID:      infraId,
						},
					},
				},
			},
			dvNamePrefix: bootImageNamePrefix,
			asserFunc:    assertDV,
		},
		{
			name:        "cleanup - different cluster - should not clean",
			nodePool:    nodePool,
			errExpected: false,
			existingResources: []client.Object{
				&v1beta1.DataVolume{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: bootImageNamePrefix,
						Name:         bootImageNamePrefix + "another-one",
						Namespace:    hostedClusterNamespace,
						Annotations: map[string]string{
							bootImageDVAnnotationHash: imageHash,
							bootImageDVLabelUID:       "not-the-same-cluster",
						},
						Labels: map[string]string{
							bootImageDVLabelRoleName: bootImageDVLabelRoleValue,
						},
					},
				},
			},
			dvNamePrefix: bootImageNamePrefix,
			asserFunc: func(g Gomega, dvs []v1beta1.DataVolume, expectedDVNamePrefix string, bootImage *cachedBootImage) {
				g.ExpectWithOffset(1, dvs).Should(HaveLen(2), "should not clear the other DV")
				for _, dv := range dvs {
					if dv.Name != bootImageNamePrefix+"another-one" {
						g.ExpectWithOffset(1, dv.Name).Should(HavePrefix(expectedDVNamePrefix))
						g.ExpectWithOffset(1, bootImage.dvName).Should(Equal(dv.Name), "failed to set the dvName filed in the cacheImage object")
					}
				}
			},
		},
	}

	ctx := logr.NewContext(context.Background(), zapr.NewLogger(zaptest.NewLogger(t)))
	for _, tc := range testCases {
		t.Run(tc.name, func(tst *testing.T) {
			g := NewWithT(tst)
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			_ = v1beta1.AddToScheme(scheme)
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.existingResources...).Build()

			bootImage := newCachedBootImage(bootImageName, imageHash, hostedClusterNamespace, false)
			err := bootImage.CacheImage(ctx, cl, tc.nodePool, infraId)

			if tc.errExpected != (err != nil) {
				if tc.errExpected {
					g.Fail("should return error, but it didn't")
				} else {
					g.Fail(fmt.Sprintf(`should not return error, but it did; the error is: "%v"`, err))
				}
			}

			dvs := v1beta1.DataVolumeList{}
			g.Expect(cl.List(ctx, &dvs)).To(Succeed())
			tc.asserFunc(g, dvs.Items, tc.dvNamePrefix, bootImage)
		})
	}
}

func assertDV(g Gomega, dvs []v1beta1.DataVolume, expectedDVNamePrefix string, bootImage *cachedBootImage) {
	g.ExpectWithOffset(1, dvs).Should(HaveLen(1), "failed to read the DataVolume back; No matched DataVolume")
	g.ExpectWithOffset(1, dvs[0].Name).Should(HavePrefix(expectedDVNamePrefix))
	g.ExpectWithOffset(1, bootImage.dvName).Should(Equal(dvs[0].Name), "failed to set the dvName filed in the cacheImage object")
}

type nodePoolOption func(kvNodePool *hyperv1.KubevirtNodePoolPlatform)

func memoryNPOption(memory string) nodePoolOption {
	return func(kvNodePool *hyperv1.KubevirtNodePoolPlatform) {
		if kvNodePool.Compute == nil {
			kvNodePool.Compute = &hyperv1.KubevirtCompute{}
		}

		memoryQuantity := apiresource.MustParse(memory)
		kvNodePool.Compute.Memory = &memoryQuantity
	}
}

func coresNPOption(cores uint32) nodePoolOption {
	return func(kvNodePool *hyperv1.KubevirtNodePoolPlatform) {
		if kvNodePool.Compute == nil {
			kvNodePool.Compute = &hyperv1.KubevirtCompute{}
		}

		kvNodePool.Compute.Cores = &cores
	}
}

func qosClassGuaranteedNPOption() nodePoolOption {
	return func(kvNodePool *hyperv1.KubevirtNodePoolPlatform) {
		if kvNodePool.Compute == nil {
			kvNodePool.Compute = &hyperv1.KubevirtCompute{}
		}

		qosClassGuaranteed := hyperv1.QoSClassGuaranteed
		kvNodePool.Compute.QosClass = &qosClassGuaranteed
	}
}

func imageNPOption(image string) nodePoolOption {
	return func(kvNodePool *hyperv1.KubevirtNodePoolPlatform) {
		if kvNodePool.RootVolume == nil {
			kvNodePool.RootVolume = &hyperv1.KubevirtRootVolume{}
		}

		kvNodePool.RootVolume.Image = &hyperv1.KubevirtDiskImage{
			ContainerDiskImage: &image,
		}
	}
}

func volumeNPOption(volumeSize string) nodePoolOption {
	volumeSizeQuantity := apiresource.MustParse(volumeSize)

	return func(kvNodePool *hyperv1.KubevirtNodePoolPlatform) {
		if kvNodePool.RootVolume == nil {
			kvNodePool.RootVolume = &hyperv1.KubevirtRootVolume{}
		}

		kvNodePool.RootVolume.KubevirtVolume = hyperv1.KubevirtVolume{
			Type: hyperv1.KubevirtVolumeTypePersistent,
			Persistent: &hyperv1.KubevirtPersistentVolume{
				Size:         &volumeSizeQuantity,
				StorageClass: nil,
			},
		}
	}
}

func multiQueueNPOption(multiQueue hyperv1.MultiQueueSetting) nodePoolOption {
	return func(kvNodePool *hyperv1.KubevirtNodePoolPlatform) {
		kvNodePool.NetworkInterfaceMultiQueue = &multiQueue
	}
}

func generateKubevirtPlatform(options ...nodePoolOption) *hyperv1.KubevirtNodePoolPlatform {
	exampleTemplate := &hyperv1.KubevirtNodePoolPlatform{}

	for _, opt := range options {
		opt(exampleTemplate)
	}

	return exampleTemplate
}

type nodeTemplateOption func(template *capikubevirt.VirtualMachineTemplateSpec)

func cpuTmpltOpt(cores uint32) nodeTemplateOption {
	return func(template *capikubevirt.VirtualMachineTemplateSpec) {
		template.Spec.Template.Spec.Domain.CPU = &kubevirtv1.CPU{Cores: cores}
	}
}

func memoryTmpltOpt(memory string) nodeTemplateOption {
	guestQuantity := apiresource.MustParse(memory)

	return func(template *capikubevirt.VirtualMachineTemplateSpec) {
		template.Spec.Template.Spec.Domain.Memory = &kubevirtv1.Memory{Guest: &guestQuantity}
	}
}

func storageTmpltOpt(volumeSize string) nodeTemplateOption {
	storage := &v1beta1.StorageSpec{
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: apiresource.MustParse(volumeSize),
			},
		},
	}

	return func(template *capikubevirt.VirtualMachineTemplateSpec) {
		if len(template.Spec.DataVolumeTemplates) == 1 {
			template.Spec.DataVolumeTemplates[0].Spec.Storage = storage
		}
	}
}

func networkInterfaceMultiQueueTmpltOpt() nodeTemplateOption {
	return func(template *capikubevirt.VirtualMachineTemplateSpec) {
		template.Spec.Template.Spec.Domain.Devices.NetworkInterfaceMultiQueue = pointer.Bool(true)
	}
}

func guaranteedResourcesOpt(cores uint32, memory string) nodeTemplateOption {
	memReq := apiresource.MustParse(memory)
	coresReq := *apiresource.NewQuantity(int64(cores), apiresource.DecimalSI)

	return func(template *capikubevirt.VirtualMachineTemplateSpec) {
		if len(template.Spec.Template.Spec.Domain.Resources.Requests) == 0 {
			template.Spec.Template.Spec.Domain.Resources.Requests = make(corev1.ResourceList)
		}

		if len(template.Spec.Template.Spec.Domain.Resources.Limits) == 0 {
			template.Spec.Template.Spec.Domain.Resources.Limits = make(corev1.ResourceList)
		}

		template.Spec.Template.Spec.Domain.Resources.Requests[corev1.ResourceMemory] = memReq
		template.Spec.Template.Spec.Domain.Resources.Limits[corev1.ResourceMemory] = memReq

		template.Spec.Template.Spec.Domain.Resources.Requests[corev1.ResourceCPU] = coresReq
		template.Spec.Template.Spec.Domain.Resources.Limits[corev1.ResourceCPU] = coresReq
	}
}

func generateNodeTemplate(options ...nodeTemplateOption) *capikubevirt.VirtualMachineTemplateSpec {
	runAlways := kubevirtv1.RunStrategyAlways

	template := &capikubevirt.VirtualMachineTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				hyperv1.NodePoolNameLabel: "my-pool",
				hyperv1.InfraIDLabel:      "1234",
			},
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: &runAlways,
			DataVolumeTemplates: []kubevirtv1.DataVolumeTemplateSpec{
				{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name: "rhcos",
					},
					Spec: v1beta1.DataVolumeSpec{
						Source: &v1beta1.DataVolumeSource{
							PVC: &v1beta1.DataVolumeSourcePVC{
								Namespace: hostedClusterNamespace,
								Name:      bootImageNamePrefix + "12345",
							},
						},
					},
				},
			},
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						hyperv1.NodePoolNameLabel: "my-pool",
						hyperv1.InfraIDLabel:      "1234",
					},
					Annotations: map[string]string{
						suppconfig.PodSafeToEvictLocalVolumesKey:              strings.Join(LocalStorageVolumes, ","),
						"kubevirt.io/allow-pod-bridge-network-live-migration": "",
					},
				},
				Spec: kubevirtv1.VirtualMachineInstanceSpec{

					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: int32(100),
									PodAffinityTerm: corev1.PodAffinityTerm{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      hyperv1.NodePoolNameLabel,
													Operator: metav1.LabelSelectorOpIn,
													Values:   []string{"my-pool"},
												},
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
					},

					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							Disks: []kubevirtv1.Disk{
								{
									Name: "rhcos",
									DiskDevice: kubevirtv1.DiskDevice{
										Disk: &kubevirtv1.DiskTarget{
											Bus: "virtio",
										},
									},
								},
							},
							Interfaces: []kubevirtv1.Interface{
								{
									Name: "default",
									InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
										Bridge: &kubevirtv1.InterfaceBridge{},
									},
								},
							},
						},
					},

					Networks: []kubevirtv1.Network{
						{
							Name: "default",
							NetworkSource: kubevirtv1.NetworkSource{
								Pod: &kubevirtv1.PodNetwork{},
							},
						},
					},
					Volumes: []kubevirtv1.Volume{
						{
							Name: "rhcos",
							VolumeSource: kubevirtv1.VolumeSource{
								DataVolume: &kubevirtv1.DataVolumeSource{
									Name: "rhcos",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, opt := range options {
		opt(template)
	}

	return template
}
