/*
Copyright 2017 The Kubernetes Authors.

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

package utils

import (
	"errors"
	"io/ioutil"
	"testing"
	"time"

	apiv1 "k8s.io/api/core/v1"
	kube_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/autoscaler/cluster-autoscaler/clusterstate/api"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"

	"github.com/stretchr/testify/assert"
)

type testInfo struct {
	client       *fake.Clientset
	configMap    *apiv1.ConfigMap
	namespace    string
	getError     error
	getCalled    bool
	updateCalled bool
	createCalled bool
	t            *testing.T
}

func setUpTest(t *testing.T) *testInfo {
	namespace := "kube-system"
	result := testInfo{
		client: &fake.Clientset{},
		configMap: &apiv1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "my-cool-configmap",
			},
			Data: map[string]string{},
		},
		namespace:    namespace,
		getCalled:    false,
		updateCalled: false,
		createCalled: false,
		t:            t,
	}
	result.client.Fake.AddReactor("get", "configmaps", func(action core.Action) (bool, runtime.Object, error) {
		get := action.(core.GetAction)
		assert.Equal(result.t, namespace, get.GetNamespace())
		assert.Equal(result.t, "my-cool-configmap", get.GetName())
		result.getCalled = true
		if result.getError != nil {
			return true, nil, result.getError
		}
		return true, result.configMap, nil
	})
	result.client.Fake.AddReactor("update", "configmaps", func(action core.Action) (bool, runtime.Object, error) {
		update := action.(core.UpdateAction)
		assert.Equal(result.t, namespace, update.GetNamespace())
		result.updateCalled = true
		return true, result.configMap, nil
	})
	result.client.Fake.AddReactor("create", "configmaps", func(action core.Action) (bool, runtime.Object, error) {
		create := action.(core.CreateAction)
		assert.Equal(result.t, namespace, create.GetNamespace())
		configMap := create.GetObject().(*apiv1.ConfigMap)
		assert.Equal(result.t, "my-cool-configmap", configMap.ObjectMeta.Name)
		result.createCalled = true
		return true, configMap, nil
	})
	return &result
}

func TestWriteStatusConfigMapExisting(t *testing.T) {
	ti := setUpTest(t)
	result, err := WriteStatusConfigMap(ti.client, ti.namespace, api.ClusterAutoscalerStatus{Message: "TEST_MSG"}, nil, "my-cool-configmap", time.Now())
	assert.Equal(t, ti.configMap, result)
	assert.Contains(t, result.Data["status"], "TEST_MSG")
	assert.Contains(t, result.ObjectMeta.Annotations, ConfigMapLastUpdatedKey)
	assert.Nil(t, err)
	assert.True(t, ti.getCalled)
	assert.True(t, ti.updateCalled)
	assert.False(t, ti.createCalled)

	// to test the case where configmap is empty
	ti.configMap.Data = nil
	result, err = WriteStatusConfigMap(ti.client, ti.namespace, api.ClusterAutoscalerStatus{Message: "TEST_MSG"}, nil, "my-cool-configmap", time.Now())
	assert.Equal(t, ti.configMap, result)
	assert.Contains(t, result.Data["status"], "TEST_MSG")
	assert.Contains(t, result.ObjectMeta.Annotations, ConfigMapLastUpdatedKey)
	assert.Nil(t, err)
	assert.True(t, ti.getCalled)
	assert.True(t, ti.updateCalled)
	assert.False(t, ti.createCalled)
}

func TestWriteStatusConfigMapCreate(t *testing.T) {
	ti := setUpTest(t)
	ti.getError = kube_errors.NewNotFound(apiv1.Resource("configmap"), "nope, not found")
	result, err := WriteStatusConfigMap(ti.client, ti.namespace, api.ClusterAutoscalerStatus{Message: "TEST_MSG"}, nil, "my-cool-configmap", time.Now())
	assert.Contains(t, result.Data["status"], "TEST_MSG")
	assert.Contains(t, result.ObjectMeta.Annotations, ConfigMapLastUpdatedKey)
	assert.Nil(t, err)
	assert.True(t, ti.getCalled)
	assert.False(t, ti.updateCalled)
	assert.True(t, ti.createCalled)
}

func TestWriteStatusConfigMapError(t *testing.T) {
	ti := setUpTest(t)
	ti.getError = errors.New("stuff bad")
	result, err := WriteStatusConfigMap(ti.client, ti.namespace, api.ClusterAutoscalerStatus{Message: "TEST_MSG"}, nil, "my-cool-configmap", time.Now())
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "stuff bad")
	assert.Nil(t, result)
	assert.True(t, ti.getCalled)
	assert.False(t, ti.updateCalled)
	assert.False(t, ti.createCalled)
}

var status api.ClusterAutoscalerStatus = api.ClusterAutoscalerStatus{
	Message:          "TEST_MSG",
	AutoscalerStatus: "Running",
	ClusterWide: api.ClusterWideStatus{
		Health: api.ClusterHealthCondition{
			Status: "Healthy",
			NodeCounts: api.NodeCount{
				Registered: api.RegisteredNodeCount{
					Total:        10,
					Ready:        4,
					NotStarted:   3,
					BeingDeleted: 1,
					Unready: api.RegisteredUnreadyNodeCount{
						Total:           2,
						ResourceUnready: 1,
					},
				},
				LongUnregistered: 1,
				Unregistered:     2,
			},
			LastProbeTime:      metav1.Date(2023, 11, 24, 04, 28, 19, 48988, time.UTC),
			LastTransitionTime: metav1.Date(2023, 11, 23, 14, 52, 02, 11000, time.UTC),
		},
		ScaleUp: api.ClusterScaleUpCondition{
			Status:             "NoActivity",
			LastProbeTime:      metav1.Date(2023, 11, 24, 04, 28, 19, 48988, time.UTC),
			LastTransitionTime: metav1.Date(2023, 11, 23, 14, 52, 02, 11000, time.UTC),
		},
		ScaleDown: api.ScaleDownCondition{
			Status:             "NoCandidates",
			Candidates:         2,
			LastProbeTime:      metav1.Date(2023, 11, 24, 04, 28, 19, 48988, time.UTC),
			LastTransitionTime: metav1.Date(2023, 11, 23, 14, 52, 02, 11000, time.UTC),
		},
	},
	NodeGroups: []api.NodeGroupStatus{{
		Name: "sample-node-group",
		Health: api.NodeGroupHealthCondition{
			Status: "Healthy",
			NodeCounts: api.NodeCount{
				Registered: api.RegisteredNodeCount{
					Total:        10,
					Ready:        4,
					NotStarted:   3,
					BeingDeleted: 1,
					Unready: api.RegisteredUnreadyNodeCount{
						Total:           2,
						ResourceUnready: 1,
					},
				},
				LongUnregistered: 1,
				Unregistered:     2,
			},
			CloudProviderTarget: 8,
			MinSize:             2,
			MaxSize:             12,
			LastProbeTime:       metav1.Date(2023, 11, 24, 04, 28, 19, 48988, time.UTC),
			LastTransitionTime:  metav1.Date(2023, 11, 23, 14, 52, 02, 11000, time.UTC),
		},
		ScaleUp: api.NodeGroupScaleUpCondition{
			Status: "Backoff",
			BackoffInfo: api.BackoffInfo{
				ErrorCode:    "QUOTA_EXCEEDED",
				ErrorMessage: "Instance 'sample-node-group-40ce0341-t28s' creation failed: Quota 'CPUS' exceeded. Limit: 57.0 in region us-central1.",
			},
			LastProbeTime:      metav1.Date(2023, 11, 24, 04, 28, 19, 48988, time.UTC),
			LastTransitionTime: metav1.Date(2023, 11, 23, 14, 52, 02, 11000, time.UTC),
		},
		ScaleDown: api.ScaleDownCondition{
			Status:             "NoCandidates",
			Candidates:         2,
			LastProbeTime:      metav1.Date(2023, 11, 24, 04, 28, 19, 48988, time.UTC),
			LastTransitionTime: metav1.Date(2023, 11, 23, 14, 52, 02, 11000, time.UTC),
		},
	}},
}

func TestWriteStatusConfigMapMarshal(t *testing.T) {
	const statusYamlTestFile = "status_test.yaml"
	ti := setUpTest(t)
	want, err := ioutil.ReadFile(statusYamlTestFile)
	if err != nil {
		t.Fatalf("Failed to Marshal %s: %v", statusYamlTestFile, err)
	}
	result, err := WriteStatusConfigMap(ti.client, ti.namespace, status, nil, "my-cool-configmap", time.Date(2023, 11, 24, 4, 28, 19, 546750398, time.UTC))
	if err != nil {
		t.Fatalf("Expected WriteStatusConfigMap not to return error, got: %v", err)
	}
	assert.YAMLEq(t, string(want), result.Data["status"])
}
