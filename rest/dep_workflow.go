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

package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
	"github.com/ystia/yorc/v4/deployments"
	"github.com/ystia/yorc/v4/helper/collections"
	"github.com/ystia/yorc/v4/log"
	"github.com/ystia/yorc/v4/tasks"
)

func (s *Server) newWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	var params httprouter.Params
	ctx := r.Context()
	params = ctx.Value(paramsLookupKey).(httprouter.Params)
	deploymentID := params.ByName("id")
	workflowName := params.ByName("workflowName")

	dExits, err := deployments.DoesDeploymentExists(ctx, deploymentID)
	if err != nil {
		log.Panicf("%v", err)
	}
	if !dExits {
		writeError(w, r, errNotFound)
		return
	}

	if !checkBlockingOperationOnDeployment(ctx, deploymentID, w, r) {
		return
	}

	deploymentStatus, err := deployments.GetDeploymentStatus(ctx, deploymentID)
	if err != nil {
		log.Panicf("%v", err)
	}
	if deploymentStatus == deployments.UPDATE_IN_PROGRESS {
		writeError(w, r, newConflictRequest("Workflow can't be executed as an update is in progress for this deployment"))
		return
	}

	workflows, err := deployments.GetWorkflows(ctx, deploymentID)
	if err != nil {
		log.Panic(err)
	}

	if !collections.ContainsString(workflows, workflowName) {
		writeError(w, r, errNotFound)
		return
	}

	data := make(map[string]string)
	data["workflowName"] = workflowName
	if _, ok := r.URL.Query()["continueOnError"]; ok {
		data["continueOnError"] = strconv.FormatBool(true)
	} else {
		data["continueOnError"] = strconv.FormatBool(false)
	}
	// Get instances selection if provided in the request body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Panic(err)
	}

	if len(body) > 0 {
		var wfRequest WorkflowRequest
		err = json.Unmarshal(body, &wfRequest)
		if err != nil {
			log.Panic(err)
		}
		for _, nodeInstances := range wfRequest.NodesInstances {
			nodeName := nodeInstances.NodeName
			// Check that provided node exists
			nodeExists, err := deployments.DoesNodeExist(ctx, deploymentID, nodeName)
			if err != nil {
				log.Panicf("%v", err)
			}
			if !nodeExists {
				writeError(w, r, newBadRequestParameter("node", errors.Errorf("Node %q must exist", nodeName)))
				return
			}
			// Check that provided instances exist
			checked, inexistent := s.checkInstances(ctx, deploymentID, nodeName, nodeInstances.Instances)
			if !checked {
				writeError(w, r, newBadRequestParameter("instance", errors.Errorf("Instance %q must exist", inexistent)))
				return
			}
			instances := strings.Join(nodeInstances.Instances, ",")
			data["nodes/"+nodeName] = instances
		}

		// Adding workflow inputs in task data
		for inputName, inputValue := range wfRequest.Inputs {
			data[path.Join("inputs", inputName)] = fmt.Sprintf("%v", inputValue)
		}

		// Check all workflow required input parameters have a value
		wf, err := deployments.GetWorkflow(ctx, deploymentID, workflowName)
		if err != nil {
			log.Panic(err)
		}
		if wf == nil {
			log.Panic(errors.Errorf("Can't check inputs of workflow %q in deployment %q, workflow definition not found", workflowName, deploymentID))
		}
		for inputName, def := range wf.Inputs {
			// A property is considered as required by default, unless def.Required
			// is set to false
			if (def.Required == nil || *def.Required) && def.Default == nil {
				_, found := wfRequest.Inputs[inputName]
				if !found {
					writeError(w, r, newBadRequestParameter("inputs", errors.Errorf("Missing value for required workflow input parameter %s", inputName)))
					return
				}
			}
		}

	}

	taskID, err := s.tasksCollector.RegisterTaskWithData(deploymentID, tasks.TaskTypeCustomWorkflow, data)
	if err != nil {
		if ok, _ := tasks.IsAnotherLivingTaskAlreadyExistsError(err); ok {
			writeError(w, r, newBadRequestError(err))
			return
		}
		log.Panic(err)
	}

	w.Header().Set("Location", fmt.Sprintf("/deployments/%s/tasks/%s", deploymentID, taskID))
	w.WriteHeader(http.StatusCreated)

}

func (s *Server) listWorkflowsHandler(w http.ResponseWriter, r *http.Request) {
	var params httprouter.Params
	ctx := r.Context()
	params = ctx.Value(paramsLookupKey).(httprouter.Params)
	deploymentID := params.ByName("id")

	dExits, err := deployments.DoesDeploymentExists(ctx, deploymentID)
	if err != nil {
		log.Panicf("%v", err)
	}
	if !dExits {
		writeError(w, r, errNotFound)
		return
	}

	workflows, err := deployments.GetWorkflows(ctx, deploymentID)
	if err != nil {
		log.Panic(err)
	}

	wfCol := WorkflowsCollection{Workflows: make([]AtomLink, len(workflows))}
	for i, wf := range workflows {
		wfCol.Workflows[i] = newAtomLink(LinkRelWorkflow, path.Join("/deployments", deploymentID, "workflows", wf))
	}
	encodeJSONResponse(w, r, wfCol)
}

func (s *Server) getWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	var params httprouter.Params
	ctx := r.Context()
	params = ctx.Value(paramsLookupKey).(httprouter.Params)
	deploymentID := params.ByName("id")
	workflowName := params.ByName("workflowName")

	dExits, err := deployments.DoesDeploymentExists(ctx, deploymentID)
	if err != nil {
		log.Panicf("%v", err)
	}
	if !dExits {
		writeError(w, r, errNotFound)
		return
	}

	workflows, err := deployments.GetWorkflows(ctx, deploymentID)
	if err != nil {
		log.Panic(err)
	}

	if !collections.ContainsString(workflows, workflowName) {
		writeError(w, r, errNotFound)
		return
	}
	wf, err := deployments.GetWorkflow(ctx, deploymentID, workflowName)
	if err != nil {
		log.Panic(err)
	}
	if wf == nil {
		log.Panic(errors.Errorf("Can't retrieve workflow %q in deployment %q, workflow definition not found", workflowName, deploymentID))
	}
	encodeJSONResponse(w, r, Workflow{Name: workflowName, Workflow: *wf})
}
