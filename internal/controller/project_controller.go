package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	projectsv1 "github.com/Matheusd3v/kubernetes-project-operator/api/v1"
)

// ProjectReconciler reconciles a Project object
type ProjectReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=projects.home.lab,resources=projects,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=projects.home.lab,resources=projects/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete

func (r *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Project instance
	var project projectsv1.Project
	if err := r.Get(ctx, req.NamespacedName, &project); err != nil {
		if errors.IsNotFound(err) {
			// Project not found (was deleted)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 1. Create Namespace if not exists
	ns := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: project.Spec.NamespaceName}, ns)
	if err != nil && errors.IsNotFound(err) {
		ns = &corev1.Namespace{
			ObjectMeta: ctrl.ObjectMeta{
				Name: project.Spec.NamespaceName,
				Labels: map[string]string{
					"owner": project.Spec.Owner,
				},
			},
		}
		if err := r.Create(ctx, ns); err != nil {
			logger.Error(err, "Failed to create Namespace")
			return ctrl.Result{}, err
		}
		logger.Info("Created Namespace", "namespace", project.Spec.NamespaceName)
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// 2. Optionally create RBAC
	if project.Spec.EnableRBAC {
		// Create Role
		role := &rbacv1.Role{
			ObjectMeta: ctrl.ObjectMeta{
				Name:      "developer-role",
				Namespace: project.Spec.NamespaceName,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods", "services"},
					Verbs:     []string{"get", "list", "create", "update", "delete"},
				},
			},
		}
		_ = controllerutil.SetControllerReference(&project, role, r.Scheme)
		if err := r.Create(ctx, role); err != nil && !errors.IsAlreadyExists(err) {
			logger.Error(err, "Failed to create Role")
			return ctrl.Result{}, err
		}

		// Create RoleBinding
		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: ctrl.ObjectMeta{
				Name:      "developer-binding",
				Namespace: project.Spec.NamespaceName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "Group",
					Name: project.Spec.Owner,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "Role",
				Name: "developer-role",
				APIGroup: "rbac.authorization.k8s.io",
			},
		}
		_ = controllerutil.SetControllerReference(&project, roleBinding, r.Scheme)
		if err := r.Create(ctx, roleBinding); err != nil && !errors.IsAlreadyExists(err) {
			logger.Error(err, "Failed to create RoleBinding")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&projectsv1.Project{}).
		Complete(r)
}
