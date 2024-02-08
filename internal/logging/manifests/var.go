package manifests

import (
	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	"github.com/rhobs/multicluster-observability-addon/internal/manifests"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AnnotationTargetOutputName = "logging.mcoa.openshift.io/target-output-name"
	AnnotationCAToInject       = "logging.mcoa.openshift.io/ca"

	subscriptionChannelValueKey = "loggingSubscriptionChannel"
	defaultLoggingVersion       = "stable-5.8"
)

var AuthDefaultConfig = &authentication.Config{
	StaticAuthConfig: manifests.StaticAuthenticationConfig{
		ExistingSecret: client.ObjectKey{
			Name:      "static-authentication",
			Namespace: "open-cluster-management",
		},
	},
	// TODO(JoaoBraveCoding) Implement when support for LokiStack is added
	MTLSConfig: manifests.MTLSConfig{
		CommonName: "",
		Subject: &v1.X509Subject{
			OrganizationalUnits: []string{
				"logging-ocm-addon",
			},
		},
		DNSNames: []string{
			"collector.openshift-logging.svc",
		},
	},
}
