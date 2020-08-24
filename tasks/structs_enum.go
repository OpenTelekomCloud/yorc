// Copyright 2019 Bull S.A.S. Atos Technologies - Bull, Rue Jean Jaures, B.P.68, 78340, Les Clayes-sous-Bois, France.
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

// Code generated by go-enum
// DO NOT EDIT!

package tasks

import (
	"fmt"
)

const (
	// TaskStatusINITIAL is a TaskStatus of type INITIAL
	TaskStatusINITIAL TaskStatus = iota
	// TaskStatusRUNNING is a TaskStatus of type RUNNING
	TaskStatusRUNNING
	// TaskStatusDONE is a TaskStatus of type DONE
	TaskStatusDONE
	// TaskStatusFAILED is a TaskStatus of type FAILED
	TaskStatusFAILED
	// TaskStatusCANCELED is a TaskStatus of type CANCELED
	TaskStatusCANCELED
)

const _TaskStatusName = "INITIALRUNNINGDONEFAILEDCANCELED"

var _TaskStatusMap = map[TaskStatus]string{
	0: _TaskStatusName[0:7],
	1: _TaskStatusName[7:14],
	2: _TaskStatusName[14:18],
	3: _TaskStatusName[18:24],
	4: _TaskStatusName[24:32],
}

// String implements the Stringer interface.
func (x TaskStatus) String() string {
	if str, ok := _TaskStatusMap[x]; ok {
		return str
	}
	return fmt.Sprintf("TaskStatus(%d)", x)
}

var _TaskStatusValue = map[string]TaskStatus{
	_TaskStatusName[0:7]:   0,
	_TaskStatusName[7:14]:  1,
	_TaskStatusName[14:18]: 2,
	_TaskStatusName[18:24]: 3,
	_TaskStatusName[24:32]: 4,
}

// ParseTaskStatus attempts to convert a string to a TaskStatus
func ParseTaskStatus(name string) (TaskStatus, error) {
	if x, ok := _TaskStatusValue[name]; ok {
		return x, nil
	}
	return TaskStatus(0), fmt.Errorf("%s is not a valid TaskStatus", name)
}

const (
	// TaskTypeDeploy is a TaskType of type Deploy
	TaskTypeDeploy TaskType = iota
	// TaskTypeUnDeploy is a TaskType of type UnDeploy
	TaskTypeUnDeploy
	// TaskTypeScaleOut is a TaskType of type ScaleOut
	TaskTypeScaleOut
	// TaskTypeScaleIn is a TaskType of type ScaleIn
	TaskTypeScaleIn
	// TaskTypePurge is a TaskType of type Purge
	TaskTypePurge
	// TaskTypeCustomCommand is a TaskType of type CustomCommand
	TaskTypeCustomCommand
	// TaskTypeCustomWorkflow is a TaskType of type CustomWorkflow
	TaskTypeCustomWorkflow
	// TaskTypeQuery is a TaskType of type Query
	TaskTypeQuery
	// TaskTypeAction is a TaskType of type Action
	TaskTypeAction
	// TaskTypeForcePurge is a TaskType of type ForcePurge
	// ForcePurge is deprecated and should not be used anymore this stay here to prevent task renumbering colision
	TaskTypeForcePurge
	// TaskTypeAddNodes is a TaskType of type AddNodes
	TaskTypeAddNodes
	// TaskTypeRemoveNodes is a TaskType of type RemoveNodes
	TaskTypeRemoveNodes
)

const _TaskTypeName = "DeployUnDeployScaleOutScaleInPurgeCustomCommandCustomWorkflowQueryActionForcePurgeAddNodesRemoveNodes"

var _TaskTypeMap = map[TaskType]string{
	0:  _TaskTypeName[0:6],
	1:  _TaskTypeName[6:14],
	2:  _TaskTypeName[14:22],
	3:  _TaskTypeName[22:29],
	4:  _TaskTypeName[29:34],
	5:  _TaskTypeName[34:47],
	6:  _TaskTypeName[47:61],
	7:  _TaskTypeName[61:66],
	8:  _TaskTypeName[66:72],
	9:  _TaskTypeName[72:82],
	10: _TaskTypeName[82:90],
	11: _TaskTypeName[90:101],
}

// String implements the Stringer interface.
func (x TaskType) String() string {
	if str, ok := _TaskTypeMap[x]; ok {
		return str
	}
	return fmt.Sprintf("TaskType(%d)", x)
}

var _TaskTypeValue = map[string]TaskType{
	_TaskTypeName[0:6]:    0,
	_TaskTypeName[6:14]:   1,
	_TaskTypeName[14:22]:  2,
	_TaskTypeName[22:29]:  3,
	_TaskTypeName[29:34]:  4,
	_TaskTypeName[34:47]:  5,
	_TaskTypeName[47:61]:  6,
	_TaskTypeName[61:66]:  7,
	_TaskTypeName[66:72]:  8,
	_TaskTypeName[72:82]:  9,
	_TaskTypeName[82:90]:  10,
	_TaskTypeName[90:101]: 11,
}

// ParseTaskType attempts to convert a string to a TaskType
func ParseTaskType(name string) (TaskType, error) {
	if x, ok := _TaskTypeValue[name]; ok {
		return x, nil
	}
	return TaskType(0), fmt.Errorf("%s is not a valid TaskType", name)
}
