// Package k8sx — backend kubernetes_service.py'ning worker'ga kerakli qismi:
// deployment (+ secret/configmap/service) o'chirish va engine podlarini ko'rish.
package k8sx

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	cs        *kubernetes.Clientset
	Namespace string
	log       *zap.Logger
}

func New(namespace string, log *zap.Logger) (*Client, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		// Cluster tashqarisida (local dev) — kubeconfig fallback.
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{},
		).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("k8s config: %w", err)
		}
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{cs: cs, Namespace: namespace, log: log}, nil
}

// RemoveDeployment — backend remove_deployment bilan bir xil tartib:
// deployment + <name>-env secret + <name>-flow configmap + label bo'yicha service.
// NotFound xatolar e'tiborsiz (allaqachon yo'q).
func (c *Client) RemoveDeployment(ctx context.Context, name string) error {
	ignoreNotFound := func(err error, what string) error {
		if err == nil || apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("%s: %w", what, err)
	}

	if err := ignoreNotFound(
		c.cs.AppsV1().Deployments(c.Namespace).Delete(ctx, name, metav1.DeleteOptions{}),
		"delete deployment",
	); err != nil {
		return err
	}
	c.log.Info("removed deployment", zap.String("name", name))

	if err := ignoreNotFound(
		c.cs.CoreV1().Secrets(c.Namespace).Delete(ctx, name+"-env", metav1.DeleteOptions{}),
		"delete secret",
	); err != nil {
		c.log.Warn("secret delete", zap.Error(err))
	}
	if err := ignoreNotFound(
		c.cs.CoreV1().ConfigMaps(c.Namespace).Delete(ctx, name+"-flow", metav1.DeleteOptions{}),
		"delete configmap",
	); err != nil {
		c.log.Warn("configmap delete", zap.Error(err))
	}

	svcs, err := c.cs.CoreV1().Services(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "deployment=" + name,
	})
	if err != nil {
		c.log.Warn("service list", zap.Error(err))
		return nil
	}
	for _, svc := range svcs.Items {
		if err := ignoreNotFound(
			c.cs.CoreV1().Services(c.Namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{}),
			"delete service",
		); err != nil {
			c.log.Warn("service delete", zap.String("name", svc.Name), zap.Error(err))
		}
	}
	return nil
}

// EnginePods — app=botmother-engine label'li podlar.
func (c *Client) EnginePods(ctx context.Context) ([]corev1.Pod, error) {
	pods, err := c.cs.CoreV1().Pods(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=botmother-engine",
	})
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}

// CrashRestartThreshold — shuncha restartdan keyin pod "haqiqatan yiqilgan" sanaladi.
const CrashRestartThreshold = 5

var failedWaitingReasons = map[string]bool{
	"CrashLoopBackOff":  true,
	"Error":             true,
	"RunContainerError": true,
	"ImagePullBackOff":  true,
	"ErrImagePull":      true,
}

// IsCrashlooping — backend _is_crashlooping bilan bir xil mezon.
func IsCrashlooping(pod corev1.Pod) bool {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.RestartCount < CrashRestartThreshold {
			continue
		}
		if cs.State.Waiting != nil && failedWaitingReasons[cs.State.Waiting.Reason] {
			return true
		}
	}
	return false
}

// ReleaseEngineName — pod label'idan release deployment nomi ("" = release emas).
func ReleaseEngineName(pod corev1.Pod) string {
	name := pod.Labels["engine-name"]
	if strings.HasPrefix(name, "release-") {
		return name
	}
	return ""
}
