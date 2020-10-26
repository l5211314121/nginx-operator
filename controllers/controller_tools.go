package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/presslabs/controller-util/syncer"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	nginxv1 "nginx-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type deploySyncer struct {
	nginx 		*nginxv1.Nginx
}

func (s *deploySyncer) SyncFn(in runtime.Object,log logr.Logger) error {
	out := in.(*appsv1.Deployment)
	out.Spec.Replicas = &s.nginx.Spec.Size
	out.Spec.Selector = metav1.SetAsLabelSelector(labels.Set{
		"app": "nginx",
		"name": s.nginx.Name,
	})
	out.Spec.Template.ObjectMeta.Labels = map[string]string{
		"app": "nginx",
		"name": s.nginx.Name,
	}

	s.nginx.Status.ReadyNodes = out.Status.ReadyReplicas
	log.Info("** Syncer ReadyNodes **", "Status", out.ObjectMeta.Generation)

	return nil
}

func NewDeploymentSyncer(c client.Client, scheme *runtime.Scheme, nginx *nginxv1.Nginx, log logr.Logger) syncer.Interface {
	obj  := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:                       nginx.Name,
			Namespace:                  nginx.Namespace,
		},
		Spec:	appsv1.DeploymentSpec{
			Replicas: &nginx.Spec.Size,
			Selector: &metav1.LabelSelector{
				MatchLabels:      map[string]string{
					"app": "nginx",
					"name": nginx.Name,
				},
				MatchExpressions: nil,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:         nginx.Name,
					Namespace:    nginx.Namespace,
				},
				Spec:       corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  nginx.Name,
							Image: nginx.Spec.Image,
						},
					},
				},
			},
		},
	}
	sync := &deploySyncer {
		nginx: nginx,
	}
	return syncer.NewObjectSyncer("Deployment", nginx, obj, c, scheme, func() error {
		return sync.SyncFn(obj, log)
	})
}

func (r *NginxReconciler) Sync (s syncer.Interface,log logr.Logger, recorder record.EventRecorder) (syncer.SyncResult, error){
	// 进行更新
	result, err := s.Sync(context.Background())
	owner := s.GetOwner()
	if recorder != nil && owner != nil && result.EventType != "" && result.EventReason != "" && result.EventMessage != "" {
		recorder.Eventf(owner, result.EventType, result.EventReason, result.EventMessage)
	}
	return result, err
}