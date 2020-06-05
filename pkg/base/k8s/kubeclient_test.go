package k8s

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	vcrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	coreV1 "k8s.io/api/core/v1"
	k8sCl "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testNs             = "default"
	testID             = "someID"
	testNode1Name      = "node1"
	testDriveLocation1 = "drive"
)

var (
	testLogger = logrus.New()
	testCtx    = context.Background()
	testUUID   = uuid.New().String()
	testVolume = vcrd.Volume{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "Volume", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testID, Namespace: testNs},
		Spec: api.Volume{
			Id:       testID,
			NodeId:   "pod",
			Size:     1000,
			Type:     "Type",
			Location: "location",
		},
	}

	testApiAC = api.AvailableCapacity{
		Size:         1024 * 1024,
		StorageClass: apiV1.StorageClassHDD,
		Location:     testDriveLocation1,
		NodeId:       testNode1Name,
	}
	testACTypeMeta = k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1Version}
	testACName     = fmt.Sprintf("%s-%s", testApiAC.NodeId, testApiAC.Location)
	testACCR       = accrd.AvailableCapacity{
		TypeMeta:   testACTypeMeta,
		ObjectMeta: k8smetav1.ObjectMeta{Name: testACName, Namespace: testNs},
		Spec:       testApiAC,
	}

	testApiDrive = api.Drive{
		UUID:         testUUID,
		VID:          "testVID",
		PID:          "testPID",
		SerialNumber: "testSN",
		NodeId:       testNode1Name,
		Health:       apiV1.HealthGood,
		Type:         apiV1.DriveTypeHDD,
		Size:         1024 * 1024,
		Status:       apiV1.DriveStatusOnline,
	}
	testDriveTypeMeta = k8smetav1.TypeMeta{Kind: "Drive", APIVersion: apiV1.APIV1Version}
	testDriveCR       = drivecrd.Drive{
		TypeMeta:   testDriveTypeMeta,
		ObjectMeta: k8smetav1.ObjectMeta{Name: testUUID, Namespace: testNs},
		Spec:       testApiDrive,
	}

	testVolumeTypeMeta = k8smetav1.TypeMeta{Kind: "Volume", APIVersion: apiV1.APIV1Version}
	testApiVolume      = api.Volume{
		Id:       testID,
		NodeId:   testNode1Name,
		Size:     1000,
		Type:     "Type",
		Location: "location",
	}
	testVolumeCR = vcrd.Volume{
		TypeMeta: testVolumeTypeMeta,
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:              testID,
			Namespace:         testNs,
			CreationTimestamp: k8smetav1.Time{Time: time.Now()},
		},
		Spec: testApiVolume,
	}

	testApiLVG = api.LogicalVolumeGroup{
		Name:      testUUID,
		Node:      testNode1Name,
		Locations: []string{testDriveLocation1},
		Size:      1024,
	}
	testLVGName = fmt.Sprintf("lvg-%s", strings.ToLower(testApiLVG.Locations[0]))
	testLVGCR   = lvgcrd.LVG{
		TypeMeta: k8smetav1.TypeMeta{
			Kind:       "LVG",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      testLVGName,
			Namespace: testNs,
		},
		Spec: testApiLVG,
	}
)

// variables to test pods listing
var (
	// pod with prefix 1
	testPodNamePrefix1 = "pod-prefix-1"
	testPod1Name       = fmt.Sprintf("%s-testPod1", testPodNamePrefix1)
	testReadyPod1      = &coreV1.Pod{
		ObjectMeta: k8smetav1.ObjectMeta{Name: testPod1Name, Namespace: testNs},
	}

	// pod with prefix 2
	testPodNamePrefix2 = "pod-prefix-2"
	testPod2Name       = fmt.Sprintf("%s-testPod2", testPodNamePrefix2)
	testUnreadyPod2    = &coreV1.Pod{
		ObjectMeta: k8smetav1.ObjectMeta{Name: testPod2Name, Namespace: testNs},
	}

	// pod from another namespace
	testPod3Name = "SomeName"
	testPod3     = &coreV1.Pod{
		ObjectMeta: k8smetav1.ObjectMeta{Name: testPod3Name},
	}
)

func TestKubernetesClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kubernetes client testing suite")
}

var _ = Describe("pods listing", func() {
	var (
		kubeClient *KubeClient
		err        error
	)

	BeforeEach(func() {
		kubeClient, err = GetFakeKubeClient(testNs, testLogger)
		Expect(err).To(BeNil())
		// create test PODs
		createPods(kubeClient, testReadyPod1, testUnreadyPod2, testPod3)
	})

	AfterEach(func() {
		removeAllPods(kubeClient)
	})

	Context("Obtain list of pods with specific prefix", func() {
		It("Must receive pod 1", func() {
			pods, err := kubeClient.GetPods(testCtx, testPodNamePrefix1)
			Expect(err).To(BeNil())
			Expect(len(pods)).To(Equal(1))
			Expect(pods[0].Name).To(Equal(testPod1Name))
		})

		It("Must receive pod 2", func() {
			pods, err := kubeClient.GetPods(testCtx, testPodNamePrefix2)
			Expect(err).To(BeNil())
			Expect(len(pods)).To(Equal(1))
			Expect(pods[0].Name).To(Equal(testPod2Name))
		})

		It("Must receive all pods in namespace", func() {
			pods, err := kubeClient.GetPods(testCtx, "")
			Expect(err).To(BeNil())
			Expect(len(pods)).To(Equal(2))
		})

		It("Must receive empty list", func() {
			pods, err := kubeClient.GetPods(testCtx, "fake")
			Expect(err).To(BeNil())
			Expect(len(pods)).To(Equal(0))
		})
	})
})

// create provided pods via client from provided svc
func createPods(kubeClient *KubeClient, pods ...*coreV1.Pod) {
	for _, pod := range pods {
		err := kubeClient.Create(context.Background(), pod)
		if err != nil {
			Fail(fmt.Sprintf("uable to create pod %s, error: %v", pod.Name, err))
		}
	}
}

// remove all pods via client from provided svc
func removeAllPods(kubeClient *KubeClient) {
	pods := coreV1.PodList{}
	err := kubeClient.List(context.Background(), &pods, k8sCl.InNamespace(testNs))
	if err != nil {
		Fail(fmt.Sprintf("unable to get pods list: %v", err))
	}
	for _, pod := range pods.Items {
		err = kubeClient.Delete(context.Background(), &pod)
		if err != nil {
			Fail(fmt.Sprintf("unable to delete pod: %v", err))
		}
	}
}

