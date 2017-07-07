package tasks

import (
	"reflect"
	"testing"

	"novaforge.bull.com/starlings-janus/janus/helper/consulutil"
	"novaforge.bull.com/starlings-janus/janus/log"

	"path"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil"
	"github.com/stretchr/testify/require"
)

func TestTasks(t *testing.T) {
	t.Parallel()
	log.SetDebug(true)
	srv1, err := testutil.NewTestServer()
	if err != nil {
		t.Fatalf("Failed to create consul server: %v", err)
	}
	defer srv1.Stop()

	consulConfig := api.DefaultConfig()
	consulConfig.Address = srv1.HTTPAddr

	client, err := api.NewClient(consulConfig)
	require.Nil(t, err)

	kv := client.KV()

	srv1.PopulateKV(t, map[string][]byte{
		consulutil.TasksPrefix + "/t1/targetId":        []byte("id1"),
		consulutil.TasksPrefix + "/t1/status":          []byte("0"),
		consulutil.TasksPrefix + "/t1/type":            []byte("0"),
		consulutil.TasksPrefix + "/t1/inputs/i0":       []byte("0"),
		consulutil.TasksPrefix + "/t1/nodes/node1":     []byte("0,1,2"),
		consulutil.TasksPrefix + "/t2/targetId":        []byte("id1"),
		consulutil.TasksPrefix + "/t2/status":          []byte("1"),
		consulutil.TasksPrefix + "/t2/type":            []byte("1"),
		consulutil.TasksPrefix + "/t3/targetId":        []byte("id2"),
		consulutil.TasksPrefix + "/t3/status":          []byte("2"),
		consulutil.TasksPrefix + "/t3/type":            []byte("2"),
		consulutil.TasksPrefix + "/t3/nodes/n1":        []byte("2"),
		consulutil.TasksPrefix + "/t3/nodes/n2":        []byte("2"),
		consulutil.TasksPrefix + "/t3/nodes/n3":        []byte("2"),
		consulutil.TasksPrefix + "/t4/targetId":        []byte("id1"),
		consulutil.TasksPrefix + "/t4/status":          []byte("3"),
		consulutil.TasksPrefix + "/t4/type":            []byte("3"),
		consulutil.TasksPrefix + "/t5/targetId":        []byte("id"),
		consulutil.TasksPrefix + "/t5/status":          []byte("4"),
		consulutil.TasksPrefix + "/t5/type":            []byte("4"),
		consulutil.TasksPrefix + "/tCustomWF/targetId": []byte("id"),
		consulutil.TasksPrefix + "/tCustomWF/status":   []byte("0"),
		consulutil.TasksPrefix + "/tCustomWF/type":     []byte("6"),
		consulutil.TasksPrefix + "/t6/targetId":        []byte("id"),
		consulutil.TasksPrefix + "/t6/status":          []byte("5"),
		consulutil.TasksPrefix + "/t6/type":            []byte("5"),
		consulutil.TasksPrefix + "/t7/targetId":        []byte("id"),
		consulutil.TasksPrefix + "/t7/status":          []byte("5"),
		consulutil.TasksPrefix + "/t7/type":            []byte("6666"),
		consulutil.TasksPrefix + "/tNotInt/targetId":   []byte("targetNotInt"),
		consulutil.TasksPrefix + "/tNotInt/status":     []byte("not a status"),
		consulutil.TasksPrefix + "/tNotInt/type":       []byte("not a type"),

		consulutil.DeploymentKVPrefix + "/id1/topology/instances/node2/0/id": []byte("0"),
		consulutil.DeploymentKVPrefix + "/id1/topology/instances/node2/1/id": []byte("1"),
	})

	t.Run("tasks", func(t *testing.T) {
		t.Run("GetTasksIdsForTarget", func(t *testing.T) {
			testGetTasksIdsForTarget(t, kv)
		})
		t.Run("GetTaskStatus", func(t *testing.T) {
			testGetTaskStatus(t, kv)
		})
		t.Run("GetTaskType", func(t *testing.T) {
			testGetTaskType(t, kv)
		})
		t.Run("GetTaskTarget", func(t *testing.T) {
			testGetTaskTarget(t, kv)
		})
		t.Run("TaskExists", func(t *testing.T) {
			testTaskExists(t, kv)
		})
		t.Run("CancelTask", func(t *testing.T) {
			testCancelTask(t, kv)
		})
		t.Run("TargetHasLivingTasks", func(t *testing.T) {
			testTargetHasLivingTasks(t, kv)
		})
		t.Run("GetTaskInput", func(t *testing.T) {
			testGetTaskInput(t, kv)
		})
		t.Run("GetInstances", func(t *testing.T) {
			testGetInstances(t, kv)
		})
		t.Run("GetTaskRelatedNodes", func(t *testing.T) {
			testGetTaskRelatedNodes(t, kv)
		})
		t.Run("testIsTaskRelatedNode", func(t *testing.T) {
			testIsTaskRelatedNode(t, kv)
		})

	})
}

