/*


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

package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	nginxv1 "nginx-operator/api/v1"
	"time"
)

// NginxReconciler reconciles a Nginx object
type NginxReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Recorder record.EventRecorder
}





// +kubebuilder:rbac:groups=nginx.example.com,resources=nginxes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nginx.example.com,resources=nginxes/status,verbs=get;update;patch

func (r *NginxReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("nginx", req.NamespacedName)

	nginxCR := &nginxv1.Nginx{}
	err := r.Get(ctx, req.NamespacedName, nginxCR)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Nginx resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get Nginx")
		return ctrl.Result{}, err
	}

	deploymentSync := NewDeploymentSyncer(r.Client, r.Scheme, nginxCR, log)
	result, err := r.Sync(deploymentSync, log, r.Recorder)
	//if err = syncer.Sync(ctx, deploymentSync, r.recorder); err != nil {
	//	return ctrl.Result{}, err
	//}

	log.Info("** Result Type **", "TYPE", result.Operation)
	if result.Operation == controllerutil.OperationResultNone{
		log.Info("** Update Status **")
		err := r.Status().Update(context.TODO(), nginxCR)
		if err != nil {
			log.Error(err, "Failed to update cluster status")
		}
	} else {
		return ctrl.Result{RequeueAfter: 4*time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *NginxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nginxv1.Nginx{}).
		Complete(r)
}
