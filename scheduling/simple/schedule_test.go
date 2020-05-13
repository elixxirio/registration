package simple

import (
	"encoding/json"
	"gitlab.com/elixxir/crypto/signature/rsa"
	"gitlab.com/elixxir/primitives/current"
	"gitlab.com/elixxir/primitives/id"
	"gitlab.com/elixxir/primitives/utils"
	"gitlab.com/elixxir/registration/storage"
	"gitlab.com/elixxir/registration/testkeys"
	"reflect"
	"strconv"
	"testing"
	"time"
)

// Happy path
func TestScheduler(t *testing.T) {
	configJson, err := utils.ReadFile(testkeys.GetSchedulingConfig())
	if err != nil {
		t.Errorf("Failed to open %s", testkeys.GetSchedulingConfig())
	}

	var testParams Params
	err = json.Unmarshal(configJson, &testParams)
	if err != nil {
		t.Errorf("Could not extract parameters: %v", err)
	}

	// Read in private key
	key, err := utils.ReadFile(testkeys.GetCAKeyPath())
	if err != nil {
		t.Errorf("failed to read key at %+v: %+v",
			testkeys.GetCAKeyPath(), err)
	}

	pk, err := rsa.LoadPrivateKeyFromPem(key)
	if err != nil {
		t.Errorf("Failed to parse permissioning server key: %+v. "+
			"PermissioningKey is %+v", err, pk)
	}
	// Start registration server
	state, err := storage.NewState(pk)
	if err != nil {
		t.Errorf("Unable to create state: %+v", err)
	}

	teamSize := 3

	nodeList := make([]*id.Node, teamSize)
	for i := 0; i < teamSize; i++ {
		nid := id.NewNodeFromUInt(uint64(i), t)
		nodeList[i] = nid

		err = state.GetNodeMap().AddNode(nid, strconv.Itoa(i))
		if err != nil {
			t.Errorf("Failed to add node %d to map: %v", i, err)
		}
		state.GetNodeMap().GetNode(nodeList[i]).GetPollingLock().Lock()
		err = state.NodeUpdateNotification(nid, current.NOT_STARTED, current.WAITING)
		if err != nil {
			t.Errorf("Failed to update node %d from %s to %s: %v",
				i, current.NOT_STARTED, current.WAITING, err)
		}

	}

	go func() {
		err = Scheduler(configJson, state)
		if err != nil {
			t.Errorf("Scheduler failed with error: %v", err)
		}
	}()

	time.Sleep(1 * time.Second)

	roundInfo, err := state.GetUpdates(0)

	if len(roundInfo) == 0 {
		t.Errorf("Expected round to start. " +
			"Received no round info indicating this")
	}

	if err != nil {
		t.Errorf("Unexpected error retrieving round info: %v", err)
	}

	receivedNodeList, err := id.NewNodeListFromStrings(roundInfo[0].Topology)
	if err != nil {
		t.Errorf("Failed to convert topology of round info: %v", err)
	}

	if !reflect.DeepEqual(receivedNodeList, nodeList) {
		t.Errorf("Node list received from round info was not expected."+
			"\n\tExpected: %v"+
			"\n\tReceived: %v", nodeList, receivedNodeList)
	}

	if roundInfo[0].BatchSize != testParams.BatchSize {
		t.Errorf("Batchsize in round info is unexpected value."+
			"\n\tExpected: %v"+
			"\n\tReceived: %v", testParams.BatchSize, roundInfo[0].BatchSize)
	}
}