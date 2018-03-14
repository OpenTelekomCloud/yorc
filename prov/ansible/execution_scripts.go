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

const scriptCustomWrapper = `#!/usr/bin/env bash
# Workaround JSON structures being treated as python objects
# basically it prevent double quotes to be changed into single quotes
# by prefixing the value by a space
# https://stackoverflow.com/questions/31969872/why-ansible-always-replaces-double-quotes-with-single-quotes-in-templates
# We remove this space here. Obviously becomes a reserved keyword yorc_escape_workaround
for yorc_escape_workaround in [[[StringsJoin .VarInputsNames " "]]] ;
do
  eval "[[ \"\${${yorc_escape_workaround}}\" == \" \"* ]] && { export ${yorc_escape_workaround}=\${${yorc_escape_workaround}:1};}"
done
[[[printf ". $HOME/%s/%s" $.OperationRemotePath .BasePrimary]]]
[[[range $artName, $art := .Outputs -]]]
[[[printf "echo %s,$%s >> $HOME/%s/out.csv" $artName (cut $artName) $.OperationRemotePath]]]
[[[printf "echo $%s" $artName]]]
[[[end]]]
[[[if .HaveOutput]]]
[[[printf "chmod 777 $HOME/%s/out.csv" $.OperationRemotePath]]]
[[[end]]]
`

const pythonCustomWrapper = `#!/usr/bin/env python

from os import chmod
from os import environ
from os.path import expanduser
import sys

home = expanduser("~")
# Workaround JSON structures being treated as python objects
# basically it prevent double quotes to be changed into single quotes
# by prefixing the value by a space
# https://stackoverflow.com/questions/31969872/why-ansible-always-replaces-double-quotes-with-single-quotes-in-templates
# We remove this space here. Obviously becomes a reserved keyword yorc_escape_workaround
gVar = {}
names=[ [[[qJoin .VarInputsNames]]] ]

for n in names:
	val = environ[n]
	if val.startswith(' '):
		val=val[1:]
		environ[n]=val
	gVar[n]=val

names=[ [[[range $eidx, $ei := .EnvInputs]]][[[if ($eidx) gt 0]]], [[[end]]]"[[[if (len $ei.InstanceName) gt 0]]][[[printf "%s_" $ei.InstanceName]]][[[end]]][[[$ei.Name]]]"[[[end]]] ]
names.extend([ [[[qJoinKeys .Artifacts]]] ])
names.extend([ [[[qJoinKeys .Context]]] ])
for n in names:
	val = environ[n]
	gVar[n]=val

fName="{0}/{1}/{2}".format(home, [[[printf "%q, %q"  $.OperationRemotePath .BasePrimary]]])
exec(compile(open(fName, "rb").read(), fName, 'exec'), gVar)

[[[if .HaveOutput]]]
import csv
outFile="{0}/{1}/out.csv".format(home, [[[printf "%q"  $.OperationRemotePath]]])
with open(outFile, 'w') as csvfile:
	writer=csv.writer(csvfile, delimiter=',')
	[[[range $outName, $outVal := .Outputs -]]]
	if '[[[print (cut $outName)]]]' not in gVar:
		sys.exit("Error: {0} doesn't define the required output value '[[[print (cut $outName)]]]'".format(fName))
	writer.writerow([ '[[[print $outName]]]', gVar['[[[print (cut $outName)]]]'] ])
	[[[end]]]
	chmod(outFile, 0777)
[[[end]]]

`

func quoteAndComaJoin(s []string) string {
	var b bytes.Buffer
	for i, e := range s {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteRune('"')
		b.WriteString(e)
		b.WriteRune('"')
	}
	return b.String()
}

func quoteAndComaJoinMapKeys(m map[string]string) string {
	l := make([]string, 0, len(m))
	for k := range m {
		l = append(l, k)
	}
	return quoteAndComaJoin(l)
}

const shellAnsiblePlaybook = `
- name: Executing script {{ script_to_run }}
  hosts: all
  strategy: free
  tasks:
    - file: path="{{ ansible_env.HOME}}/[[[.OperationRemotePath]]]" state=directory mode=0755
    [[[printf  "- copy: src=\"{{ wrapper_location }}\" dest=\"{{ ansible_env.HOME}}/%s/wrapper\" mode=0744" $.OperationRemotePath]]]
    - copy: src="{{ script_to_run }}" dest="{{ ansible_env.HOME}}/[[[.OperationRemotePath]]]" mode=0744
    [[[ range $artName, $art := .Artifacts -]]]
    [[[printf "- file: path=\"{{ ansible_env.HOME}}/%s/%s\" state=directory mode=0755" $.OperationRemotePath (path $art)]]]
    [[[printf "- copy: src=\"%s/%s\" dest=\"{{ ansible_env.HOME}}/%s/%s\"" $.OverlayPath $art $.OperationRemotePath (path $art)]]]
    [[[end]]]
    [[[printf "- shell: \"/bin/bash -l -c {{ ansible_env.HOME}}/%s/wrapper\"" $.OperationRemotePath]]]
      environment:
        [[[ range $key, $envInput := .EnvInputs -]]]
        [[[ if (len $envInput.InstanceName) gt 0]]][[[ if (len $envInput.Value) gt 0]]][[[printf  "%s_%s: %q" $envInput.InstanceName $envInput.Name $envInput.Value]]][[[else]]][[[printf  "%s_%s: \"\"" $envInput.InstanceName $envInput.Name]]]
        [[[end]]][[[else]]][[[ if (len $envInput.Value) gt 0]]][[[printf  "%s: %q" $envInput.Name $envInput.Value]]][[[else]]]
        [[[printf  "%s: \"\"" $envInput.Name]]]
        [[[end]]][[[end]]]
        [[[end]]][[[ range $artName, $art := .Artifacts -]]]
        [[[printf "%s: \"{{ ansible_env.HOME}}/%s/%s\"" $artName $.OperationRemotePath $art]]]
        [[[end]]][[[ range $contextK, $contextV := .Context -]]]
        [[[printf "%s: %q" $contextK $contextV]]]
        [[[end]]][[[ range $hostVarIndex, $hostVarValue := .VarInputsNames -]]]
        [[[printf "%s: \" {{%s}}\"" $hostVarValue $hostVarValue]]]
        [[[end]]]
    [[[if .HaveOutput]]]
    [[[printf "- fetch: src={{ ansible_env.HOME}}/%s/out.csv dest={{dest_folder}}/{{ansible_host}}-out.csv flat=yes" $.OperationRemotePath]]]
    [[[end]]]
    [[[if not .KeepOperationRemotePath ]]]
    - file: path="{{ ansible_env.HOME}}/[[[.OperationRemoteBaseDir]]]" state=absent
    [[[end]]]
`

