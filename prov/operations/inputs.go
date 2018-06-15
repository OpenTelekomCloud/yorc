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

package operations

import (
	"fmt"

	"github.com/hashicorp/consul/api"

	"github.com/ystia/yorc/deployments"
	"github.com/ystia/yorc/helper/provutil"
	"github.com/ystia/yorc/log"
	"github.com/ystia/yorc/prov"
	"github.com/ystia/yorc/tasks"
)

// An EnvInput represent a TOSCA operation input
//
// This element is exported in order to be used by text.Template but should be consider as internal
type EnvInput struct {
	Name         string
	Value        string
	InstanceName string
}

func (ei EnvInput) String() string {
	return fmt.Sprintf("EnvInput: [Name: %q, Value: %q, InstanceName: %q]", ei.Name, ei.Value, ei.InstanceName)
}

// ResolveInputs allows to resolve inputs for an operation
func ResolveInputs(kv *api.KV, deploymentID, nodeName, taskID string, operation prov.Operation) ([]*EnvInput, []string, error) {
	sourceInstances, err := tasks.GetInstances(kv, taskID, deploymentID, nodeName)
	if err != nil {
		return nil, nil, err
	}

	var targetInstances []string
	if operation.RelOp.IsRelationshipOperation {
		targetInstances, err = tasks.GetInstances(kv, taskID, deploymentID, operation.RelOp.TargetNodeName)
		if err != nil {
			return nil, nil, err
		}
	}
	return ResolveInputsWithInstances(kv, deploymentID, nodeName, taskID, operation, sourceInstances, targetInstances)
}

// ResolveInputsWithInstances used to resolve inputs for an operation
func ResolveInputsWithInstances(kv *api.KV, deploymentID, nodeName, taskID string, operation prov.Operation,
	sourceNodeInstances, targetNodeInstances []string) ([]*EnvInput, []string, error) {

	log.Debug("resolving inputs")

	envInputs := make([]*EnvInput, 0)
	varInputsNames := make([]string, 0)

	inputKeys, err := deployments.GetOperationInputs(kv, deploymentID, operation.ImplementedInNodeTemplate, operation.ImplementedInType, operation.Name)
	if err != nil {
		return nil, nil, err
	}

	for _, input := range inputKeys {
		isPropDef, err := deployments.IsOperationInputAPropertyDefinition(kv, deploymentID, operation.ImplementedInNodeTemplate, operation.ImplementedInType, operation.Name, input)
		if err != nil {
			return nil, nil, err
		}

		if isPropDef {
			inputValue, err := tasks.GetTaskInput(kv, taskID, input)
			if err != nil {
				if !tasks.IsTaskDataNotFoundError(err) {
					return nil, nil, err
				}
				defaultInputValues, err := deployments.GetOperationInputPropertyDefinitionDefault(kv, deploymentID, nodeName, operation, input)
				if err != nil {
					return nil, nil, err
				}
				for i, iv := range defaultInputValues {
					envInputs = append(envInputs, &EnvInput{Name: input, InstanceName: GetInstanceName(iv.NodeName, iv.InstanceName), Value: iv.Value})
					if i == 0 {
						varInputsNames = append(varInputsNames, provutil.SanitizeForShell(input))
					}
				}
				continue
			}
			instances, err := deployments.GetNodeInstancesIds(kv, deploymentID, nodeName)
			if err != nil {
				return nil, nil, err
			}
			for i, ins := range instances {
				envInputs = append(envInputs, &EnvInput{Name: input, InstanceName: GetInstanceName(nodeName, ins), Value: inputValue})
				if i == 0 {
					varInputsNames = append(varInputsNames, provutil.SanitizeForShell(input))
				}
			}
		} else {
			inputValues, err := deployments.GetOperationInput(kv, deploymentID, nodeName, operation, input)
			if err != nil {
				return nil, nil, err
			}
			for i, iv := range inputValues {
				envInputs = append(envInputs, &EnvInput{Name: input, InstanceName: GetInstanceName(iv.NodeName, iv.InstanceName), Value: iv.Value})
				if i == 0 {
					varInputsNames = append(varInputsNames, provutil.SanitizeForShell(input))
				}
			}
		}
	}

	log.Debugf("Resolved env inputs: %s", envInputs)
	return envInputs, varInputsNames, nil
}
