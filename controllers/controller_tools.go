package controllers

import (
	"context"
	"github.com/presslabs/controller-util/syncer"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	nginxv1 "nginx-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type deploySyncer struct {
	nginx *nginxv1.Nginx
}

func (r *NginxReconciler) ConfigMapSyncer(nginx *nginxv1.Nginx) syncer.Interface {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nginx.Name,
			Namespace: nginx.Namespace,
			Labels: map[string]string{
				"app":  "configmap",
				"name": nginx.Name,
			},
		},
		Data: NginxDefaultSettings(),
	}

	return syncer.NewObjectSyncer("ConfigMap", nginx, cm, r.Client, r.Scheme, func() error {
		return nil
	})
}

func (s *deploySyncer) ensureVolumes() []corev1.Volume {
	fileMode := int32(0644)
	volumes := []corev1.Volume{
		{
			Name: s.nginx.Name,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: s.nginx.Name,
					},
					DefaultMode: &fileMode,
				},
			},
		},
	}
	return volumes
}

func (s *deploySyncer) ensureContainers() []corev1.Container {
	containers := []corev1.Container{
		{
			Name:  s.nginx.Name,
			Image: s.nginx.Spec.Image,
			VolumeMounts: []corev1.VolumeMount{
				{Name: s.nginx.Name, MountPath: "/mnt/"},
			},
		},
	}
	return containers
}

func (s *deploySyncer) ensurePodSpec() corev1.PodSpec {
	podSpec := corev1.PodSpec{
		Volumes:    s.ensureVolumes(),
		Containers: s.ensureContainers(),
	}
	return podSpec
}

func (s *deploySyncer) DeploymentSyncFn(in runtime.Object) error {
	out := in.(*appsv1.Deployment)
	out.Spec.Replicas = &s.nginx.Spec.Size
	out.Spec.Selector = metav1.SetAsLabelSelector(labels.Set{
		"app":  "nginx",
		"name": s.nginx.Name,
	})
	out.Spec.Template.ObjectMeta.Labels = map[string]string{
		"app":  "nginx",
		"name": s.nginx.Name,
	}
	out.Spec.Template.Spec = s.ensurePodSpec()

	s.nginx.Status.ReadyNodes = out.Status.ReadyReplicas
	return nil
}

func (r *NginxReconciler) NewDeploymentSyncer(nginx *nginxv1.Nginx) syncer.Interface {
	obj := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nginx.Name,
			Namespace: nginx.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &nginx.Spec.Size,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":  "nginx",
					"name": nginx.Name,
				},
				MatchExpressions: nil,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nginx.Name,
					Namespace: nginx.Namespace,
				},
				Spec: corev1.PodSpec{},
			},
		},
	}
	sync := &deploySyncer{
		nginx: nginx,
	}
	return syncer.NewObjectSyncer("Deployment", nginx, obj, r.Client, r.Scheme, func() error {
		return sync.DeploymentSyncFn(obj)
	})
}

func (r *NginxReconciler) Sync(s syncer.Interface) (syncer.SyncResult, error) {
	// 进行更新
	result, err := s.Sync(context.Background())
	owner := s.GetOwner()
	if r.Recorder != nil && owner != nil && result.EventType != "" && result.EventReason != "" && result.EventMessage != "" {
		r.Recorder.Eventf(owner, result.EventType, result.EventReason, result.EventMessage)
	}
	return result, err
}

func (r *NginxReconciler) GetPodList(nginxCR *nginxv1.Nginx) []string {
	listOpts := []client.ListOption{
		client.InNamespace(nginxCR.Namespace),
		client.MatchingLabels(labelsForNginxd(nginxCR.Name)),
	}
	podList := &corev1.PodList{}
	if err := r.List(context.TODO(), podList, listOpts...); err != nil {
		r.Log.Error(err, "Failed to get Pod List")
	}
	podNames := []string{}
	for _, pod := range podList.Items {
		r.Log.Info("Pod", "podItem", pod.Name)
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

func labelsForNginxd(name string) map[string]string {
	return map[string]string{"app": "nginx", "name": name}
}

func NginxDefaultSettings() map[string]string {
	return map[string]string{
		"default": `server {
    listen       80;
    listen  [::]:80;
    server_name  localhost;

    #charset koi8-r;
    #access_log  /var/log/nginx/host.access.log  main;
	#abcdefaffjawefakgdsfa

    location / {
        root   /usr/share/nginx/html;
        index  index.html index.htm;
    }

    #error_page  404              /404.html;

    # redirect server error pages to the static page /50x.html
    #
    error_page   500 502 503 504  /50x.html;
    location = /50x.html {
        root   /usr/share/nginx/html;
    }

    # proxy the PHP scripts to Apache listening on 127.0.0.1:80
    #
    #location ~ \.php$ {
    #    proxy_pass   http://127.0.0.1;
    #}

    # pass the PHP scripts to FastCGI server listening on 127.0.0.1:9000
    #
    #location ~ \.php$ {
    #    root           html;
    #    fastcgi_pass   127.0.0.1:9000;
    #    fastcgi_index  index.php;
    #    fastcgi_param  SCRIPT_FILENAME  /scripts$fastcgi_script_name;
    #    include        fastcgi_params;
    #}

    # deny access to .htaccess files, if Apache's document root
    # concurs with nginx's one
    #
    #location ~ /\.ht {
    #    deny  all;
    #}
}`,
	}
}
