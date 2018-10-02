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

package scheduling

import (
	"github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
	"github.com/ystia/yorc/helper/consulutil"
	"github.com/ystia/yorc/log"
	"github.com/ystia/yorc/prov"
	"path"
	"strings"
	"time"
)

// RegisterAction allows to register a scheduled action and to start scheduling it
func RegisterAction(client *api.Client, deploymentID string, timeInterval time.Duration, action *prov.Action) (string, error) {
	log.Debugf("Action:%+v has been requested to be registered for scheduling with [deploymentID:%q, timeInterval:%q]", action, deploymentID, timeInterval.String())
	id := uuid.NewV4().String()

	// Check mandatory parameters
	if deploymentID == "" {
		return "", errors.New("deploymentID is mandatory parameter to register scheduled action")
	}
	if action == nil || action.ActionType == "" {
		return "", errors.New("actionType is mandatory parameter to register scheduled action")
	}
	scaPath := path.Join(consulutil.SchedulingKVPrefix, "actions", id)
	scaOps := api.KVTxnOps{
		&api.KVTxnOp{
			Verb:  api.KVSet,
			Key:   path.Join(scaPath, "deploymentID"),
			Value: []byte(deploymentID),
		},
		&api.KVTxnOp{
			Verb:  api.KVSet,
			Key:   path.Join(scaPath, "type"),
			Value: []byte(action.ActionType),
		},
		&api.KVTxnOp{
			Verb:  api.KVSet,
			Key:   path.Join(scaPath, "interval"),
			Value: []byte(timeInterval.String()),
		},
	}

	if action.Data != nil {
		for k, v := range action.Data {
			scaOps = append(scaOps, &api.KVTxnOp{
				Verb:  api.KVSet,
				Key:   path.Join(scaPath, k),
				Value: []byte(v),
			})
		}
	}

	ok, response, _, err := client.KV().Txn(scaOps, nil)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to register scheduled action for deploymentID:%q, type:%q, id:%q", deploymentID, action.ActionType, id)
	}
	if !ok {
		// Check the response
		errs := make([]string, 0)
		for _, e := range response.Errors {
			errs = append(errs, e.What)
		}
		return "", errors.Errorf("Failed to register scheduled action for deploymentID:%q, type:%q, id:%q due to:%s", deploymentID, action.ActionType, id, strings.Join(errs, ", "))
	}
	return id, nil
}

// UnregisterAction allows to unregister a scheduled action and to stop scheduling it
func UnregisterAction(client *api.Client, id string) error {
	log.Debugf("Unregister scheduled action with id:%q", id)
	scaPath := path.Join(consulutil.SchedulingKVPrefix, "actions", id)
	kvp := &api.KVPair{Key: path.Join(scaPath, ".unregisterFlag"), Value: []byte("true")}
	_, err := client.KV().Put(kvp, nil)
	return errors.Wrap(err, "Failed to flag scheduled action for removal")
}
