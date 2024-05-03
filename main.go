package main

import (
	"context"
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	loggingapis "github.com/openshift/cluster-logging-operator/apis"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/mcoa"
	"github.com/rhobs/multicluster-observability-addon/internal/reconciler"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	utilflag "k8s.io/component-base/cli/flag"
	logs "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
	cmdfactory "open-cluster-management.io/addon-framework/pkg/cmd/factory"
	"open-cluster-management.io/addon-framework/pkg/version"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano()) // nolint:staticcheck

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.AddFlags(logs.NewLoggingConfiguration(), pflag.CommandLine)

	command := newCommand()
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func newCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "multicluster-observability-addon",
		Short: "multicluster-observability-addon",
		Run: func(cmd *cobra.Command, _ []string) {
			if err := cmd.Help(); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}
			os.Exit(1)
		},
	}

	if v := version.Get().String(); len(v) == 0 {
		cmd.Version = "<unknown>"
	} else {
		cmd.Version = v
	}

	cmd.AddCommand(newControllerCommand())

	return cmd
}

func newControllerCommand() *cobra.Command {
	cmd := cmdfactory.
		NewControllerCommandConfig("multicluster-observability-addon-controller", version.Get(), runController).
		NewCommand()
	cmd.Use = "controller"
	cmd.Short = "Start the addon controller"

	return cmd
}

func addApisToScheme() error {
	// Necessary to reconcile ClusterLogging and ClusterLogForwarder
	err := loggingapis.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	// Necessary to reconcile OpenTelemetryCollectors
	err = otelv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	// Necessary to reconcile OperatorGroups
	err = operatorsv1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	// Necessary to reconcile Subscriptions
	err = operatorsv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	// Necessary for metrics to get Routes hosts
	if err = routev1.Install(scheme.Scheme); err != nil {
		return err
	}

	// Necessary to reconcile cert-manager resources
	err = certmanagerv1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}

	// Reconcile AddOnDeploymentConfig
	err = addonapiv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	return nil
}

func runController(ctx context.Context, kubeConfig *rest.Config) error {
	err := addApisToScheme()
	if err != nil {
		return err
	}

	registrationOption := addon.NewRegistrationOption(utilrand.String(5))

	mcoa, err := mcoa.NewMCOA(kubeConfig, registrationOption)
	if err != nil {
		return err
	}

	cfg, err := config.GetConfig()
	if err != nil {
		fmt.Println("Error getting config:", err)
		os.Exit(1)
	}

	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		fmt.Println("Failed to create manager:", err)
		os.Exit(1)
	}

	mcoaCtx, cancel := context.WithCancelCause(ctx)

	watcher := &reconciler.OTELWatcher{
		Client: mgr.GetClient(),
		MCOA:   mcoa,
		Cancel: cancel,
	}

	ctrl.SetLogger(klog.LoggerWithName(klog.Background(), "addon"))

	ctrl.NewControllerManagedBy(mgr).
		For(&otelv1alpha1.OpenTelemetryCollector{}).
		Complete(watcher)

	go mcoa.Start(mcoaCtx)

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fmt.Println("Failed to start manager:", err)
		os.Exit(1)
	}

	<-ctx.Done()

	return nil
}
