package provision

import pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"

func ApplyOperatorMeta(ctx pipeline.Context) {
	// 1. Load default labels and annotations in controller
	// 2. Load supported types in controller
	// 3. Pass supported all to pipeline builder so that context is aware. e.g. ctx.IsTypeSupported, ctx.DefaultLabels, ctx.DefaultAnnotations
	// 4. Apply to Infinispan
}