var _ = Describe("Working with CRD", func() {
	var (
		k8sclient *KubeClient
		err       error
	)

	BeforeEach(func() {
		k8sclient, err = GetFakeKubeClient(testNs, testLogger)
		Expect(err).To(BeNil())
	})
	AfterEach(func() {
		removeAllCrds(k8sclient)
	})

	Context("Create and read CRDs (volume, AC and drive)", func() {
		It("Should create and read Volume CR", func() {
			err := k8sclient.CreateCR(testCtx, testID, &testVolume)
			Expect(err).To(BeNil())
			rVolume := &vcrd.Volume{}
			err = k8sclient.ReadCR(testCtx, testID, rVolume)
			Expect(err).To(BeNil())
			Expect(rVolume.ObjectMeta.Name).To(Equal(testID))
		})

		It("Should create and read Available Capacity CR", func() {
			err := k8sclient.CreateCR(testCtx, testACName, &testACCR)
			Expect(err).To(BeNil())
			rAC := &accrd.AvailableCapacity{}
			err = k8sclient.ReadCR(testCtx, testACName, rAC)
			Expect(err).To(BeNil())
			Expect(rAC.ObjectMeta.Name).To(Equal(testACName))
		})

		It("Should create and read drive CR", func() {
			err := k8sclient.CreateCR(testCtx, testUUID, &testDriveCR)
			Expect(err).To(BeNil())
			rdrive := &drivecrd.Drive{}
			err = k8sclient.ReadCR(testCtx, testUUID, rdrive)
			Expect(err).To(BeNil())
			Expect(rdrive.ObjectMeta.Name).To(Equal(testUUID))
		})

		It("Should read volumes CR List", func() {
			err := k8sclient.CreateCR(context.Background(), testACName, &testVolume)
			Expect(err).To(BeNil())

			vList := &vcrd.VolumeList{}
			err = k8sclient.ReadList(context.Background(), vList)
			Expect(err).To(BeNil())
			Expect(len(vList.Items)).To(Equal(1))
			Expect(vList.Items[0].Namespace).To(Equal(testNs))
		})

		It("Should read drive CR List", func() {
			err := k8sclient.CreateCR(testCtx, testACName, &testDriveCR)
			Expect(err).To(BeNil())

			dList := &drivecrd.DriveList{}
			err = k8sclient.ReadList(context.Background(), dList)
			Expect(err).To(BeNil())
			Expect(len(dList.Items)).To(Equal(1))
			Expect(dList.Items[0].Namespace).To(Equal(testNs))
		})

		It("Try to read CRD that doesn't exist", func() {
			name := "notexistingcrd"
			ac := accrd.AvailableCapacity{}
			err := k8sclient.ReadCR(testCtx, name, &ac)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("\"%s\" not found", name)))
		})

	})

	Context("Update CR instance", func() {
		It("Should Available Capacity update successfully", func() {
			acCR := testACCR
			err := k8sclient.CreateCR(testCtx, testACName, &acCR)
			Expect(err).To(BeNil())

			newSize := int64(1024 * 105)
			acCR.Spec.Size = newSize

			err = k8sclient.UpdateCR(testCtx, &acCR)
			Expect(err).To(BeNil())
			Expect(acCR.Spec.Size).To(Equal(newSize))

			acCopy := acCR.DeepCopy()
			err = k8sclient.Update(testCtx, &acCR)
			Expect(err).To(BeNil())
			Expect(acCR.Spec).To(Equal(acCopy.Spec))
		})

		It("Should Drive update successfully", func() {
			driveCR := testDriveCR
			err := k8sclient.CreateCR(testCtx, testUUID, &driveCR)
			Expect(err).To(BeNil())

			driveCR.Spec.Health = apiV1.HealthBad

			err = k8sclient.UpdateCR(testCtx, &driveCR)
			Expect(err).To(BeNil())
			Expect(driveCR.Spec.Health).To(Equal(apiV1.HealthBad))

			driveCopy := driveCR.DeepCopy()
			err = k8sclient.Update(testCtx, &driveCR)
			Expect(err).To(BeNil())
			Expect(driveCR.Spec).To(Equal(driveCopy.Spec))
		})
	})

	Context("Delete CR", func() {
		It("AC should be deleted", func() {
			err := k8sclient.CreateCR(testCtx, testUUID, &testACCR)
			Expect(err).To(BeNil())
			acList := accrd.AvailableCapacityList{}

			err = k8sclient.ReadList(testCtx, &acList)
			Expect(err).To(BeNil())
			Expect(len(acList.Items)).To(Equal(1))

			err = k8sclient.DeleteCR(testCtx, &testACCR)
			Expect(err).To(BeNil())

			err = k8sclient.ReadList(testCtx, &acList)
			Expect(err).To(BeNil())
			Expect(len(acList.Items)).To(Equal(0))
		})
		It("Drive should be deleted", func() {
			err := k8sclient.CreateCR(testCtx, testUUID, &testDriveCR)
			Expect(err).To(BeNil())
			driveList := drivecrd.DriveList{}

			err = k8sclient.ReadList(testCtx, &driveList)
			Expect(err).To(BeNil())
			Expect(len(driveList.Items)).To(Equal(1))

			err = k8sclient.DeleteCR(testCtx, &testDriveCR)
			Expect(err).To(BeNil())

			err = k8sclient.ReadList(testCtx, &driveList)
			Expect(err).To(BeNil())
			Expect(len(driveList.Items)).To(Equal(0))
		})

	})
})