type executionScript struct {
	*executionCommon
	isPython bool
}

func cutAfterLastUnderscore(str string) string {
	idx := strings.LastIndex(str, "_")
	return str[:idx]
}

func (e *executionScript) runAnsible(ctx context.Context, retry bool, currentInstance, ansibleRecipePath string) error {
	// Fill log optional fields for log registration
	wfName, _ := tasks.GetTaskData(e.kv, e.taskID, "workflowName")
	logOptFields := events.LogOptionalFields{
		events.WorkFlowID:    wfName,
		events.NodeID:        e.NodeName,
		events.OperationName: stringutil.GetLastElement(e.operation.Name, "."),
		events.InstanceID:    currentInstance,
		events.InterfaceName: stringutil.GetAllExceptLastElement(e.operation.Name, "."),
	}

	var buffer bytes.Buffer
	funcMap := template.FuncMap{
		// The name "path" is what the function will be called in the template text.
		"path":        filepath.Dir,
		"abs":         filepath.Abs,
		"cut":         cutAfterLastUnderscore,
		"StringsJoin": strings.Join,
		"qJoin":       quoteAndComaJoin,
		"qJoinKeys":   quoteAndComaJoinMapKeys,
	}

	tmpl := template.New("execTemplate")
	tmpl = tmpl.Delims("[[[", "]]]")
	tmpl = tmpl.Funcs(funcMap)
	wrapTemplate := template.New("execTemplate")
	wrapTemplate = wrapTemplate.Delims("[[[", "]]]")
	var err error
	if e.isPython {
		wrapTemplate, err = tmpl.Parse(pythonCustomWrapper)
	} else {
		wrapTemplate, err = tmpl.Parse(scriptCustomWrapper)
	}
	if err != nil {
		return err
	}
	if err := wrapTemplate.Execute(&buffer, e); err != nil {
		err = errors.Wrap(err, "Failed to Generate wrapper template")
		events.WithOptionalFields(logOptFields).NewLogEntry(events.ERROR, e.deploymentID).RegisterAsString(err.Error())
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(ansibleRecipePath, "wrapper"), buffer.Bytes(), 0664); err != nil {
		err = errors.Wrap(err, "Failed to write playbook file")
		events.WithOptionalFields(logOptFields).NewLogEntry(events.ERROR, e.deploymentID).RegisterAsString(err.Error())
		return err
	}

	buffer.Reset()
	tmpl, err = tmpl.Parse(shellAnsiblePlaybook)
	if err != nil {
		err = errors.Wrap(err, "Failed to Generate ansible playbook")
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

	scriptPath, err := filepath.Abs(filepath.Join(e.OverlayPath, e.Primary))
	if err != nil {
		return err
	}

	events.WithOptionalFields(logOptFields).NewLogEntry(events.DEBUG, e.deploymentID).RegisterAsString(fmt.Sprintf("Ansible recipe for node %q: executing %q on remote host(s)", e.NodeName, filepath.Base(scriptPath)))
	var cmd *executil.Cmd
	wrapperPath, err := filepath.Abs(ansibleRecipePath)
	if err != nil {
		return errors.Wrap(err, "Failed to retrieve script wrapper absolute path")
	}
	cmd = executil.Command(ctx, "ansible-playbook", "-i", "hosts", "run.ansible.yml", "--extra-vars", fmt.Sprintf("script_to_run=%s , wrapper_location=%s/wrapper , dest_folder=%s", scriptPath, wrapperPath, wrapperPath))
	if _, err = os.Stat(filepath.Join(ansibleRecipePath, "run.ansible.retry")); retry && (err == nil || !os.IsNotExist(err)) {
		cmd.Args = append(cmd.Args, "--limit", filepath.Join("@", ansibleRecipePath, "run.ansible.retry"))
	}
	if e.cfg.Ansible.DebugExec {
		cmd.Args = append(cmd.Args, "-vvvvv")
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
