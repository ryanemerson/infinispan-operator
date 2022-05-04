package context

import (
	pipeline "github.com/infinispan/infinispan-operator/pkg/reconcile/pipeline/infinispan"
	"time"
)

type flowCtrl struct {
	retry bool
	stop  bool
	err   error
	delay time.Duration
}

func (i *flowCtrl) FlowStatus() pipeline.FlowStatus {
	return pipeline.FlowStatus{
		Retry: i.retry,
		Stop:  i.stop,
		Err:   i.err,
		Delay: i.delay,
	}
}

func (i *flowCtrl) Requeue(reason error) {
	i.RequeueAfter(0, reason)
}

func (i *flowCtrl) RequeueAfter(delay time.Duration, reason error) {
	i.retry = true
	i.stop = true
	i.err = reason
	i.delay = delay
}

func (i *flowCtrl) Error(err error) {
	i.err = err
}

func (i *flowCtrl) Stop() {
	i.stop = true
}
