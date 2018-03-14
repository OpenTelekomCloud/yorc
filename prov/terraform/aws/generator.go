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

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/ystia/yorc/config"
	"github.com/ystia/yorc/deployments"
	"github.com/ystia/yorc/helper/consulutil"
	"github.com/ystia/yorc/log"
	"github.com/ystia/yorc/prov/terraform/commons"
	"github.com/ystia/yorc/tosca"
)

const infrastructureName = "aws"

type awsGenerator struct {
}

func (g *awsGenerator) GenerateTerraformInfraForNode(ctx context.Context, cfg config.Configuration, deploymentID, nodeName string) (bool, map[string]string, []string, error) {
	log.Debugf("Generating infrastructure for deployment with id %s", deploymentID)
	cClient, err := cfg.GetConsulClient()
	if err != nil {
		return false, nil, nil, err
	}
	kv := cClient.KV()
	nodeKey := path.Join(consulutil.DeploymentKVPrefix, deploymentID, "topology", "nodes", nodeName)
	terraformStateKey := path.Join(consulutil.DeploymentKVPrefix, deploymentID, "terraform-state", nodeName)

	infrastructure := commons.Infrastructure{}

	consulAddress := "127.0.0.1:8500"
	if cfg.Consul.Address != "" {
		consulAddress = cfg.Consul.Address
	}
	consulScheme := "http"
	if cfg.Consul.SSL {
		consulScheme = "https"
	}
	consulCA := ""
	if cfg.Consul.CA != "" {
		consulCA = cfg.Consul.CA
	}
	consulKey := ""
	if cfg.Consul.Key != "" {
		consulKey = cfg.Consul.Key
	}
	consulCert := ""
	if cfg.Consul.Cert != "" {
		consulCert = cfg.Consul.Cert
	}

	// Remote Configuration for Terraform State to store it in the Consul KV store
	infrastructure.Terraform = map[string]interface{}{
		"backend": map[string]interface{}{
			"consul": map[string]interface{}{
				"path": terraformStateKey,
			},
		},
	}
	cmdEnv := []string{
		fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", cfg.Infrastructures[infrastructureName].GetString("access_key")),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", cfg.Infrastructures[infrastructureName].GetString("secret_key")),
	}
	// Management of variables for Terraform
	infrastructure.Provider = map[string]interface{}{
		"aws": map[string]interface{}{
			"region": cfg.Infrastructures[infrastructureName].GetString("region"),
		},
		"consul": map[string]interface{}{
			"address":   consulAddress,
			"scheme":    consulScheme,
			"ca_file":   consulCA,
			"cert_file": consulCert,
			"key_file":  consulKey,
		},
	}

	log.Debugf("inspecting node %s", nodeKey)
	nodeType, err := deployments.GetNodeType(kv, deploymentID, nodeName)
	if err != nil {
		return false, nil, nil, err
	}
	outputs := make(map[string]string)
	var instances []string
	switch nodeType {
	case "yorc.nodes.aws.Compute":
		instances, err = deployments.GetNodeInstancesIds(kv, deploymentID, nodeName)
		if err != nil {
			return false, nil, nil, err
		}

		for _, instanceName := range instances {
			var instanceState tosca.NodeState
			instanceState, err = deployments.GetInstanceState(kv, deploymentID, nodeName, instanceName)
			if err != nil {
				return false, nil, nil, err
			}
			if instanceState == tosca.NodeStateDeleting || instanceState == tosca.NodeStateDeleted {
				// Do not generate something for this node instance (will be deleted if exists)
				continue
			}
			err = g.generateAWSInstance(ctx, kv, cfg, deploymentID, nodeName, instanceName, &infrastructure, outputs)
			if err != nil {
				return false, nil, nil, err
			}
		}

	case "yorc.nodes.aws.PublicNetwork":
		// Nothing to do
	default:
		return false, nil, nil, errors.Errorf("Unsupported node type '%s' for node '%s' in deployment '%s'", nodeType, nodeName, deploymentID)
	}

	jsonInfra, err := json.MarshalIndent(infrastructure, "", "  ")
	if err != nil {
		return false, nil, nil, errors.Wrap(err, "Failed to generate JSON of terraform Infrastructure description")
	}
	infraPath := filepath.Join(cfg.WorkingDirectory, "deployments", fmt.Sprint(deploymentID), "infra", nodeName)
	if err = os.MkdirAll(infraPath, 0775); err != nil {
		return false, nil, nil, errors.Wrapf(err, "Failed to create infrastructure working directory %q", infraPath)
	}

	if err = ioutil.WriteFile(filepath.Join(infraPath, "infra.tf.json"), jsonInfra, 0664); err != nil {
		return false, nil, nil, errors.Wrapf(err, "Failed to write file %q", filepath.Join(infraPath, "infra.tf.json"))
	}

	log.Debugf("Infrastructure generated for deployment with id %s", deploymentID)
	return true, outputs, cmdEnv, nil
}
