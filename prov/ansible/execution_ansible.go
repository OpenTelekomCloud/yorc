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

package ansible

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pkg/errors"

	"strings"

	"github.com/ystia/yorc/events"
	"github.com/ystia/yorc/helper/executil"
	"github.com/ystia/yorc/helper/stringutil"
	"github.com/ystia/yorc/log"
	"github.com/ystia/yorc/tasks"
)

const ansiblePlaybook = `
- name: Upload artifacts
  hosts: all
  strategy: free
  tasks:
[[[ range $artName, $art := .Artifacts ]]]    [[[printf "- file: path=\"{{ ansible_env.HOME}}/%s/%s\" state=directory mode=0755" $.OperationRemotePath (path $art)]]]
    [[[printf "- copy: src=\"%s/%s\" dest=\"{{ ansible_env.HOME}}/%s/%s\"" $.OverlayPath $art $.OperationRemotePath (path $art)]]]
[[[end]]]
- import_playbook: [[[.PlaybookPath]]]
[[[if .HaveOutput]]]
- name: Retrieving Operation outputs
  hosts: all
  strategy: free
  tasks:
    [[[printf "- file: path=\"{{ ansible_env.HOME}}/%s\" state=directory mode=0755" $.OperationRemotePath]]]
    [[[printf "- template: src=\"outputs.csv.j2\" dest=\"{{ ansible_env.HOME}}/%s/out.csv\"" $.OperationRemotePath]]]
    [[[printf "- fetch: src=\"{{ ansible_env.HOME}}/%s/out.csv\" dest={{dest_folder}}/{{ansible_host}}-out.csv flat=yes" $.OperationRemotePath]]]
[[[end]]]
[[[if not .KeepOperationRemotePath]]]
- name: Cleanup temp directories
  hosts: all
  strategy: free
  tasks:
    - file: path="{{ ansible_env.HOME}}/[[[.OperationRemoteBaseDir]]]" state=absent
[[[end]]]
`

type executionAnsible struct {
	*executionCommon
	PlaybookPath string
}

