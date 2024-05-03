package mcoa

import (
	"context"
	"fmt"

	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	addonhelm "github.com/rhobs/multicluster-observability-addon/internal/addon/helm"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonv1alpha1client "open-cluster-management.io/api/client/addon/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type MCOA struct {
	kubeConfig         *rest.Config
	registrationOption *agent.RegistrationOption
	mgr                *addonmanager.AddonManager
	ctx                *context.Context
}

func NewMCOA(kubeConfig *rest.Config, registrationOption *agent.RegistrationOption) (*MCOA, error) {
	mcoa := &MCOA{
		kubeConfig:         kubeConfig,
		registrationOption: registrationOption,
	}
	err := mcoa.Init()
	if err != nil {
		return nil, err
	}

	return mcoa, nil
}

func (m *MCOA) Init() error {
	addonClient, err := addonv1alpha1client.NewForConfig(m.kubeConfig)
	if err != nil {
		return err
	}
	mgr, err := addonmanager.New(m.kubeConfig)
	if err != nil {
		return err
	}
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(addonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)

	httpClient, err := rest.HTTPClientFor(m.kubeConfig)
	if err != nil {
		return err
	}

	mapper, err := apiutil.NewDynamicRESTMapper(m.kubeConfig, httpClient)
	if err != nil {
		return err
	}

	opts := client.Options{
		Scheme:     scheme.Scheme,
		Mapper:     mapper,
		HTTPClient: httpClient,
	}

	k8sClient, err := client.New(m.kubeConfig, opts)
	if err != nil {
		return err
	}

	mcoaAgentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, "manifests/charts/mcoa").
		WithConfigGVRs(
			schema.GroupVersionResource{Version: "v1", Resource: "secrets"},
			schema.GroupVersionResource{Version: "v1", Resource: "configmaps"},
			schema.GroupVersionResource{Version: "v1", Group: "logging.openshift.io", Resource: "clusterlogforwarders"},
			schema.GroupVersionResource{Version: "v1alpha1", Group: "opentelemetry.io", Resource: "opentelemetrycollectors"},
			utils.AddOnDeploymentConfigGVR,
		).
		WithGetValuesFuncs(addonConfigValuesFn, addonhelm.GetValuesFunc(k8sClient)).
		WithAgentRegistrationOption(m.registrationOption).
		WithScheme(scheme.Scheme).
		BuildHelmAgentAddon()
	if err != nil {
		return fmt.Errorf("error builing the agent %v", err)
	}

	err = mgr.AddAgent(mcoaAgentAddon)
	if err != nil {
		return fmt.Errorf("error adding the addon to the agent %v", err)
	}

	m.mgr = &mgr
	return nil
}

func (m *MCOA) Start(ctx context.Context) {
	m.ctx = &ctx
	(*m.mgr).Start(ctx)
}
