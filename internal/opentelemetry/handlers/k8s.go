package handlers

import (
	"context"

	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"
	"github.com/rhobs/multicluster-observability-addon/internal/opentelemetry/manifests"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	opentelemetryCollectorResource = "opentelemetrycollectors"
)

// GetOpenTelemetryCollector get the OpenTelemetry Collector template
func GetOpenTelemetryCollector(k8s client.Client, mcAddon *addonapiv1alpha1.ManagedClusterAddOn) (*otelv1alpha1.OpenTelemetryCollector, error) {
	key := addon.GetObjectKey(mcAddon.Status.ConfigReferences, otelv1alpha1.GroupVersion.Group, opentelemetryCollectorResource)

	klog.Info("Retrieving OpenTelemetry Collector template", "namespace", key)
	otelCol := &otelv1alpha1.OpenTelemetryCollector{}
	if err := k8s.Get(context.Background(), key, otelCol, &client.GetOptions{}); err != nil {
		return otelCol, err
	}
	return otelCol, nil
}

// GetConfigMap gets the configmaps associated to the OpenTelemetry resources. The second second output parameter is true when
// the ConfigMap configures OpenTelemetry stuff
func GetConfigMap(k8s client.Client, config addonapiv1alpha1.AddOnConfig) (*corev1.ConfigMap, bool) {
	klog.Infof("processing cm %s/%s", config.Namespace, config.Name)

	key := client.ObjectKey{Name: config.Name, Namespace: config.Namespace}

	cm := &corev1.ConfigMap{}
	if err := k8s.Get(context.Background(), key, cm, &client.GetOptions{}); err != nil {
		klog.Errorf("there was a problem processing ConfigMap '%s/%s': %s", config.Name, config.Name, err)
		return nil, false
	}

	return cm, isOtelResource(cm)
}

// GetSecret gets the Secrets associated to the OpenTelemetry resources. The second second output parameter is true when
// the Secret configures OpenTelemetry stuff
func GetSecret(k8s client.Client, config addonapiv1alpha1.AddOnConfig) (*corev1.Secret, bool) {
	klog.Infof("processing secret %s/%s", config.Namespace, config.Name)

	key := client.ObjectKey{Name: config.Name, Namespace: config.Namespace}
	secret := &corev1.Secret{}
	if err := k8s.Get(context.Background(), key, secret, &client.GetOptions{}); err != nil {
		klog.Errorf("there was a problem processing Secret '%s/%s': %s", config.Name, config.Name, err)
		return nil, false
	}

	return secret, isOtelResource(secret)
}

// isAuthCM returns true if this is the authentication ConfigMap
func isAuthCM(cm *corev1.ConfigMap) bool {
	// If a ConfigMap doesn't have a target annotation then it's configuring authentication
	_, ok := cm.Annotations[manifests.AnnotationTargetOutputName]
	return !ok
}

// isCASecret returns true if this is the secret containing the CA
func isCASecret(s *corev1.Secret) bool {
	_, ok := s.Annotations[AnnotationCAToInject]
	return ok
}

// isOtelResource returns true if is one OpenTelemetry resource
func isOtelResource(o metav1.Object) bool {
	l := o.GetLabels()
	if signal, ok := l[addon.SignalLabelKey]; !ok || signal != addon.OpenTelemetry.String() {
		return false
	}
	return true
}
