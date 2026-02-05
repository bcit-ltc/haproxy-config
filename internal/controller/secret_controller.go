package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/example/haproxy-operator/internal/haproxy"
)

const (
	// Label to identify Secrets managed by this operator
	SecretLabelKey   = "haproxy.operator/config"
	SecretLabelValue = "true"

	// Annotation for last applied configuration hash
	LastAppliedHashAnnotation = "haproxy.operator/last-applied-hash"

	// Status annotation
	StatusAnnotation = "haproxy.operator/status"

	// Requeue interval
	RequeueInterval = 5 * time.Minute
)

// SecretReconciler reconciles Secret objects containing HAProxy configuration
type SecretReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	SecretKey string
}

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets/status,verbs=get;update;patch

// Reconcile is the main reconciliation loop
func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("secret", req.NamespacedName)

	// Fetch the Secret
	secret := &corev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, secret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Secret not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Secret")
		return ctrl.Result{}, err
	}

	// Check if Secret has the required label
	if !r.shouldReconcile(secret) {
		log.Info("Secret does not have required label, skipping")
		return ctrl.Result{}, nil
	}

	// Get configuration data
	configData, exists := secret.Data[r.SecretKey]
	if !exists {
		err := fmt.Errorf("key %s not found in Secret", r.SecretKey)
		log.Error(err, "configuration key missing")
		return r.updateStatus(ctx, secret, "Error", err.Error())
	}

	// Parse configuration
	config, err := haproxy.ParseConfig(configData)
	if err != nil {
		log.Error(err, "failed to parse HAProxy configuration")
		return r.updateStatus(ctx, secret, "ParseError", err.Error())
	}

	// Get HAProxy API credentials from Secret
	apiConfig, err := r.getAPIConfig(ctx, secret.Namespace, config)
	if err != nil {
		log.Error(err, "failed to get API configuration")
		return r.updateStatus(ctx, secret, "ConfigError", err.Error())
	}

	// Create HAProxy client
	haproxyClient := haproxy.NewClient(apiConfig)

	// Check current configuration hash
	currentHash := haproxy.HashConfig(string(configData))
	lastAppliedHash := secret.Annotations[LastAppliedHashAnnotation]

	if currentHash == lastAppliedHash {
		log.Info("configuration unchanged, skipping reconciliation")
		return ctrl.Result{RequeueAfter: RequeueInterval}, nil
	}

	log.Info("configuration changed, applying to HAProxy",
		"currentHash", currentHash,
		"lastAppliedHash", lastAppliedHash)

	// Apply configuration to HAProxy
	if err := haproxyClient.ApplyConfiguration(ctx, config); err != nil {
		log.Error(err, "failed to apply configuration to HAProxy")
		return r.updateStatus(ctx, secret, "ApplyError", err.Error())
	}

	log.Info("configuration successfully applied to HAProxy")

	// Update Secret annotations with new hash and status
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[LastAppliedHashAnnotation] = currentHash
	secret.Annotations[StatusAnnotation] = "Applied"
	secret.Annotations["haproxy.operator/last-applied-time"] = time.Now().Format(time.RFC3339)

	if err := r.Update(ctx, secret); err != nil {
		log.Error(err, "failed to update Secret annotations")
		return ctrl.Result{}, err
	}

	log.Info("reconciliation complete")
	return ctrl.Result{RequeueAfter: RequeueInterval}, nil
}

// shouldReconcile checks if the Secret should be reconciled
func (r *SecretReconciler) shouldReconcile(secret *corev1.Secret) bool {
	if secret.Labels == nil {
		return false
	}
	return secret.Labels[SecretLabelKey] == SecretLabelValue
}

// getAPIConfig retrieves HAProxy API configuration from referenced Secret
func (r *SecretReconciler) getAPIConfig(ctx context.Context, namespace string, config *haproxy.Config) (*haproxy.APIConfig, error) {
	log := log.FromContext(ctx)

	if config.APIConfig.SecretRef == "" {
		return nil, fmt.Errorf("apiConfig.secretRef not specified in configuration")
	}

	// Fetch the Secret containing API credentials
	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{
		Name:      config.APIConfig.SecretRef,
		Namespace: namespace,
	}

	if err := r.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", config.APIConfig.SecretRef, err)
	}

	// Extract credentials
	username := string(secret.Data["username"])
	password := string(secret.Data["password"])

	if username == "" || password == "" {
		return nil, fmt.Errorf("username or password not found in secret %s", config.APIConfig.SecretRef)
	}

	log.Info("retrieved API credentials from secret", "secret", config.APIConfig.SecretRef)

	return &haproxy.APIConfig{
		URL:      config.APIConfig.URL,
		Username: username,
		Password: password,
		Insecure: config.APIConfig.Insecure,
	}, nil
}

// updateStatus updates the Secret status annotation
func (r *SecretReconciler) updateStatus(ctx context.Context, secret *corev1.Secret, status, message string) (ctrl.Result, error) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[StatusAnnotation] = status
	secret.Annotations["haproxy.operator/status-message"] = message
	secret.Annotations["haproxy.operator/last-update-time"] = time.Now().Format(time.RFC3339)

	if err := r.Update(ctx, secret); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue after a delay for retries
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.LabelChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
		)).
		Complete(r)
}
