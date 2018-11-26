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
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"

	"github.com/ystia/yorc/deployments"
	"github.com/ystia/yorc/events"
	"github.com/ystia/yorc/prov"
	"github.com/ystia/yorc/tasks"
)

func (e *execution) executeAsync(ctx context.Context, stepName string, clientset kubernetes.Interface) (*prov.Action, time.Duration, error) {
	if strings.ToLower(e.operation.Name) != "tosca.interfaces.node.lifecycle.runnable.run" {
		return nil, 0, errors.Errorf("%q operation is not supported by the Kubernetes executor only \"tosca.interfaces.node.lifecycle.Runnable.run\" is.", e.operation.Name)
	}

	if e.nodeType != "yorc.nodes.kubernetes.api.types.JobResource" {
		return nil, 0, errors.Errorf("%q node type is not supported by the Kubernetes executor only \"yorc.nodes.kubernetes.api.types.JobResource\" is.", e.nodeType)
	}

	jobID, err := tasks.GetTaskData(e.kv, e.taskID, e.nodeName+"-jobId")
	if err != nil {
		return nil, 0, err
	}

	job, err := getJob(e.kv, e.deploymentID, e.nodeName)

	// Fill all used data for job monitoring
	data := make(map[string]string)
	data["originalTaskID"] = e.taskID
	data["jobID"] = jobID
	data["namespace"] = job.namespace
	data["namespaceProvided"] = strconv.FormatBool(job.namespaceProvided)
	data["stepName"] = stepName
	// TODO deal with outputs?
	// data["outputs"] = strings.Join(e.jobInfo.outputs, ",")
	return &prov.Action{ActionType: "k8s-job-monitoring", Data: data}, 5 * time.Second, nil
}

func (e *execution) submitJob(ctx context.Context, clientset kubernetes.Interface) error {
	job, err := getJob(e.kv, e.deploymentID, e.nodeName)
	if err != nil {
		return err
	}

	if !job.namespaceProvided {
		err = createNamespaceIfMissing(e.deploymentID, job.namespace, clientset)
		if err != nil {
			return err
		}
		events.WithContextOptionalFields(ctx).NewLogEntry(events.LogLevelINFO, e.deploymentID).Registerf("k8s Namespace %s created", job.namespace)
	}

	if job.jobRepr.Spec.Template.Spec.RestartPolicy != "Never" && job.jobRepr.Spec.Template.Spec.RestartPolicy != "OnFailure" {
		events.WithContextOptionalFields(ctx).NewLogEntry(events.LogLevelDEBUG, e.deploymentID).Registerf(`job RestartPolicy %q is invalid for a job, settings to "Never" by default`, job.jobRepr.Spec.Template.Spec.RestartPolicy)
		job.jobRepr.Spec.Template.Spec.RestartPolicy = "Never"
	}

	job.jobRepr, err = clientset.BatchV1().Jobs(job.namespace).Create(job.jobRepr)
	if err != nil {
		return errors.Wrapf(err, "failed to create job for node %q", e.nodeName)
	}

	// Set job id to the instance attribute
	err = deployments.SetAttributeForAllInstances(e.kv, e.deploymentID, e.nodeName, "job_id", job.jobRepr.Name)
	if err != nil {
		return errors.Wrapf(err, "failed to create job for node %q", e.nodeName)
	}

	return tasks.SetTaskData(e.kv, e.taskID, e.nodeName+"-jobId", job.jobRepr.Name)
}

func (e *execution) cancelJob(ctx context.Context, clientset kubernetes.Interface) error {
	jobID, err := tasks.GetTaskData(e.kv, e.taskID, e.nodeName+"-jobId")
	if err != nil {
		if !tasks.IsTaskDataNotFoundError(err) {
			return err
		}
		// Not cancelling within the same task try to get jobID from attribute
		_, jobID, err = deployments.GetInstanceAttribute(e.kv, e.deploymentID, e.nodeName, "0", "job_id")
		events.WithContextOptionalFields(ctx).NewLogEntry(events.LogLevelDEBUG, e.deploymentID).Registerf(
			"k8s job cancellation called from a dedicated \"cancel\" workflow. JobID retrieved from node %q attribute. This may cause issues if multiple workflows are running in parallel. Prefer using a workflow cancellation.", e.nodeName)
	}
	job, err := getJob(e.kv, e.deploymentID, e.nodeName)
	if err != nil {
		return errors.Wrapf(err, "failed to delete job for node %q", e.nodeName)
	}
	return deleteJob(ctx, e.deploymentID, job.namespace, jobID, job.namespaceProvided, clientset)
}

func (e *execution) executeJobOperation(ctx context.Context, clientset kubernetes.Interface) (err error) {
	switch strings.ToLower(e.operation.Name) {
	case "tosca.interfaces.node.lifecycle.runnable.submit":
		return e.submitJob(ctx, clientset)
	case "tosca.interfaces.node.lifecycle.runnable.cancel":
		return e.cancelJob(ctx, clientset)
	default:
		return errors.Errorf("unsupported operation %q for node %q", e.operation.Name, e.nodeName)
	}

}
