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

	"fmt"

	"strings"

	"github.com/hashicorp/terraform-exec/tfexec"
	"k8s.io/apimachinery/pkg/runtime"
	workerv1 "my.domain/guestbook/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// WorkerReconciler reconciles a Worker object
type WorkerReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	terraformPath string
}

// +kubebuilder:rbac:groups=worker.my.domain,resources=workers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=worker.my.domain,resources=workers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=worker.my.domain,resources=workers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Worker object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *WorkerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	// Fetch the Worker instance
	worker := &workerv1.Worker{}
	if err := r.Get(ctx, req.NamespacedName, worker); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Handle deletion
	if !worker.ObjectMeta.DeletionTimestamp.IsZero() {
		// Run the script or command here
		err := executeTerraformCommandWithVars(r.terraformPath, "destroy", worker.Spec)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Remove the finalizer to allow deletion
		controllerutil.RemoveFinalizer(worker, "worker.finalizers.my.domain")
		if err := r.Update(ctx, worker); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}
	// Handle creation or update
	if worker.ObjectMeta.DeletionTimestamp.IsZero() {
		// Run the script or command here
		err := executeTerraformCommandWithVars("/path/to/terraform", "apply", worker.Spec)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.terraformPath = "/usr/local/bin/terraform" // Set the terraformPath

	tf, err := tfexec.NewTerraform("/path/to/working/dir", r.terraformPath)
	if err != nil {
		return fmt.Errorf("failed to initialize Terraform: %w", err)
	}

	// Initialize Terraform
	if err := tf.Init(context.Background()); err != nil {
		return fmt.Errorf("failed to initialize Terraform: %w", err)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&workerv1.Worker{}).
		Complete(r)
}

func executeTerraformCommandWithVars(terraformPath, command string, vars workerv1.WorkerSpec) error {
	// Initialize the Terraform instance
	tf, err := tfexec.NewTerraform("/path/to/working/dir", terraformPath)
	if err != nil {
		return fmt.Errorf("failed to initialize terraform: %v", err)
	}

	//add the variables to the terraform instance
	if err := setTerraformVariables(tf, vars); err != nil {
		return fmt.Errorf("failed to set terraform variables: %v", err)
	}
	// Set the Terraform workspace
	if err := setTerraformWorkspace(tf, vars.NamePrefix); err != nil {
		return fmt.Errorf("failed to set terraform workspace: %v", err)
	}

	// Execute the command
	switch command {
	case "apply":
		err = tf.Apply(context.Background())
	case "destroy":
		err = tf.Destroy(context.Background())
	default:
		return fmt.Errorf("unsupported command: %s", command)
	}

	if err != nil {
		return fmt.Errorf("failed to execute terraform command: %v", err)
	}

	return nil
}

func setTerraformVariables(tf *tfexec.Terraform, workerSpec workerv1.WorkerSpec) error {
	varsMap := map[string]string{
		"Name":             workerSpec.NamePrefix,
		"Network":          workerSpec.Network,
		"AvailabilityZone": workerSpec.AvailabilityZone,
		"Flavor":           workerSpec.Flavor,
		"ImageRef":         workerSpec.ImageRef,
		"Count":            fmt.Sprintf("%d", workerSpec.Count),
		"KeyName":          workerSpec.KeyName,
		"SecurityGroups":   strings.Join(workerSpec.SecurityGroups, ","),
	}

	// Set the variables in the Terraform instance
	if err := tf.SetEnv(varsMap); err != nil {
		return fmt.Errorf("failed to set terraform variables: %v", err)
	}

	return nil
}

func setTerraformWorkspace(tf *tfexec.Terraform, workspace string) error {
	// select workspace
	if err := tf.WorkspaceSelect(context.Background(), workspace); err != nil {
		if err := tf.WorkspaceNew(context.Background(), workspace); err != nil {
			return fmt.Errorf("failed to create terraform workspace: %v", err)
		}
	}

	if err := tf.WorkspaceSelect(context.Background(), workspace); err != nil {
		return fmt.Errorf("failed to select terraform workspace after creation: %v", err)
	}
	return nil
}
