package cache

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/iancoleman/strcase"
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

	ispn := testKube.WaitForInfinispanCondition(spec.Name, spec.Namespace, v1.ConditionWellFormed)
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
		testKube.WaitForCacheConditionReady(cache.Spec.Name, cache.Namespace)
		hostAddr, client := tutils.HTTPClientAndHost(ispn, testKube)
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
	t.Parallel()
	ispn, cleanup := initCluster(t, false)
	defer cleanup()
	cacheName := ispn.Name
	originalYaml := "localCache:\n  memory:\n    maxCount: 10\n"

	// Create Cache CR with Yaml template
	cr := cacheCR(cacheName, ispn)
	cr.Spec.Template = originalYaml
	testKube.Create(cr)
	cr = testKube.WaitForCacheConditionReady(cacheName, tutils.Namespace)

	validUpdateYaml := strings.Replace(cr.Spec.Template, "10", "50", 1)
	cr.Spec.Template = validUpdateYaml
	testKube.Update(cr)

	// Assert CR spec.Template updated
	testKube.WaitForCacheState(cacheName, tutils.Namespace, func(cache *v2alpha1.Cache) bool {
		return cache.Spec.Template == validUpdateYaml
	})

	// Assert CR remains ready
	cr = testKube.WaitForCacheConditionReady(cacheName, tutils.Namespace)

	invalidUpdateYaml := `distributedCache: {}`
	cr.Spec.Template = invalidUpdateYaml
	testKube.Update(cr)

	// Assert CR spec.Template updated
	testKube.WaitForCacheState(cacheName, tutils.Namespace, func(cache *v2alpha1.Cache) bool {
		return cache.Spec.Template == invalidUpdateYaml
	})

	// Wait for the Cache CR to become unready as the spec.Template cannot be reconciled with the server
	testKube.WaitForCacheCondition(cacheName, tutils.Namespace, v2alpha1.CacheCondition{
		Type:   v2alpha1.CacheConditionReady,
		Status: metav1.ConditionFalse,
	})
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
	cr := testKube.WaitForCacheConditionReady(cacheName, tutils.Namespace)

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

func TestCacheWithXML(t *testing.T) {
	t.Parallel()
	ispn, cleanup := initCluster(t, true)
	defer cleanup()
	cacheName := ispn.Name
	originalXml := `<local-cache><memory max-count="100"/></local-cache>`

	// Create Cache CR with XML template
	cr := cacheCR(cacheName, ispn)
	cr.Spec.Template = originalXml
	testKube.Create(cr)
	testKube.WaitForCacheConditionReady(cacheName, tutils.Namespace)

	// Wait for 2nd generation of Cache CR with server formatting
	cr = testKube.WaitForCacheState(cacheName, tutils.Namespace, func(cache *v2alpha1.Cache) bool {
		return cache.ObjectMeta.Generation == 2
	})

	// Assert CR spec.Template updated and returned template is in the XML format
	if cr.Spec.Template == originalXml {
		panic("Expected CR template format to be different to original")
	}

	if !strings.Contains(cr.Spec.Template, `memory max-count="100"`) {
		panic("Unexpected cr.Spec.Template content")
	}

	// Update cache via REST
	hostAddr, client := tutils.HTTPClientAndHost(ispn, testKube)
	cacheHelper := tutils.NewCacheHelper(cacheName, hostAddr, client)
	updatedXml := strings.Replace(cr.Spec.Template, "100", "50", 1)
	cacheHelper.UpdateWithXML(updatedXml)

	// Assert CR spec.Template updated and returned template is in the XML format
	testKube.WaitForCacheState(cacheName, tutils.Namespace, func(cache *v2alpha1.Cache) bool {
		return cache.Spec.Template == updatedXml
	})
}

func TestCacheWithJSON(t *testing.T) {
	t.Parallel()
	ispn, cleanup := initCluster(t, true)
	defer cleanup()
	cacheName := ispn.Name
	originalJson := `{"local-cache":{"memory":{"max-count":"100"}}}`

	// Create Cache CR with XML template
	cr := cacheCR(cacheName, ispn)
	cr.Spec.Template = originalJson
	testKube.Create(cr)
	testKube.WaitForCacheConditionReady(cacheName, tutils.Namespace)

	// Wait for 2nd generation of Cache CR with server formatting
	cr = testKube.WaitForCacheState(cacheName, tutils.Namespace, func(cache *v2alpha1.Cache) bool {
		return cache.ObjectMeta.Generation == 2
	})

	// Assert CR spec.Template updated and returned template is in the JSON format
	if cr.Spec.Template == originalJson {
		panic("Expected CR template format to be different to original")
	}

	if !strings.Contains(cr.Spec.Template, `"100"`) {
		panic("Unexpected cr.Spec.Template content")
	}

	// Update cache via REST
	hostAddr, client := tutils.HTTPClientAndHost(ispn, testKube)
	cacheHelper := tutils.NewCacheHelper(cacheName, hostAddr, client)
	updatedJson := strings.Replace(cr.Spec.Template, "100", "50", 1)
	cacheHelper.UpdateWithJSON(updatedJson)

	// Assert CR spec.Template updated and returned template is in the JSON format
	testKube.WaitForCacheState(cacheName, tutils.Namespace, func(cache *v2alpha1.Cache) bool {
		return cache.Spec.Template == updatedJson
	})
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
