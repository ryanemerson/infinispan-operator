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

func (i *flowCtrl) Requeue(err error) {
	i.RequeueAfter(0, err)
}

func (i *flowCtrl) RequeueAfter(delay time.Duration, err error) {
	i.retry = true
	i.delay = delay
	i.Stop(err)
}

func (i *flowCtrl) Stop(err error) {
	i.stop = true
	i.err = err
}
