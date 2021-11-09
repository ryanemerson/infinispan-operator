package v13

import (
	"fmt"
	"net/http"
	"strings"

	httpClient "github.com/infinispan/infinispan-operator/pkg/http"
	"github.com/infinispan/infinispan-operator/pkg/mime"
)

const ClusterPath = BasePath + "/cluster"

type cluster struct {
	httpClient.HttpClient
}

func (c *cluster) GracefulShutdown() (err error) {
	rsp, reason, err := c.Post(ClusterPath+"?action=stop", "", nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, reason, err, "during graceful shutdown", http.StatusNoContent)
	return err
}

// GracefulShutdownTask (ISPN-13141) uploads custom task to perform graceful shutdown that does not fail on cache errors
// This task calls Cache#shutdown which disables rebalancing on the cache before stopping it
func (c *cluster) GracefulShutdownTask() (err error) {
	scriptName := "___org.infinispan.operator.gracefulshutdown.js"
	url := fmt.Sprintf("%s/tasks/%s", BasePath, scriptName)
	headers := map[string]string{"Content-Type": string(mime.TextPlain)}
	task := `
	/* mode=local,language=javascript */
	print("Executing operator shutdown task");
	var System = Java.type("java.lang.System");
	var HashSet = Java.type("java.util.HashSet");
	var InternalCacheRegistry = Java.type("org.infinispan.registry.InternalCacheRegistry");

	var stdErr = System.err;
	var icr = cacheManager.getGlobalComponentRegistry().getComponent(InternalCacheRegistry.class);

	var cacheNames = cacheManager.getCacheNames();
	shutdown(cacheNames);

	var internalCaches = new HashSet(icr.getInternalCacheNames());
	/* The ___script_cache is included in both getCacheNames() and getInternalCacheNames so prevent repeated shutdown calls */
	internalCaches.removeAll(cacheNames);
	shutdown(internalCaches);

	function shutdown(cacheNames) {
	   var it = cacheNames.iterator();
	   while (it.hasNext()) {
			 name = it.next();
			 print("Shutting down cache " + name);
			 try {
				cacheManager.getCache(name).shutdown();
			 } catch (err) {
				stdErr.println("Encountered error trying to shutdown cache " + name + ": " + err);
			 }
	   }
	}
	`
	// Remove all new lines to prevent a "100 continue" response
	task = strings.ReplaceAll(task, "\n", "")

	rsp, reason, err := c.Post(url, task, headers)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	if err = httpClient.ValidateResponse(rsp, reason, err, "Uploading GracefulShutdownTask", http.StatusOK); err != nil {
		return
	}

	rsp, reason, err = c.Post(url+"?action=exec", "", nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, reason, err, "Executing GracefulShutdownTask", http.StatusOK)
	return
}
