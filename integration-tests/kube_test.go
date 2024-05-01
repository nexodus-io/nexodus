//go:build integration

package integration_tests

import (
	"bytes"
	"context"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"io"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreStateInKubeFileSystem(t *testing.T) {

	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	namespace := username
	client := helper.getKubeClient()

	// create a namespace to work in...
	_, err := client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})
	require.NoError(err)
	defer func() {
		_ = client.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	}()

	// Load the root CA into a Secret
	helper.createKubeCACerts(ctx, namespace)

	// Don't setup RBAC.. nexd won't be able to create secrets and should fall back to using file system state.

	// Run the nexd as a deployment.
	helper.createNexdDeployment(ctx, namespace, username, password)

	pod, originalIP := helper.getPodAndTunnelIP(ctx, client, namespace, "app.kubernetes.io/name=nexd")

	// Delete the pod...
	err = client.CoreV1().Pods(namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	require.NoError(err)

	helper.waitForDelete(ctx, client, pod)

	// The new pod should have a different name
	newPod, newIP := helper.getPodAndTunnelIP(ctx, client, namespace, "app.kubernetes.io/name=nexd")

	require.NotEqual(pod.Name, newPod.Name)

	// Since the file system state was not saved between pod restarts, we should get a new IP.
	require.NotEqual(originalIP, newIP)

}

// TestStoreStateInKubeSecrets tests that nexd proxy can be used with a single egress rule
func TestStoreStateInKubeSecrets(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	namespace := username
	client := helper.getKubeClient()

	// create a namespace to work in...
	_, err := client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})
	require.NoError(err)
	defer func() {
		_ = client.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	}()

	// Give the nexd pod access to read/write secrets and inspect his pod/app deployment.
	helper.setupKubeRBAC(ctx, namespace)

	// Load the root CA into a Secret
	helper.createKubeCACerts(ctx, namespace)

	// Run the nexd as a deployment.
	helper.createNexdDeployment(ctx, namespace, username, password)

	pod, originalIP := helper.getPodAndTunnelIP(ctx, client, namespace, "app.kubernetes.io/name=nexd")

	// Delete the pod...
	err = client.CoreV1().Pods(namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	require.NoError(err)

	helper.waitForDelete(ctx, client, pod)

	// The new pod should have a different name, but the same IP since the state was saved.
	newPod, newIP := helper.getPodAndTunnelIP(ctx, client, namespace, "app.kubernetes.io/name=nexd")

	require.NotEqual(pod.Name, newPod.Name)
	require.Equal(originalIP, newIP)
}

func (h *Helper) createNexdDeployment(ctx context.Context, namespace string, username string, password string) {
	_, err := h.getKubeClient().AppsV1().Deployments(namespace).Create(ctx, &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "nexd",
		},
		Spec: appv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/component": "nexd",
					"app.kubernetes.io/instance":  "nexd",
					"app.kubernetes.io/name":      "nexd",
					"app.kubernetes.io/part-of":   "nexd",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/component": "nexd",
						"app.kubernetes.io/instance":  "nexd",
						"app.kubernetes.io/name":      "nexd",
						"app.kubernetes.io/part-of":   "nexd",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "nexd",
							Image:           "quay.io/nexodus/nexd:latest",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name:  "NEXD_USERNAME",
									Value: username,
								},
								{
									Name:  "NEXD_PASSWORD",
									Value: password,
								},
								{
									Name:  "NEXD_SERVICE_URL",
									Value: "https://try.nexodus.127.0.0.1.nip.io",
								},
							},
							Command: []string{
								"/bin/sh",
								"-c",
								fmt.Sprintf(`
							if [ -d /var/run/secrets/nexd-certs ] ; then
								CAROOT=/var/run/secrets/nexd-certs /bin/mkcert -install
							fi
							echo >> /etc/hosts
							echo "%s auth.try.nexodus.127.0.0.1.nip.io api.try.nexodus.127.0.0.1.nip.io" >> /etc/hosts
							exec /bin/nexd proxy
						`, hostDNSName),
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: boolPtr(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "nexd-certs",
									MountPath: "/var/run/secrets/nexd-certs",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "nexd-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  "nexd-certs",
									DefaultMode: int32Ptr(0600),
									Optional:    boolPtr(false),
								},
							},
						},
					},
				},
			},
			Strategy:                appv1.DeploymentStrategy{},
			MinReadySeconds:         0,
			RevisionHistoryLimit:    nil,
			Paused:                  false,
			ProgressDeadlineSeconds: nil,
		},
	}, metav1.CreateOptions{})
	h.require.NoError(err)
}
func (h *Helper) waitForDelete(ctx context.Context, client *kubernetes.Clientset, pod *corev1.Pod) {
	err := backoff.Retry(
		func() error {
			_, err := client.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
			if err == nil {
				return fmt.Errorf("found")
			}
			return nil
		},
		backoff.WithContext(backoff.NewConstantBackOff(1*time.Second), ctx),
	)
	h.require.NoError(err)
}

