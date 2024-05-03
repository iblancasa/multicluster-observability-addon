package reconciler

import (
	"context"
	"fmt"
	"time"

	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/mcoa"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type OTELWatcher struct {
	Client client.Client
	MCOA   *mcoa.MCOA
	Cancel context.CancelCauseFunc
}

func (ow *OTELWatcher) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	klog.Info("========================Start reconciliation===============================================")
	otelCol := otelv1alpha1.OpenTelemetryCollector{}
	if err := ow.Client.Get(ctx, req.NamespacedName, &otelCol); err != nil {
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	// TODO: Check if any MCOA label is there

	ow.Cancel(fmt.Errorf("Reconciliation triggered"))

	err := ow.MCOA.Init()
	if err != nil {
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	mcoaCtx, cancel := context.WithCancelCause(context.Background())
	ow.Cancel = cancel
	go ow.MCOA.Start(mcoaCtx)

	return reconcile.Result{}, nil
}