var _ = Describe("Constructor methods", func() {
	var (
		k8sclient *KubeClient
		err       error
	)

	BeforeEach(func() {
		k8sclient, err = GetFakeKubeClient(testNs, testLogger)
		Expect(err).To(BeNil())
	})

	Context("ConstructACCR", func() {
		It("Should return right AC CR", func() {
			name := fmt.Sprintf("%s-%s", testApiAC.NodeId, testApiAC.Location)
			constructedCR := k8sclient.ConstructACCR(name, testApiAC)
			Expect(constructedCR.TypeMeta.Kind).To(Equal(testACCR.TypeMeta.Kind))
			Expect(constructedCR.TypeMeta.APIVersion).To(Equal(testACCR.TypeMeta.APIVersion))
			Expect(constructedCR.ObjectMeta.Name).To(Equal(testACCR.ObjectMeta.Name))
			Expect(constructedCR.ObjectMeta.Namespace).To(Equal(testACCR.ObjectMeta.Namespace))
			Expect(constructedCR.Spec).To(Equal(testACCR.Spec))
		})
	})
	Context("ConstructDriveCR", func() {
		It("Should return right Drive CR", func() {
			constructedCR := k8sclient.ConstructDriveCR(testApiDrive.UUID, testApiDrive)
			Expect(constructedCR.TypeMeta.Kind).To(Equal(testDriveCR.TypeMeta.Kind))
			Expect(constructedCR.TypeMeta.APIVersion).To(Equal(testDriveCR.TypeMeta.APIVersion))
			Expect(constructedCR.ObjectMeta.Name).To(Equal(testDriveCR.ObjectMeta.Name))
			Expect(constructedCR.ObjectMeta.Namespace).To(Equal(testDriveCR.ObjectMeta.Namespace))
			Expect(constructedCR.Spec).To(Equal(testDriveCR.Spec))
		})
	})
	Context("ConstructVolumeCR", func() {
		It("Should return right Volume CR", func() {
			constructedCR := k8sclient.ConstructVolumeCR(testApiVolume.Id, testApiVolume)
			Expect(constructedCR.TypeMeta.Kind).To(Equal(testVolumeCR.TypeMeta.Kind))
			Expect(constructedCR.TypeMeta.APIVersion).To(Equal(testVolumeCR.TypeMeta.APIVersion))
			Expect(constructedCR.ObjectMeta.Name).To(Equal(testVolumeCR.ObjectMeta.Name))
			Expect(constructedCR.ObjectMeta.Namespace).To(Equal(testVolumeCR.ObjectMeta.Namespace))
			Expect(constructedCR.Spec).To(Equal(testVolumeCR.Spec))
		})
	})
	Context("ConstructLVGCR", func() {
		It("Should return right LVG CR", func() {
			constructedCR := k8sclient.ConstructLVGCR(testLVGName, testApiLVG)
			Expect(constructedCR.TypeMeta.Kind).To(Equal(testLVGCR.TypeMeta.Kind))
			Expect(constructedCR.TypeMeta.APIVersion).To(Equal(testLVGCR.TypeMeta.APIVersion))
			Expect(constructedCR.ObjectMeta.Name).To(Equal(testLVGCR.ObjectMeta.Name))
			Expect(constructedCR.ObjectMeta.Namespace).To(Equal(testLVGCR.ObjectMeta.Namespace))
			Expect(constructedCR.Spec).To(Equal(testLVGCR.Spec))
		})
	})
})

// remove all crds (volume and ac)
func removeAllCrds(s *KubeClient) {
	var (
		vList  = &vcrd.VolumeList{}
		acList = &accrd.AvailableCapacityList{}
		err    error
	)

	if err = s.ReadList(testCtx, vList); err != nil {
		Fail(fmt.Sprintf("unable to read volume crds list: %v", err))
	}

	if err = s.ReadList(testCtx, acList); err != nil {
		Fail(fmt.Sprintf("unable to read available capacity crds list: %v", err))
	}

	// remove all volume crds
	for _, obj := range vList.Items {
		if err = s.Delete(testCtx, &obj); err != nil {
			Fail(fmt.Sprintf("unable to delete volume crd: %v", err))
		}
	}

	// remove all ac crds
	for _, obj := range acList.Items {
		if err = s.Delete(testCtx, &obj); err != nil {
			Fail(fmt.Sprintf("unable to delete ac crd: %v", err))
		}
	}
}