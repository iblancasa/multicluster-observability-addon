package handlers

import (
	"context"

	"github.com/ViaQ/logerr/v2/kverrors"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/addon/authentication"
	"github.com/rhobs/multicluster-observability-addon/internal/opentelemetry/manifests"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AnnotationCAToInject = "opentelemetry.mcoa.openshift.io/ca"
)

func BuildOptions(ck8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn, adoc *addonapiv1alpha1.AddOnDeploymentConfig) (manifests.Options, error) {
	resources := manifests.Options{
		AddOnDeploymentConfig: adoc,
		ClusterName:           mcAddon.Namespace,
	}

	otelCol, err := GetOpenTelemetryCollector(ck8s, mcAddon)
	if err != nil {
		return resources, err
	}
	resources.OpenTelemetryCollector = otelCol
	klog.Info("OpenTelemetry Collector template found")

	var authCM *corev1.ConfigMap = nil
	var caSecret *corev1.Secret = nil

	for _, config := range mcAddon.Spec.Configs {
		switch config.ConfigGroupResource.Resource {
		case addon.ConfigMapResource:
			cm, isOtelConfig := GetConfigMap(ck8s, config)
			if !isOtelConfig {
				continue
			}
			if isAuthCM(cm) {
				if authCM != nil {
					klog.Warningf(
						"auth ConfigMap already set to %s/%s. new ConfigMap %s/%s",
						authCM.Namespace, authCM.Name,
						cm.Namespace, cm.Name,
					)
				}
				authCM = cm
				klog.Infof("auth ConfigMap set: %s/%s", authCM.Namespace, authCM.Name)
				continue
			}
			resources.ConfigMaps = append(resources.ConfigMaps, *cm)
		case addon.SecretResource:
			secret, isOtelConfig := GetSecret(ck8s, config)
			if !isOtelConfig {
				continue
			}
			if isCASecret(secret) {
				if caSecret != nil {
					klog.Warningf(
						"CA secret already set to %s/%s. new Secret %s/%s",
						caSecret.Namespace, caSecret.Name,
						secret.Namespace, secret.Name,
					)
				}
				caSecret = secret
				klog.Infof("CA secret set: %s/%s", caSecret.Namespace, caSecret.Name)
				continue
			}
		}
	}

	ctx := context.Background()
	authConfig := manifests.AuthDefaultConfig
	authConfig.MTLSConfig.CommonName = mcAddon.Namespace
	if caSecret == nil {
		klog.Warning("no CA was found")
	} else if len(caSecret.Data) > 0 {
		if ca, ok := caSecret.Data["ca.crt"]; ok {
			authConfig.MTLSConfig.CAToInject = string(ca)
		} else {
			return resources, kverrors.New("missing ca bundle in secret", "key", "ca.crt")
		}
	}

	if authCM != nil {
		secretsProvider, err := authentication.NewSecretsProvider(ck8s, mcAddon.Namespace, addon.OpenTelemetry, authConfig)
		if err != nil {
			return resources, err
		}

		targetsSecret, err := secretsProvider.GenerateSecrets(ctx, authentication.BuildAuthenticationMap(authCM.Data))
		if err != nil {
			return resources, err
		}

		resources.Secrets, err = secretsProvider.FetchSecrets(ctx, targetsSecret, manifests.AnnotationTargetOutputName)
		if err != nil {
			return resources, err
		}
	}

	return resources, nil
}
