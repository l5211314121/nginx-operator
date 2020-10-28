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
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	//"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	nginxv1 "nginx-operator/api/v1"
)

// NginxReconciler reconciles a Nginx object
type NginxReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
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

	cm := r.ConfigMapSyncer(nginxCR)
	_, _ = r.Sync(cm)

	// 1. 使用自己写的Sync方法，而不用模块中的Sync方法，是因为模块中的Sync方法只会返回一个
	// error， 所以我们拿不到Sync的中间结果， 所以我们重写Sync方法，将其结果也返回出来
	// 2. 为什么要重写这个方法？
	// 在更新yaml文件之后， 需要修改 Status 状态，但是有时候，缓存同步的慢，例如我们通过获取
	// deployment的ready replicas是无法获取当前正确的副本数的，我们获取到的是上一次的副本数
	// 所以这里我们通过判断对deployment的操作（update，create），如果是这两个操作，进行
	// 一次requeue
	// if err = syncer.Sync(ctx, deploymentSync, r.recorder); err != nil {
	//	return ctrl.Result{}, err
	// }
	deploymentSync := r.NewDeploymentSyncer(nginxCR)
	_, _ = r.Sync(deploymentSync)

	podList := r.GetPodList(nginxCR)
	nginxCR.Status.Nodes = podList
	if len(podList) > int(nginxCR.Status.ReadyNodes) {
		return ctrl.Result{RequeueAfter: 4 * time.Second}, nil
	} else {
		log.Info("Update Status", "Nodes", nginxCR.Status.Nodes)
		err = r.Status().Update(context.TODO(), nginxCR)
		if err != nil {
			log.Error(err, "Failed to update cluster status")
		}
	}

	return ctrl.Result{}, nil
}

func (r *NginxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// ctrl.NewControllerManagerdBy 会返回一个Builder的结构体
	// For方法将 &nginxv1.Nginx{} 赋值给 Builer.forInput
	blder := ctrl.NewControllerManagedBy(mgr).For(&nginxv1.Nginx{})

	// Owner 将 &appsv1.Deployment 传给 Builer.OwnsInput 后面wath的时候会用
	// 为什么要 watch Deployment ?
	// 如果不 wathch Deployment 的话， 当手动修改 Deployment 信息，例如副本数的时候，
	// 他是不会发觉的，如果你做了 watch 操作， 这个deployment的副本数会始终和你的CR的
	// 副本数是一致的
	blder.Owns(&appsv1.Deployment{})

	// 创建一个 Controller，然后 watch nginxv1.Nginx 和 上面的 appsv1.Deployment
	_, err := blder.Build(r)
	return err
}