func testGetTasksIdsForTarget(t *testing.T, kv *api.KV) {
	type args struct {
		kv       *api.KV
		targetID string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{"TestMultiTargets", args{kv, "id1"}, []string{"t1", "t2", "t4"}, false},
		{"TestSingleTarget", args{kv, "id2"}, []string{"t3"}, false},
		{"TestNoTarget", args{kv, "idDoesntExist"}, []string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTasksIdsForTarget(tt.args.kv, tt.args.targetID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTasksIdsForTarget() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetTasksIdsForTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func testGetTaskStatus(t *testing.T, kv *api.KV) {
	type args struct {
		kv     *api.KV
		taskID string
	}
	tests := []struct {
		name    string
		args    args
		want    TaskStatus
		wantErr bool
	}{
		{"StatusINITIAL", args{kv, "t1"}, INITIAL, false},
		{"StatusRUNNING", args{kv, "t2"}, RUNNING, false},
		{"StatusDONE", args{kv, "t3"}, DONE, false},
		{"StatusFAILED", args{kv, "t4"}, FAILED, false},
		{"StatusCANCELED", args{kv, "t5"}, CANCELED, false},
		{"StatusDoesntExist", args{kv, "t6"}, FAILED, true},
		{"StatusNotInt", args{kv, "tNotInt"}, FAILED, true},
		{"TaskDoesntExist", args{kv, "TaskDoesntExist"}, FAILED, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTaskStatus(tt.args.kv, tt.args.taskID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTaskStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetTaskStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func testGetTaskType(t *testing.T, kv *api.KV) {
	type args struct {
		kv     *api.KV
		taskID string
	}
	tests := []struct {
		name    string
		args    args
		want    TaskType
		wantErr bool
	}{
		{"TypeDeploy", args{kv, "t1"}, Deploy, false},
		{"TypeUnDeploy", args{kv, "t2"}, UnDeploy, false},
		{"TypeScaleUp", args{kv, "t3"}, ScaleUp, false},
		{"TypeScaleDown", args{kv, "t4"}, ScaleDown, false},
		{"TypePurge", args{kv, "t5"}, Purge, false},
		{"TypeCustomCommand", args{kv, "t6"}, CustomCommand, false},
		{"TypeDoesntExist", args{kv, "t7"}, Deploy, true},
		{"TypeNotInt", args{kv, "tNotInt"}, Deploy, true},
		{"TaskDoesntExist", args{kv, "TaskDoesntExist"}, Deploy, true},
		{"TypeCustomWorkflow", args{kv, "tCustomWF"}, CustomWorkflow, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTaskType(tt.args.kv, tt.args.taskID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTaskType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetTaskType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func testGetTaskTarget(t *testing.T, kv *api.KV) {
	type args struct {
		kv     *api.KV
		taskID string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"GetTarget", args{kv, "t1"}, "id1", false},
		{"TaskDoesntExist", args{kv, "TaskDoesntExist"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTaskTarget(tt.args.kv, tt.args.taskID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTaskTarget() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetTaskTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func testTaskExists(t *testing.T, kv *api.KV) {
	type args struct {
		kv     *api.KV
		taskID string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{"TaskExist", args{kv, "t1"}, true, false},
		{"TaskDoesntExist", args{kv, "TaskDoesntExist"}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TaskExists(tt.args.kv, tt.args.taskID)
			if (err != nil) != tt.wantErr {
				t.Errorf("TaskExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("TaskExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func testCancelTask(t *testing.T, kv *api.KV) {
	type args struct {
		kv     *api.KV
		taskID string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"CancelTask", args{kv, "t1"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CancelTask(tt.args.kv, tt.args.taskID); (err != nil) != tt.wantErr {
				t.Errorf("CancelTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			kvp, _, err := kv.Get(path.Join(consulutil.TasksPrefix, tt.args.taskID, ".canceledFlag"), nil)
			if err != nil {
				t.Errorf("Unexpected Consul communication error: %v", err)
				return
			}
			if kvp == nil {
				t.Error("canceledFlag missing")
				return
			}
			if string(kvp.Value) != "true" {
				t.Error("canceledFlag not set to \"true\"")
				return
			}
		})
	}
}

func testTargetHasLivingTasks(t *testing.T, kv *api.KV) {
	type args struct {
		kv       *api.KV
		targetID string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		want1   string
		want2   string
		wantErr bool
	}{
		{"TargetHasRunningTasks", args{kv, "id1"}, true, "t1", "INITIAL", false},
		{"TargetHasNoRunningTasks", args{kv, "id2"}, false, "", "", false},
		{"TargetDoesntExist", args{kv, "TargetDoesntExist"}, false, "", "", false},
		{"TargetNotInt", args{kv, "targetNotInt"}, false, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, err := TargetHasLivingTasks(tt.args.kv, tt.args.targetID)
			if (err != nil) != tt.wantErr {
				t.Errorf("TargetHasLivingTasks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("TargetHasLivingTasks() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("TargetHasLivingTasks() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("TargetHasLivingTasks() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func testGetTaskInput(t *testing.T, kv *api.KV) {
	type args struct {
		kv        *api.KV
		taskID    string
		inputName string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"InputExist", args{kv, "t1", "i0"}, "0", false},
		{"InputDoesnt", args{kv, "t1", "i1"}, "", true},
		{"InputsDoesnt", args{kv, "t2", "i1"}, "", true},
		{"TaskDoesntExist", args{kv, "TargetDoesntExist", "i1"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTaskInput(tt.args.kv, tt.args.taskID, tt.args.inputName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTaskInput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetTaskInput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func testGetInstances(t *testing.T, kv *api.KV) {
	type args struct {
		kv           *api.KV
		taskID       string
		deploymentID string
		nodeName     string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{"TaskRelatedNodes", args{kv, "t1", "id1", "node1"}, []string{"0", "1", "2"}, false},
		{"TaskRelatedNodes", args{kv, "t1", "id1", "node2"}, []string{"0", "1"}, false},
		{"TaskRelatedNodes", args{kv, "t2", "id1", "node2"}, []string{"0", "1"}, false},
		{"TaskDoesntExistDeploymentDoes", args{kv, "TaskDoesntExist", "id1", "node2"}, []string{"0", "1"}, false},
		{"TaskDoesntExistDeploymentDoesInstanceDont", args{kv, "TaskDoesntExist", "id1", "node3"}, []string{}, false},
		{"TaskDoesntExistDeploymentToo", args{kv, "TaskDoesntExist", "idDoesntExist", "node2"}, []string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetInstances(tt.args.kv, tt.args.taskID, tt.args.deploymentID, tt.args.nodeName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetInstances() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetInstances() = %v, want %v", got, tt.want)
			}
		})
	}
}

func testGetTaskRelatedNodes(t *testing.T, kv *api.KV) {
	type args struct {
		kv     *api.KV
		taskID string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{"TaskRelNodes", args{kv, "t1"}, []string{"node1"}, false},
		{"NoTaskRelNodes", args{kv, "t2"}, nil, false},
		{"NoTaskRelNodes", args{kv, "t3"}, []string{"n1", "n2", "n3"}, false},
		{"TaskDoesntExist", args{kv, "TaskDoesntExist"}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTaskRelatedNodes(tt.args.kv, tt.args.taskID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTaskRelatedNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetTaskRelatedNodes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func testIsTaskRelatedNode(t *testing.T, kv *api.KV) {
	type args struct {
		kv       *api.KV
		taskID   string
		nodeName string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{"TaskRelNode", args{kv, "t1", "node1"}, true, false},
		{"NotTaskRelNode", args{kv, "t1", "node2"}, false, false},
		{"NotTaskRelNode2", args{kv, "t2", "node1"}, false, false},
		{"NotTaskRelNode3", args{kv, "t2", "node2"}, false, false},
		{"TaskDoesntExist", args{kv, "TaskDoesntExist", "node2"}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsTaskRelatedNode(tt.args.kv, tt.args.taskID, tt.args.nodeName)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsTaskRelatedNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsTaskRelatedNode() = %v, want %v", got, tt.want)
			}
		})
	}
}