func (e *executionAnsible) runAnsible(ctx context.Context, retry bool, currentInstance, ansibleRecipePath string) error {
	var err error
	// Fill log optional fields for log registration
	wfName, _ := tasks.GetTaskData(e.kv, e.taskID, "workflowName")
	logOptFields := events.LogOptionalFields{
		events.WorkFlowID:    wfName,
		events.NodeID:        e.NodeName,
		events.OperationName: stringutil.GetLastElement(e.operation.Name, "."),
		events.InstanceID:    currentInstance,
		events.InterfaceName: stringutil.GetAllExceptLastElement(e.operation.Name, "."),
	}
	e.PlaybookPath, err = filepath.Abs(filepath.Join(e.OverlayPath, e.Primary))
	if err != nil {
		events.WithOptionalFields(logOptFields).NewLogEntry(events.ERROR, e.deploymentID).RegisterAsString(err.Error())
		return err
	}

	ansibleGroupsVarsPath := filepath.Join(ansibleRecipePath, "group_vars")
	if err = os.MkdirAll(ansibleGroupsVarsPath, 0775); err != nil {
		err = errors.Wrap(err, "Failed to create group_vars directory: ")
		events.WithOptionalFields(logOptFields).NewLogEntry(events.ERROR, e.deploymentID).RegisterAsString(err.Error())
		return err
	}
	var buffer bytes.Buffer
	for _, envInput := range e.EnvInputs {
		if envInput.InstanceName != "" {
			buffer.WriteString(envInput.InstanceName)
			buffer.WriteString("_")
		}
		buffer.WriteString(fmt.Sprintf("%s: %q", envInput.Name, envInput.Value))
		buffer.WriteString("\n")
	}

	for artName, art := range e.Artifacts {
		buffer.WriteString(artName)
		buffer.WriteString(": \"{{ansible_env.HOME}}/")
		buffer.WriteString(e.OperationRemotePath)
		buffer.WriteString("/")
		buffer.WriteString(art)
		buffer.WriteString("\"\n")
	}
	for contextKey, contextValue := range e.Context {
		buffer.WriteString(fmt.Sprintf("%s: %q", contextKey, contextValue))
		buffer.WriteString("\n")
	}
	buffer.WriteString("dest_folder: \"")
	buffer.WriteString(ansibleRecipePath)
	buffer.WriteString("\"\n")

	if err = ioutil.WriteFile(filepath.Join(ansibleGroupsVarsPath, "all.yml"), buffer.Bytes(), 0664); err != nil {
		err = errors.Wrap(err, "Failed to write global group vars file: ")
		events.WithOptionalFields(logOptFields).NewLogEntry(events.ERROR, e.deploymentID).RegisterAsString(err.Error())
		return err
	}

	if e.HaveOutput {
		buffer.Reset()
		for outputName := range e.Outputs {
			buffer.WriteString(outputName)
			buffer.WriteString(",{{")
			idx := strings.LastIndex(outputName, "_")
			buffer.WriteString(outputName[:idx])
			buffer.WriteString("}}\n")
		}
		if err = ioutil.WriteFile(filepath.Join(ansibleRecipePath, "outputs.csv.j2"), buffer.Bytes(), 0664); err != nil {
			err = errors.Wrap(err, "Failed to generate operation outputs file: ")
			events.WithOptionalFields(logOptFields).NewLogEntry(events.ERROR, e.deploymentID).RegisterAsString(err.Error())
			return err
		}
	}

	buffer.Reset()
	funcMap := template.FuncMap{
		// The name "path" is what the function will be called in the template text.
		"path": filepath.Dir,
		"abs":  filepath.Abs,
		"cut":  cutAfterLastUnderscore,
	}
	tmpl := template.New("execTemplate").Delims("[[[", "]]]").Funcs(funcMap)
	tmpl, err = tmpl.Parse(ansiblePlaybook)
	if err != nil {
		err = errors.Wrap(err, "Failed to generate ansible playbook")
		events.WithOptionalFields(logOptFields).NewLogEntry(events.ERROR, e.deploymentID).RegisterAsString(err.Error())
		return err
	}
	if err = tmpl.Execute(&buffer, e); err != nil {
		err = errors.Wrap(err, "Failed to Generate ansible playbook template")
		events.WithOptionalFields(logOptFields).NewLogEntry(events.ERROR, e.deploymentID).RegisterAsString(err.Error())
		return err
	}
	if err = ioutil.WriteFile(filepath.Join(ansibleRecipePath, "run.ansible.yml"), buffer.Bytes(), 0664); err != nil {
		err = errors.Wrap(err, "Failed to write playbook file")
		events.WithOptionalFields(logOptFields).NewLogEntry(events.ERROR, e.deploymentID).RegisterAsString(err.Error())
		return err
	}

	events.WithOptionalFields(logOptFields).NewLogEntry(events.DEBUG, e.deploymentID).RegisterAsString(fmt.Sprintf("Ansible recipe for node %q: executing %q on remote host(s)", e.NodeName, filepath.Base(e.PlaybookPath)))
	cmd := executil.Command(ctx, "ansible-playbook", "-i", "hosts", "run.ansible.yml")

	if _, err = os.Stat(filepath.Join(ansibleRecipePath, "run.ansible.retry")); retry && (err == nil || !os.IsNotExist(err)) {
		cmd.Args = append(cmd.Args, "--limit", filepath.Join("@", ansibleRecipePath, "run.ansible.retry"))
	}
	if e.cfg.Ansible.DebugExec {
		cmd.Args = append(cmd.Args, "-vvvv")
	}
	if e.cfg.Ansible.UseOpenSSH {
		cmd.Args = append(cmd.Args, "-c", "ssh")
	} else {
		cmd.Args = append(cmd.Args, "-c", "paramiko")
	}
	cmd.Dir = ansibleRecipePath
	var outbuf bytes.Buffer
	errbuf := events.NewBufferedLogEntryWriter()
	cmd.Stdout = &outbuf
	cmd.Stderr = errbuf

	errCloseCh := make(chan bool)
	defer close(errCloseCh)

	// Register log entry via error buffer
	events.WithOptionalFields(logOptFields).NewLogEntry(events.ERROR, e.deploymentID).RunBufferedRegistration(errbuf, errCloseCh)

	defer func(buffer *bytes.Buffer) {
		if err := e.logAnsibleOutputInConsul(buffer, logOptFields); err != nil {
			log.Printf("Failed to publish Ansible log %v", err)
			log.Debugf("%+v", err)
		}
	}(&outbuf)
	if err := cmd.Run(); err != nil {
		return e.checkAnsibleRetriableError(err, logOptFields)
	}

	return nil
}
