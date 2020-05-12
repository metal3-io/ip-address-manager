/*
Copyright 2019 The Kubernetes Authors.
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

package remote

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	clusterWithValidKubeConfig = &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test1",
			Namespace: "test",
		},
	}

	clusterWithInvalidKubeConfig = &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test2-invalid",
			Namespace: "test",
		},
	}

	clusterWithNoKubeConfig = &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test3",
			Namespace: "test",
		},
	}

	validKubeConfig = `
clusters:
- cluster:
    server: https://test-cluster-api:6443
  name: test-cluster-api
contexts:
- context:
    cluster: test-cluster-api
    user: kubernetes-admin
  name: kubernetes-admin@test-cluster-api
current-context: kubernetes-admin@test-cluster-api
kind: Config
preferences: {}
users:
- name: kubernetes-admin
`

	validSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test1-kubeconfig",
			Namespace: "test",
		},
		Data: map[string][]byte{
			secret.KubeconfigDataName: []byte(validKubeConfig),
		},
	}

	invalidSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test2-invalid-kubeconfig",
			Namespace: "test",
		},
		Data: map[string][]byte{
			secret.KubeconfigDataName: []byte("Not valid!!1"),
		},
	}
)

func TestNewClusterClient(t *testing.T) {
	t.Run("cluster with valid kubeconfig", func(t *testing.T) {
		client := fake.NewFakeClient(validSecret)
		c, err := NewClusterClient(context.TODO(), client, clusterWithValidKubeConfig)
		if err != nil {
			t.Fatalf("Expected no errors, got %v", err)
		}

		if c == nil {
			t.Fatal("Expected actual client, got nil")
		}
	})

	t.Run("cluster with no kubeconfig", func(t *testing.T) {
		client := fake.NewFakeClient()
		_, err := NewClusterClient(context.TODO(), client, clusterWithNoKubeConfig)
		if !strings.Contains(err.Error(), "not found") {
			t.Fatalf("Expected not found error, got %v", err)
		}
	})

	t.Run("cluster with invalid kubeconfig", func(t *testing.T) {
		client := fake.NewFakeClient(invalidSecret)
		_, err := NewClusterClient(context.TODO(), client, clusterWithInvalidKubeConfig)
		if err == nil || apierrors.IsNotFound(err) {
			t.Fatalf("Expected error other than not found, got %v", err)
		}
	})

}
