package manage

// TODO revist once DefaultCacheTemplateXML split into separate package from controllers allowing for re-use between cache_controller and Infinispan pipeline
//func CreateDefaultCache(ctx pipeline.Context) {
//	i := ctx.Instance()
//	log := ctx.Log()
//
//	ispnClient, err := ctx.InfinispanClient()
//	if err != nil {
//		ctx.RetryProcessing(err)
//		return
//	}
//
//	cacheClient := ispnClient.Cache(consts.DefaultCacheName)
//	if existsCache, err := cacheClient.Exists(); err != nil {
//		log.Error(err, "failed to validate default cache for cache service")
//		ctx.RetryProcessing(err)
//		return
//	} else if !existsCache {
//		log.Info("createDefaultCache")
//		defaultXml, err := DefaultCacheTemplateXML(podList.Items[0].Name, infinispan, r.kubernetes, reqLogger)
//		if err != nil {
//			ctx.RetryProcessing(err)
//			return
//		}
//
//		if err = cacheClient.Create(defaultXml, mime.ApplicationXml); err != nil {
//			log.Error(err, "failed to create default cache for cache service")
//			ctx.RetryProcessing(err)
//			return
//		}
//	}
//}
