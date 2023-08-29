//go:build kubernetes

package kstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/state"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"os"
	"sync"
)

type store struct {
	client     *kubernetes.Clientset
	state      *state.State
	mu         sync.RWMutex
	config     *rest.Config
	namespace  string
	name       string
	secretMeta metav1.ObjectMeta
}

var _ state.Store = &store{}

func NewIfInCluster() (state.Store, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		fmt.Println("not in a kube cluster")
		return nil, nil
	}

	s, err := New()
	if err != nil {
		fmt.Println("error starting ipc", err)
		return nil, fmt.Errorf("cannot store state in Kubernetes secrets: %v", err)
	}

	err = s.Load()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func New() (state.Store, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return nil, err
	}

	namespace := string(data)
	name := os.Getenv("HOSTNAME")
	if name == "" {
		return nil, fmt.Errorf("HOSTNAME env var is not set")
	}

	pod, err := client.CoreV1().Pods(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// let's make the name of the secret be the name of the owning resource of
	// this pod.
	dClient, err := dynamic.NewForConfig(config)

	if err != nil {
		return nil, err
	}

	groupResources, err := restmapper.GetAPIGroupResources(client.DiscoveryClient)
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	refs := findRootOwnerRef(context.Background(), dClient, namespace, mapper, pod.ObjectMeta.OwnerReferences)
	if len(refs) > 0 {
		name = refs[0].Name
	}

	return &store{
		config:    config,
		client:    client,
		namespace: namespace,
		name:      name,
		secretMeta: metav1.ObjectMeta{
			Namespace:       namespace,
			Name:            name,
			OwnerReferences: refs,
		},
	}, nil
}

func findRootOwnerRef(ctx context.Context, client *dynamic.DynamicClient, namespace string, mapper meta.RESTMapper, refs []metav1.OwnerReference) []metav1.OwnerReference {

	for _, ref := range refs {

		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			continue
		}

		mapping, err := mapper.RESTMapping(schema.GroupKind{
			Group: gv.Group,
			Kind:  ref.Kind,
		}, gv.Version)
		if err != nil {
			continue
		}

		res, err := client.Resource(mapping.Resource).Namespace(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			// fmt.Println("failed to get resource", err)
			continue
		}

		if len(res.GetOwnerReferences()) == 0 {
			return []metav1.OwnerReference{ref}
		}
		return findRootOwnerRef(ctx, client, namespace, mapper, res.GetOwnerReferences())
	}

	return nil
}

func (s *store) Close() error {
	return nil
}

func (s *store) String() string {
	return fmt.Sprintf("kube '%s'", s.config.Host)
}

func (s *store) State() *state.State {
	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()
	return state
}

// Load will read the state from the kube secret
func (s *store) Load() error {
	secret, err := s.client.CoreV1().Secrets(s.namespace).Get(context.Background(), s.name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		s.mu.Lock()
		s.state = &state.State{}
		s.mu.Unlock()

		secret = s.newSecret("{}")
		_, err = s.client.CoreV1().Secrets(s.namespace).Create(context.Background(), secret, metav1.CreateOptions{})
		if err != nil {
			return err
		}

		return nil
	} else if err != nil {
		return err
	}

	data := secret.Data["state.json"]
	state := state.State{}
	err = json.Unmarshal(data, &state)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.state = &state
	s.mu.Unlock()
	return nil
}

// Store saves the state to the kube secret
func (s *store) Store() error {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	err := enc.Encode(s.State())
	if err != nil {
		return err
	}

	secret := s.newSecret(buf.String())
	_, err = s.client.CoreV1().Secrets(s.namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
	return err
}

func (s *store) newSecret(state string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: s.secretMeta,
		StringData: map[string]string{
			"state.json": state,
		},
	}
}
