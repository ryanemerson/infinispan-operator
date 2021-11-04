package cache

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/iancoleman/strcase"
	ispnv1 "github.com/infinispan/infinispan-operator/api/v1"
	v1 "github.com/infinispan/infinispan-operator/api/v1"
	"github.com/infinispan/infinispan-operator/api/v2alpha1"
	tutils "github.com/infinispan/infinispan-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
)

var testKube = tutils.NewTestKubernetes(os.Getenv("TESTING_CONTEXT"))

func TestMain(m *testing.M) {
	tutils.RunOperator(m, testKube)
}

func initCluster(t *testing.T, configListener bool) (*v1.Infinispan, func()) {
	spec := tutils.DefaultSpec(testKube)
	spec.Spec.ConfigListener.Enabled = pointer.BoolPtr(configListener)
	name := strcase.ToKebab(t.Name())
	spec.Name = name
	spec.Labels = map[string]string{"test-name": t.Name()}
	testKube.CreateInfinispan(spec, tutils.Namespace)
	testKube.WaitForInfinispanPods(1, tutils.SinglePodTimeout, spec.Name, tutils.Namespace)

	ispn := testKube.WaitForInfinispanCondition(spec.Name, spec.Namespace, ispnv1.ConditionWellFormed)
	cleanup := func() {
		testKube.CleanNamespaceAndLogOnPanic(tutils.Namespace, spec.Labels)
	}

	if configListener {
		testKube.WaitForDeployment(spec.GetConfigListenerName(), tutils.Namespace)
	}
	return ispn, cleanup
}

func TestCacheCR(t *testing.T) {
	t.Parallel()
	ispn, cleanup := initCluster(t, false)
	defer cleanup()

	test := func(cache *v2alpha1.Cache) {
		testKube.Create(cache)
		hostAddr, client := tutils.HTTPClientAndHost(ispn, testKube)
		testKube.WaitForCacheCondition(cache.Spec.Name, cache.Namespace, v2alpha1.CacheCondition{
			Type:   "Ready",
			Status: "True",
		})
		cacheHelper := tutils.NewCacheHelper(cache.Spec.Name, hostAddr, client)
		cacheHelper.WaitForCacheToExist()
		cacheHelper.TestBasicUsage("testkey", "test-operator")
		testKube.DeleteCache(cache)
		// TODO Ensure caches deleted on the server
	}

	//Test for CacheCR with Templatename
	cache := cacheCR("cache-with-static-template", ispn)
	cache.Spec.TemplateName = "org.infinispan.DIST_SYNC"
	test(cache)

	//Test for CacheCR with TemplateXML
	cache = cacheCR("cache-with-xml-template", ispn)
	cache.Spec.Template = "<infinispan><cache-container><distributed-cache name=\"cache-with-xml-template\" mode=\"SYNC\"><persistence><file-store/></persistence></distributed-cache></cache-container></infinispan>"
	test(cache)
}

func TestUpdateCacheCR(t *testing.T) {
	// 1. Create cache via CR
	// 2. Update mutable attribute in .Spec
	// 3. Verify update propogated to the server
	// 4. Attempt to update immutable attribute
	// 5. Verify Cache Status updated to reflect this is not possible
}

func TestCacheWithServerLifecycle(t *testing.T) {
	t.Parallel()
	ispn, cleanup := initCluster(t, true)
	defer cleanup()

	cacheName := ispn.Name
	yamlTemplate := "localCache:\n  memory:\n    maxCount: \"%d\"\n"
	originalConfig := fmt.Sprintf(yamlTemplate, 100)

	// Create cache via REST
	hostAddr, client := tutils.HTTPClientAndHost(ispn, testKube)
	cacheHelper := tutils.NewCacheHelper(cacheName, hostAddr, client)
	cacheHelper.CreateWithYaml(originalConfig)

	// Assert CR created with owner ref as Infinispan
	cr := testKube.WaitForCacheCondition(cacheName, tutils.Namespace, v2alpha1.CacheCondition{
		Type:   "Ready",
		Status: "True",
	})

	// Assert that the owner reference has been correctly set to the Infinispan CR
	if cr.GetOwnerReferences()[0].UID != ispn.UID {
		panic("Cache has unexpected owner reference")
	}

	// Update cache configuration via REST
	updatedConfig := fmt.Sprintf(yamlTemplate, 50)
	cacheHelper.UpdateWithYaml(updatedConfig)

	// Assert CR spec.Template updated
	testKube.WaitForCacheState(cacheName, tutils.Namespace, func(cache *v2alpha1.Cache) bool {
		return cache.Spec.Template == updatedConfig
	})

	// Delete cache via REST
	cacheHelper.Delete()

	// Assert CR deleted
	err := wait.Poll(10*time.Millisecond, tutils.MaxWaitTimeout, func() (bool, error) {
		return !testKube.AssertK8ResourceExists(cacheName, tutils.Namespace, &v2alpha1.Cache{}), nil
	})
	tutils.ExpectNoError(err)
}

func TestStaticServerCacheCR(t *testing.T) {
	// TODO
	// 1. Ensure that a Cache CR is created for static caches
	// 2. Ensure that deleting the CR does not delete the runtime cache?
}

func TestCacheReconcilliationWithXML(t *testing.T) {
	// TODO
	// 1. Create Cache CR with XML template
	// 2. Update cache via REST
	// 3. Ensure that CR is updated and returned template is also in XML
}

func TestCacheReconcilliationWithJSON(t *testing.T) {
	// TODO
	// 1. Create Cache CR with XML template
	// 2. Update cache via REST
	// 3. Ensure that CR is updated and returned template is also in XML
}

func cacheCR(cacheName string, i *v1.Infinispan) *v2alpha1.Cache {
	return &v2alpha1.Cache{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "infinispan.org/v2alpha1",
			Kind:       "Cache",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cacheName,
			Namespace: i.Namespace,
		},
		Spec: v2alpha1.CacheSpec{
			ClusterName: i.Name,
			Name:        cacheName,
		},
	}
}
