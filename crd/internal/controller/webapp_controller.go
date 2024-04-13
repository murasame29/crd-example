/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	viewv1alpha1 "github.com/murasame29/webapp/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsv1apply "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	metav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	metav1apply "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// WebappReconciler reconciles a Webapp object
type WebappReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=view.webapp.github.io,resources=webapps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=view.webapp.github.io,resources=webapps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=view.webapp.github.io,resources=webapps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Webapp object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.2/pkg/reconcile
func (r *WebappReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var webapp viewv1alpha1.Webapp
	err := r.Get(ctx, req.NamespacedName, &webapp)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		logger.Error(err, "unable to get web app", "name", req.NamespacedName)
		return ctrl.Result{}, err
	}

	logger.Info("reconciling deployment", "name", req.NamespacedName)
	if err := r.reconcileCreateDeployment(ctx, webapp); err != nil {
		logger.Error(err, "unable to reconcile deployment", "name", req.NamespacedName)
		return ctrl.Result{}, err
	}

	logger.Info("reconciling service", "name", req.NamespacedName)
	if err := r.reconcileCreateService(ctx, webapp); err != nil {
		logger.Error(err, "unable to reconcile service", "name", req.NamespacedName)
		return ctrl.Result{}, err
	}

	logger.Info("webapp reconciled", "name", webapp.Name)

	return ctrl.Result{}, nil
}

func (r *WebappReconciler) reconcileCreateDeployment(ctx context.Context, webapp viewv1alpha1.Webapp) error {
	logger := log.FromContext(ctx)

	deploymentName := webapp.Name

	owner, err := controllerReference(webapp, r.Scheme)
	if err != nil {
		return err
	}

	dep := appsv1apply.Deployment(deploymentName, webapp.Namespace).
		WithOwnerReferences(owner).
		WithLabels(map[string]string{
			"app": webapp.Name,
		}).WithSpec(
		appsv1apply.
			DeploymentSpec().
			WithReplicas(1).
			WithSelector(
				metav1.LabelSelector().
					WithMatchLabels(map[string]string{"app": webapp.Name}),
			).WithTemplate(
			corev1apply.PodTemplateSpec().
				WithLabels(map[string]string{"app": webapp.Name}).
				WithSpec(
					corev1apply.PodSpec().
						WithContainers(
							corev1apply.Container().
								WithName("webapp").
								WithImage("harbor.seafood-dev.com/test/webapp:latest").
								WithVolumeMounts(
									corev1apply.VolumeMount().
										WithMountPath("/app/html").
										WithName("html")).
								WithPorts(
									corev1apply.ContainerPort().
										WithContainerPort(8080)),
						).WithVolumes(
						corev1apply.Volume().
							WithName("html").
							WithHostPath(
								corev1apply.HostPathVolumeSource().
									WithPath(webapp.Spec.VolumeMountPath))))),
	)

	logger.Info("creating deployment", "name", deploymentName)
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dep)
	if err != nil {
		logger.Error(err, "unable to convert deployment to unstructured", "name", deploymentName)
		return err
	}

	patch := &unstructured.Unstructured{
		Object: obj,
	}

	var current appsv1.Deployment
	err = r.Get(ctx, client.ObjectKey{Namespace: webapp.Namespace, Name: deploymentName}, &current)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	currApplyConfig, err := appsv1apply.ExtractDeployment(&current, "crd-controller")
	if err != nil {
		logger.Info("unable to extract deployment", "name", deploymentName)
		return err
	}

	if equality.Semantic.DeepEqual(dep, currApplyConfig) {
		logger.Info("deployment already exists", "name", deploymentName)
		return nil
	}

	err = r.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "crd-controller",
		Force:        pointer.Bool(true),
	})

	if err != nil {
		logger.Info("unable to patch deployment", "name", deploymentName)
		return err
	}

	logger.Info("deployment updated", "name", deploymentName)

	return nil
}

func controllerReference(webapp viewv1alpha1.Webapp, scheme *runtime.Scheme) (*metav1apply.OwnerReferenceApplyConfiguration, error) {
	gvk, err := apiutil.GVKForObject(&webapp, scheme)
	if err != nil {
		return nil, err
	}

	ref := metav1apply.OwnerReference().
		WithAPIVersion(gvk.GroupVersion().String()).
		WithKind(gvk.Kind).
		WithName(webapp.Name).
		WithUID(webapp.UID).
		WithBlockOwnerDeletion(true).
		WithController(true)

	return ref, nil
}

func (r *WebappReconciler) reconcileCreateService(ctx context.Context, webapp viewv1alpha1.Webapp) error {

	srvName := "webapp-service-" + webapp.Name

	owner, err := controllerReference(webapp, r.Scheme)
	if err != nil {
		return err
	}

	srv := corev1apply.Service(srvName, webapp.Namespace).
		WithOwnerReferences(owner).
		WithLabels(map[string]string{"app": webapp.Name}).
		WithSpec(corev1apply.ServiceSpec().
			WithSelector(map[string]string{"app": webapp.Name}).
			WithType("NodePort").
			WithPorts(corev1apply.ServicePort().
				WithPort(8080).
				WithTargetPort(intstr.FromInt(8080))))

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(srv)
	if err != nil {
		return err
	}

	patch := &unstructured.Unstructured{
		Object: obj,
	}

	var current corev1.Service
	err = r.Get(ctx, client.ObjectKey{Namespace: webapp.Namespace, Name: srvName}, &current)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	currApplyConfig, err := corev1apply.ExtractService(&current, "crd-controller")
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(srv, currApplyConfig) {
		return nil
	}

	err = r.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "crd-controller",
		Force:        pointer.Bool(true),
	})

	if err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WebappReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&viewv1alpha1.Webapp{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
