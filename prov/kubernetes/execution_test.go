// Copyright 2018 Bull S.A.S. Atos Technologies - Bull, Rue Jean Jaures, B.P.68, 78340, Les Clayes-sous-Bois, France.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes

import (
	"context"
	"strconv"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/ystia/yorc/v4/config"
	"github.com/ystia/yorc/v4/prov"
	"github.com/ystia/yorc/v4/tasks"
	"github.com/ystia/yorc/v4/testutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

var JSONvalidDeployment = `
{
  "apiVersion": "extensions/v1beta1",
  "kind": "Deployment",
  "metadata": {
     "name": "test-deploy"
  },
  "spec": {
     "replicas": 3,
     "template": {
      "metadata": {
       "labels": {
        "app": "yorc"
       }
      },
      "spec": {
       "containers": [
        {
           "name": "yorc-container",
           "image": "ystia/yorc:3.0.2",
           "env": [
            {
             "name": "POD_IP",
             "valueFrom": {
              "fieldRef": {
                 "fieldPath": "status.podIP"
              }
             }
            },
            {
             "name": "NAMESPACE",
             "valueFrom": {
              "fieldRef": {
                 "fieldPath": "metadata.namespace"
              }
             }
            },
            {
             "name": "POD_NAME",
             "valueFrom": {
              "fieldRef": {
                 "fieldPath": "metadata.name"
              }
             }
            },
            {
             "name": "YORC_LOG",
             "value": "DEBUG"
            }
           ]
        }
       ]
      }
     }
  }
 }`

var JSONvalidStatefulSet = `
{
	"metadata" : {
	  "name" : "test-sts"
	},
	"apiVersion" : "apps/v1",
	"kind" : "StatefulSet",
	"spec" : {
	  "template" : {
		"metadata" : {
		  "labels" : {
			"app" : "yorcdeployment-973d85c6b920"
		  }
		},
		"spec" : {
		  "volumes" : [ {
			"name" : "volume",
			"persistentVolumeClaim" : { }
		  } ],
		  "containers" : [ {
			"image" : "ystia/yorc:3.0.2",
			"name" : "yorc--1429866314",
			"resources" : {
			  "requests" : {
				"memory" : 128000000,
				"cpu" : 0.3
			  }
			},
			"ports" : [ {
			  "name" : "yorc-server",
			  "containerPort" : 8800
			}, {
			  "name" : "consul-ui",
			  "containerPort" : 8500
			} ],
			"env" : [ {
			  "name" : "YORC_LOG",
			  "value" : "NO_DEBUG"
			} ]
		  } ]
		}
	  },
	  "replicas" : 3
	}
  }
`

var JSONvalidPVC = `
{
  "apiVersion" : "v1",
  "kind" : "PersistentVolumeClaim",
  "metadata" : {
    "name" : "test-pvc"
    },
  "spec" : {
    "resources" : {
    "requests" : {
      "storage" : 5000000000
    }
    },
    "accessModes" : [ "ReadWriteOnce" ]
  }
  }
 `

var JSONvalidService = `
 {
  "apiVersion" : "v1",
  "kind" : "Service",
  "metadata" : {
    "name" : "test-service"
    },
  "spec" : {
    "selector" : {
    "app" : "yorcdeployment-1623552477"
    },
    "ports" : [ {
    "port" : 8800,
    "name" : "yorc-server",
    "targetPort" : "yorc-server"
    }, {
    "port" : 8500,
    "name" : "consul-ui",
    "targetPort" : "consul-ui"
    } ],
    "type" : "NodePort"
  }
  }
 `

var JSONinvalidService = `
{
	"apiVersion" : "v1",
	"kind" : MissingQuoteService",
	"metadata" : {
	  "name" : "yorc-yorcdeployment-service-1116022612"
	  },
	"spec" : {
	  "selector" : {
	  "app" : "yorcdeployment-1623552477"
	  },
	  "ports" : [ {
	  "port" : 8800,
	  "name" : "yorc-server",
	  "targetPort" : "yorc-server"
	  }, {
	  "port" : 8500,
	  "name" : "consul-ui",
	  "targetPort" : "consul-ui"
	  } ],
	  "type" : "NodePort"
	}
	}
   `

type testResource struct {
	K8sObj        yorcK8sObject
	rSpec         string
	resourceGroup string
}

func getSupportedResourceAndJSON() []testResource {
	supportedRes := []testResource{
		{
			&yorcK8sDeployment{},
			JSONvalidDeployment,
			"deployments",
		},
		{
			&yorcK8sPersistentVolumeClaim{},
			JSONvalidPVC,
			"persistentvolumeclaims",
		},
		{
			&yorcK8sService{},
			JSONvalidService,
			"services",
		},
		{
			&yorcK8sStatefulSet{},
			JSONvalidStatefulSet,
			"statefulsets",
		},
	}

	return supportedRes
}
func Test_execution_invalid_JSON(t *testing.T) {
	operationType := k8sCreateOperation
	e := &execution{}
	tests := []struct {
		name        string
		k8sResource yorcK8sObject
		rSpec       string
		wantErr     bool
	}{
		{
			"Test no rSpec",
			&yorcK8sDeployment{},
			" ",
			true,
		},
		// { UNDETECTED FOR NOW
		// 	"Test wrong rSpec",
		// 	&yorcK8sDeployment{},
		// 	JSONvalidPVC,
		// 	true,
		// },
		{
			"Test invalid JSON rSpec",
			&yorcK8sService{},
			JSONinvalidService,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := e.manageK8sResource(nil, nil, nil, tt.k8sResource, operationType, tt.rSpec); (err != nil) != tt.wantErr {
				t.Errorf("execution.manageK8sResource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

}

func deployTestResources(e *execution, k8s *k8s, resources []testResource) error {
	ctx := context.Background()
	for _, testRes := range resources {
		testRes.K8sObj.unmarshalResource(ctx, e, e.deploymentID, k8s.clientset, testRes.rSpec)
		if err := testRes.K8sObj.createResource(ctx, e.deploymentID, k8s.clientset, "test-namespace"); err != nil {
			return err
		}
	}
	return nil
}

func Test_execution_del_resources(t *testing.T) {
	//t.SkipNow()
	srv, client := testutil.NewTestConsulInstance(t)
	kv := client.KV()
	defer srv.Stop()
	e := &execution{
		kv:           kv,
		deploymentID: "Dep-ID",
		operation:    prov.Operation{},
	}
	k8s := newTestK8s()
	resources := getSupportedResourceAndJSON()
	deployTestResources(e, k8s, resources)
	wantErr := false
	for _, testRes := range resources {
		errorChan := make(chan struct{})
		okChan := make(chan struct{})
		k8s.clientset.(*fake.Clientset).Fake.AddReactor("get", testRes.resourceGroup, fakeObjectDeletion(testRes.K8sObj, errorChan))
		t.Run("Test delete resource "+testRes.K8sObj.String(), func(t *testing.T) {
			if err := e.manageK8sResource(context.Background(), k8s.clientset, nil, testRes.K8sObj, k8sDeleteOperation, testRes.rSpec); (err != nil) != wantErr {
				t.Errorf("execution.manageK8sResource() error = %v, wantErr %v", err, wantErr)
			}
			close(okChan)
		})
		select {
		case <-errorChan:
			t.Fatal("fatal")
		case <-okChan:
			t.Logf("Deletion ok for %s\n", testRes.K8sObj)
		}
	}
}

func Test_execution_valid_JSON(t *testing.T) {
	//t.SkipNow()
	srv, client := testutil.NewTestConsulInstance(t)
	kv := client.KV()
	defer srv.Stop()
	e := &execution{
		kv:           kv,
		deploymentID: "Dep-ID",
		operation:    prov.Operation{},
	}
	k8s := newTestK8s()
	ctx := context.Background()
	operationType := k8sCreateOperation
	wantErr := false
	for _, testRes := range getSupportedResourceAndJSON() {
		errorChan := make(chan struct{})
		okChan := make(chan struct{})
		k8s.clientset.(*fake.Clientset).Fake.AddReactor("get", testRes.resourceGroup, fakeObjectCompletion(testRes.K8sObj, errorChan))
		t.Run("Test resource "+testRes.K8sObj.String(), func(t *testing.T) {
			t.Logf("Testing %s\n", testRes.K8sObj)
			if err := e.manageK8sResource(ctx, k8s.clientset, nil, testRes.K8sObj, operationType, testRes.rSpec); (err != nil) != wantErr {
				t.Errorf("execution.manageK8sResource() error = %v, wantErr %v", err, wantErr)
			}
			close(okChan)
		})
		select {
		case <-errorChan:
			t.Fatal("fatal")
		case <-okChan:
			t.Logf("Execution ok for %s\n", testRes.K8sObj)
		}
	}

}

func Test_execution_manageK8sResource(t *testing.T) {
	type fields struct {
		kv           *api.KV
		cfg          config.Configuration
		deploymentID string
		taskID       string
		taskType     tasks.TaskType
		nodeName     string
		operation    prov.Operation
		nodeType     string
	}
	type args struct {
		ctx           context.Context
		clientset     kubernetes.Interface
		generator     *k8sGenerator
		k8sResource   yorcK8sObject
		operationType k8sResourceOperation
		rSpec         string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Test no rSpec",
			fields{},
			args{k8sResource: &yorcK8sDeployment{}},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &execution{
				kv:           tt.fields.kv,
				cfg:          tt.fields.cfg,
				deploymentID: tt.fields.deploymentID,
				taskID:       tt.fields.taskID,
				taskType:     tt.fields.taskType,
				nodeName:     tt.fields.nodeName,
				operation:    tt.fields.operation,
				nodeType:     tt.fields.nodeType,
			}
			if err := e.manageK8sResource(tt.args.ctx, tt.args.clientset, tt.args.generator, tt.args.k8sResource, tt.args.operationType, tt.args.rSpec); (err != nil) != tt.wantErr {
				t.Errorf("execution.manageK8sResource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_execution_getExpectedInstances(t *testing.T) {
	srv, client := testutil.NewTestConsulInstance(t)
	kv := client.KV()
	defer srv.Stop()

	deploymentID := "Dep-ID"

	type fields struct {
		kv           *api.KV
		deploymentID string
		taskID       string
	}
	tests := []struct {
		name    string
		fields  fields
		data    string
		want    int32
		wantErr bool
	}{
		{
			"task input filled",
			fields{kv, deploymentID, "task-id-1"},
			strconv.Itoa(int(3)),
			3,
			false,
		},
		{
			"task input wrongly filled",
			fields{kv, deploymentID, "task-id-2"},
			"not a integer",
			-1,
			true,
		},
		{
			"task input not filled",
			fields{kv, deploymentID, "task-id-3"},
			"",
			-1,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &execution{
				kv:           tt.fields.kv,
				deploymentID: tt.fields.deploymentID,
				taskID:       tt.fields.taskID,
			}
			tasks.SetTaskData(e.kv, e.taskID, "inputs/EXPECTED_INSTANCES", tt.data)
			got, err := e.getExpectedInstances()
			if (err != nil) != tt.wantErr {
				t.Errorf("execution.getExpectedInstances() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("execution.getExpectedInstances() = %v, want %v", got, tt.want)
			}
		})
	}
}
