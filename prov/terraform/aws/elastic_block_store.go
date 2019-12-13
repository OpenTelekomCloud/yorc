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
	"strings"

	"github.com/pkg/errors"
	"github.com/ystia/yorc/v4/config"
	"github.com/ystia/yorc/v4/deployments"
	"github.com/ystia/yorc/v4/helper/sizeutil"
	"github.com/ystia/yorc/v4/log"
	"github.com/ystia/yorc/v4/prov/terraform/commons"
)

func (g *awsGenerator) generateEBS(ctx context.Context, cfg config.Configuration, deploymentID string, nodeName string,
	instanceName string, instanceID int, infrastructure *commons.Infrastructure) error {

	// Verify NodeType
	nodeType, err := deployments.GetNodeType(ctx, deploymentID, nodeName)
	if err != nil {
		return err
	}
	if nodeType != "yorc.nodes.aws.EBSVolume" {
		return errors.Errorf("Unsupported node type for %q: %s", nodeName, nodeType)
	}

	ebs := &EBSVolume{}

	// Get string params
	var size string
	stringParams := []struct {
		pAttr        *string
		propertyName string
		mandatory    bool
	}{
		{&ebs.AvailabilityZone, "availability_zone", true},
		{&ebs.SnapshotID, "snapshot_id", false},
		{&ebs.KMSKeyID, "kms_key_id", false},
		{&size, "size", false},
	}

	for _, stringParam := range stringParams {
		if *stringParam.pAttr, err = deployments.GetStringNodeProperty(ctx, deploymentID, nodeName,
			stringParam.propertyName, stringParam.mandatory); err != nil {
			return err
		}
	}

	// Get bool params
	boolParams := []struct {
		pAtrr        *bool
		propertyName string
	}{
		{&ebs.Encrypted, "encrypted"},
	}

	for _, boolParam := range boolParams {
		if *boolParam.pAtrr, err = deployments.GetBooleanNodeProperty(ctx, deploymentID, nodeName,
			boolParam.propertyName); err != nil {
			return err
		}
	}

	// Handle size (eg : convertion)
	if size != "" {
		// Default size unit is MB
		log.Debugf("Initial size property value (default is MB): %q", size)
		ebs.Size, err = sizeutil.ConvertToGB(size)
		if err != nil {
			return err
		}
		log.Debugf("Computed size (in GB): %d", ebs.Size)
	}

	// If a an encryption key is given, considered the param "encrypted" to be true
	if ebs.KMSKeyID != "" && ebs.Encrypted == false {
		ebs.Encrypted = true
	}

	// Create the name for the ressource
	name := strings.ToLower(commons.GetResourcesPrefix(cfg, deploymentID) + nodeName + "-" + instanceName)
	commons.AddResource(infrastructure, "aws_ebs_volume", name, ebs)

	// TODO : provide outputs !

	return nil
}