func (h *Helper) getPodAndTunnelIP(ctx context.Context, client *kubernetes.Clientset, namespace, selector string) (*corev1.Pod, string) {
	tunnelIP := ""
	var pod *corev1.Pod
	err := backoff.Retry(
		func() error {
			podList, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: selector,
			})
			if err != nil {
				return err
			}
			if len(podList.Items) < 1 {
				return fmt.Errorf("empty list")
			}
			pod = &podList.Items[0]

			if pod.Status.Phase != "Running" {
				return fmt.Errorf("Pod is not running")
			}

			result, err := h.kubeShellOutput(ctx, pod, "/bin/nexctl nexd get tunnelip")
			if err != nil {
				return err
			}
			result = strings.TrimSpace(result)
			if result == "" {
				return fmt.Errorf("no tunnel ip yet")
			}

			tunnelIP = result
			fmt.Println("tunnel ip is", tunnelIP)
			return nil
		},
		backoff.WithContext(backoff.NewConstantBackOff(1*time.Second), ctx),
	)
	h.require.NoError(err)
	return pod, tunnelIP
}

func (h *Helper) getKubeConfig() *rest.Config {
	c, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	h.require.NoError(err)
	clientConfig := clientcmd.NewDefaultClientConfig(*c, nil)
	config, err := clientConfig.ClientConfig()
	h.require.NoError(err)
	return config
}

func (h *Helper) getKubeClient() *kubernetes.Clientset {
	client, err := kubernetes.NewForConfig(h.getKubeConfig())
	h.require.NoError(err)
	return client
}

func (h *Helper) createKubeCACerts(ctx context.Context, namespace string) {
	client := h.getKubeClient()
	certsDir, err := findCertsDir()
	h.require.NoError(err)
	rootCA, err := os.ReadFile(filepath.Join(certsDir, "rootCA.pem"))
	h.require.NoError(err)
	_, err = client.CoreV1().Secrets(namespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "nexd-certs",
		},
		StringData: map[string]string{
			"rootCA.pem": string(rootCA),
		},
	}, metav1.CreateOptions{})
	h.require.NoError(err)
}

func (h *Helper) setupKubeRBAC(ctx context.Context, namespace string) {

	client := h.getKubeClient()
	_, err := client.RbacV1().Roles(namespace).Create(ctx, &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "nexd",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "update", "create", "patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"replicasets", "deployments", "statefulsets", "daemonsets"},
				Verbs:     []string{"get"},
			},
		},
	}, metav1.CreateOptions{})
	h.require.NoError(err)
	_, err = client.RbacV1().RoleBindings(namespace).Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "nexd",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "Role",
			Name:     "nexd",
		},
		Subjects: []rbacv1.Subject{
			{
				Namespace: namespace,
				Kind:      "ServiceAccount",
				Name:      "default",
			},
		},
	}, metav1.CreateOptions{})
	h.require.NoError(err)
}

func (h *Helper) kubeShellOutput(ctx context.Context, pod *corev1.Pod, command string) (string, error) {
	out := bytes.NewBuffer(nil)
	err := h.kubeShell(ctx, pod, nil, out, out, command)
	return out.String(), err
}

func (h *Helper) kubeShell(ctx context.Context, pod *corev1.Pod, stdin io.Reader, stdout io.Writer, stderr io.Writer, command string) error {
	client := h.getKubeClient()
	config := h.getKubeConfig()

	cmd := []string{
		"/bin/sh",
		"-c",
		command,
	}
	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(pod.Name).
		Namespace(pod.Namespace).SubResource("exec")
	option := &corev1.PodExecOptions{
		Command: cmd,
		Stdin:   true,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
	}
	if stdin == nil {
		option.Stdin = false
	}
	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}
	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
}

func int32Ptr(i int32) *int32 { return &i }
func boolPtr(i bool) *bool    { return &i }
