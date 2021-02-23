package upgrade

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/iancoleman/strcase"
	v1 "github.com/infinispan/infinispan-operator/pkg/apis/infinispan/v1"
	"github.com/infinispan/infinispan-operator/pkg/controller/constants"
	ispnctrl "github.com/infinispan/infinispan-operator/pkg/controller/infinispan"
	tutils "github.com/infinispan/infinispan-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CrdPath = "../../../deploy/crds/"
)

var (
	testKube         = tutils.NewTestKubernetes(os.Getenv("TESTING_CONTEXT"))
	upgradeStateFlow = []v1.ConditionType{v1.ConditionUpgrade, v1.ConditionStopping, v1.ConditionWellFormed}
)

// TODO remove and replace with DefaultSpec once Batch PR has been merged as it removes this logic from main_test
var MinimalSpec = v1.Infinispan{
	TypeMeta: tutils.InfinispanTypeMeta,
	ObjectMeta: metav1.ObjectMeta{
		Name: tutils.DefaultClusterName,
	},
	Spec: v1.InfinispanSpec{
		Replicas: 1,
	},
}

// - Re-read breakout notes
//   - https://docs.google.com/document/d/18grmkD_FlWXfNWWTpl5UswApYjA8nIYlff0Ta0rpLrg/edit#heading=h.7txhxlesvyxf

// 0. How to determine that an upgrade is occurring?
//   - If the DEFAULT_IMAGE of the operator install is different to the DEFAULT_IMAGE env on a Cluster POD
//   - How can we test this?
//     - Makefile, if FROM_UPGRADE_VERSION specified, pull tag locally and add to /tmp/infinispan-operator/upgrade
//       - else copy current deploy versions and update DEFAULT_IMAGE to use sha256 of current tag
//       - `docker inspect --format='{{index .RepoDigests 0}}' quay.io/infinispan/cli:12.0`
//     - Install operator using /tmp/infinispan-operator/upgrade
//     - Create cluster and check well formed. Add entries to cache
//     - Install operator using default deploy scripts
//     - Ensure pods restart and no entries in cache

// 1. Create a new StatefulSet for the target cluster based upon the CR spec, wait until the cluster is formed
//   - Create generic method to create StatefulSet in infinispan_controller
//   - ispnCtrl.statefulSetForInfinispan seems to be generic enough

// 2. Retrieve all cache configurations from the cluster in the original Statefulset
//   - /v2/cache-managers/{name}/cache-configs
//     - Returns {"name": ", "configuration":""}

// 3. Append a remote-store configuration to each of the ^ configurations and create cache target cluster:
//   - How best to handle? Simple regex replace?
//   - Could add server addSource method?
//     - Seems overkill

func TestGracefulShutdown(t *testing.T) {
	namespace := tutils.Namespace
	testKube.NewNamespace(namespace)
	// Utilise the sha256 of the current Infinispan image to trick the operator into thinking a upgrade is required
	oldImage := getDockerImageSha()
	tutils.ExpectNoError(os.Setenv("DEFAULT_IMAGE", oldImage))
	stopCh := testKube.InstallAndRunOperator(namespace, CrdPath, false)
	ispn := MinimalSpec.DeepCopy()
	ispn.Name = strcase.ToKebab(t.Name())
	testKube.CreateInfinispan(ispn, namespace)
	testKube.WaitForInfinispanPods(1, tutils.SinglePodTimeout, ispn.Name, namespace)
	assertPodImage(oldImage, ispn)
	close(stopCh)

	// Unset DEFAULT_IMAGE and install the current operator version
	os.Unsetenv("DEFAULT_IMAGE")
	stopCh = testKube.InstallAndRunOperator(namespace, CrdPath, false)
	defer close(stopCh)
	for _, state := range upgradeStateFlow {
		testKube.WaitForInfinispanCondition(ispn.Name, namespace, state)
	}
	assertPodImage(tutils.ExpectedImage, ispn)
}

func assertPodImage(image string, ispn *v1.Infinispan) {
	pods := &corev1.PodList{}
	err := testKube.Kubernetes.ResourcesList(ispn.Namespace, ispnctrl.PodLabels(ispn.Name), pods)
	tutils.ExpectNoError(err)
	for _, pod := range pods.Items {
		if pod.Spec.Containers[0].Image != image {
			tutils.ExpectNoError(fmt.Errorf("upgraded image [%v] in Pod not equal desired cluster image [%v]", pod.Spec.Containers[0].Image, image))
		}
	}
}

func getDockerImageSha() string {
	cmd := exec.Command("docker", "inspect", "--format={{index .RepoDigests 0}}", constants.DefaultOperandImageOpenJDK)
	stdout, err := cmd.Output()

	if err != nil {
		panic(err)
	}
	s := string(stdout)
	return strings.TrimSuffix(s, "\n")
}
