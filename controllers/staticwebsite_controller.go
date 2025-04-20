package controller

import (
	"context"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	websitesv1alpha1 "static-website-operator/api/v1alpha1"
)

// StaticWebsiteReconciler reconciles a StaticWebsite object
type StaticWebsiteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=websites.example.com,resources=staticwebsites,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=websites.example.com,resources=staticwebsites/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=websites.example.com,resources=staticwebsites/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *StaticWebsiteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling StaticWebsite", "Request.Namespace", req.Namespace, "Request.Name", req.Name)

	// Fetch the StaticWebsite instance
	website := &websitesv1alpha1.StaticWebsite{}
	err := r.Get(ctx, req.NamespacedName, website)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			logger.Info("StaticWebsite resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get StaticWebsite")
		return ctrl.Result{}, err
	}

	// Set the initial status if not set
	if website.Status.Phase == "" {
		website.Status.Phase = "Pending"
		if err := r.Status().Update(ctx, website); err != nil {
			logger.Error(err, "Failed to update StaticWebsite status")
			return ctrl.Result{}, err
		}
	}

	// Check if PVC is required and create it if needed
	var pvc *corev1.PersistentVolumeClaim
	if website.Spec.Storage != nil {
		// Define a new PVC
		pvc = r.pvcForWebsite(website)
		// Check if the PVC already exists
		foundPVC := &corev1.PersistentVolumeClaim{}
		err = r.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, foundPVC)
		if err != nil && errors.IsNotFound(err) {
			logger.Info("Creating a new PVC", "PVC.Namespace", pvc.Namespace, "PVC.Name", pvc.Name)
			err = r.Create(ctx, pvc)
			if err != nil {
				logger.Error(err, "Failed to create new PVC", "PVC.Namespace", pvc.Namespace, "PVC.Name", pvc.Name)
				return ctrl.Result{}, err
			}
		} else if err != nil {
			logger.Error(err, "Failed to get PVC")
			return ctrl.Result{}, err
		}
	}

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: website.Name, Namespace: website.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForWebsite(website, pvc)
		logger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			logger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Update the status
		website.Status.Phase = "Creating"
		if err := r.Status().Update(ctx, website); err != nil {
			logger.Error(err, "Failed to update StaticWebsite status")
			return ctrl.Result{}, err
		}
	} else if err != nil {
		logger.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	} else {
		// Ensure the deployment spec is as expected
		desiredDep := r.deploymentForWebsite(website, pvc)
		// Update the found deployment with the desired spec if needed
		if !reflect.DeepEqual(found.Spec.Template.Spec.Containers[0].Image, desiredDep.Spec.Template.Spec.Containers[0].Image) ||
			!reflect.DeepEqual(found.Spec.Replicas, desiredDep.Spec.Replicas) ||
			!reflect.DeepEqual(found.Spec.Template.Spec.Containers[0].Resources, desiredDep.Spec.Template.Spec.Containers[0].Resources) {

			found.Spec.Template.Spec.Containers[0].Image = desiredDep.Spec.Template.Spec.Containers[0].Image
			found.Spec.Replicas = desiredDep.Spec.Replicas
			found.Spec.Template.Spec.Containers[0].Resources = desiredDep.Spec.Template.Spec.Containers[0].Resources

			if err := r.Update(ctx, found); err != nil {
				logger.Error(err, "Failed to update Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
				return ctrl.Result{}, err
			}
			logger.Info("Updated Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
		}
	}

	// Check if the service already exists, if not create a new one
	service := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: website.Name, Namespace: website.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		// Define a new service
		svc := r.serviceForWebsite(website)
		logger.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		err = r.Create(ctx, svc)
		if err != nil {
			logger.Error(err, "Failed to create new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
	} else if err != nil {
		logger.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	// Update the status
	if found.Status.ReadyReplicas > 0 && website.Status.Phase != "Running" {
		website.Status.Phase = "Running"
		website.Status.AvailableReplicas = found.Status.ReadyReplicas
		website.Status.URL = fmt.Sprintf("http://%s.%s.svc.cluster.local", website.Name, website.Namespace)
		if err := r.Status().Update(ctx, website); err != nil {
			logger.Error(err, "Failed to update StaticWebsite status")
			return ctrl.Result{}, err
		}
	} else if found.Status.ReadyReplicas != website.Status.AvailableReplicas {
		website.Status.AvailableReplicas = found.Status.ReadyReplicas
		if err := r.Status().Update(ctx, website); err != nil {
			logger.Error(err, "Failed to update StaticWebsite status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// deploymentForWebsite returns a website Deployment object
func (r *StaticWebsiteReconciler) deploymentForWebsite(website *websitesv1alpha1.StaticWebsite, pvc *corev1.PersistentVolumeClaim) *appsv1.Deployment {
	labels := map[string]string{
		"app":        "staticwebsite",
		"controller": website.Name,
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      website.Name,
			Namespace: website.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &website.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: website.Spec.Image,
						Name:  "website",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 80,
							Name:          "http",
						}},
						Resources: convertResources(website.Spec.Resources),
					}},
				},
			},
		},
	}

	// If a PVC is provided, mount it to the container
	if pvc != nil {
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      "website-content",
				MountPath: website.Spec.Storage.MountPath,
			},
		}
		dep.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "website-content",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
					},
				},
			},
		}
	}

	// Set the owner reference for garbage collection
	controllerutil.SetControllerReference(website, dep, r.Scheme)
	return dep
}

// serviceForWebsite returns a website Service object
func (r *StaticWebsiteReconciler) serviceForWebsite(website *websitesv1alpha1.StaticWebsite) *corev1.Service {
	labels := map[string]string{
		"app":        "staticwebsite",
		"controller": website.Name,
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      website.Name,
			Namespace: website.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Port:       80,
				TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 80},
				Name:       "http",
			}},
		},
	}

	// Set the owner reference for garbage collection
	controllerutil.SetControllerReference(website, svc, r.Scheme)
	return svc
}

// pvcForWebsite returns a PVC for the website's content
func (r *StaticWebsiteReconciler) pvcForWebsite(website *websitesv1alpha1.StaticWebsite) *corev1.PersistentVolumeClaim {
	storageClassName := website.Spec.Storage.StorageClassName

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-content", website.Name),
			Namespace: website.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(website.Spec.Storage.Size),
				},
			},
		},
	}

	if storageClassName != "" {
		pvc.Spec.StorageClassName = &storageClassName
	}

	// Set the owner reference for garbage collection
	controllerutil.SetControllerReference(website, pvc, r.Scheme)
	return pvc
}

// Convert our custom resources type to k8s ResourceRequirements
func convertResources(resources websitesv1alpha1.ResourceRequirements) corev1.ResourceRequirements {
	k8sResources := corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}

	// Convert limits
	for name, value := range resources.Limits {
		k8sResources.Limits[corev1.ResourceName(name)] = resource.MustParse(value)
	}

	// Convert requests
	for name, value := range resources.Requests {
		k8sResources.Requests[corev1.ResourceName(name)] = resource.MustParse(value)
	}

	return k8sResources
}

// SetupWithManager sets up the controller with the Manager.
func (r *StaticWebsiteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&websitesv1alpha1.StaticWebsite{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
