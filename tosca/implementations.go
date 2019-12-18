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

package tosca

// An Implementation is the representation of the implementation part of a TOSCA Operation Definition
//
// See http://docs.oasis-open.org/tosca/TOSCA-Simple-Profile-YAML/v1.2/TOSCA-Simple-Profile-YAML-v1.2.html#DEFN_ELEMENT_OPERATION_DEF for more details
type Implementation struct {
	Primary       string             `yaml:"primary" json:"primary"`
	Dependencies  []string           `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Artifact      ArtifactDefinition `yaml:",inline" json:"artifact,omitempty"`
	OperationHost string             `yaml:"operation_host,omitempty" json:"operation_host,omitempty"`
}

// UnmarshalYAML unmarshals a yaml into an Implementation
func (i *Implementation) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var err error
	var s string
	if err = unmarshal(&s); err == nil {
		i.Primary = s
		return nil
	}

	var str struct {
		Primary       string             `yaml:"primary,omitempty"`
		Dependencies  []string           `yaml:"dependencies,omitempty"`
		Artifact      ArtifactDefinition `yaml:",inline"`
		OperationHost string             `yaml:"operation_host,omitempty"`
	}
	if err = unmarshal(&str); err == nil {
		i.Primary = str.Primary
		i.Dependencies = str.Dependencies
		i.Artifact = str.Artifact
		i.OperationHost = str.OperationHost
		return nil
	}

	return err
}
