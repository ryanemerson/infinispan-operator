package context

import pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"

type flowCtrl struct {
	retry bool
	stop  bool
	err   error
}

func (i *flowCtrl) FlowStatus() pipeline.FlowStatus {
	return pipeline.FlowStatus{
		Retry: i.retry,
		Stop:  i.stop,
		Err:   i.err,
	}
}

func (i *flowCtrl) RetryProcessing(reason error) {
	i.retry = true
	i.stop = true
	i.err = reason
}

func (i *flowCtrl) Error(err error) {
	i.err = err
}

func (i *flowCtrl) StopProcessing() {
	i.stop = true
}
