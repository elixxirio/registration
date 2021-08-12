package scheduling

import (
	"encoding/json"
	"gitlab.com/elixxir/comms/testutils"
	"gitlab.com/elixxir/primitives/current"
	"gitlab.com/elixxir/registration/storage"
	"gitlab.com/elixxir/registration/storage/node"
	"gitlab.com/elixxir/registration/testkeys"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/region"
	"gitlab.com/xx_network/primitives/utils"
	"reflect"
	"strconv"
	"testing"
	"time"
)

// Happy path
func TestScheduler_NonRandom(t *testing.T) {
	var err error
	storage.PermissioningDb, _, err = storage.NewDatabase("", "", "", "", "")
	if err != nil {
		t.Errorf("Failed to create new PermissioningDb: %+v", err)
	}

	configJson, err := utils.ReadFile(testkeys.GetSchedulingSimple(false))
	if err != nil {
		t.Errorf("Failed to open %s", testkeys.GetSchedulingSimple(false))
	}

	var testParams Params
	err = json.Unmarshal(configJson, &testParams)
	if err != nil {
		t.Errorf("Could not extract parameters: %v", err)
	}

	// Read in private key
	pk, err := testutils.LoadPrivateKeyTesting(t)
	if err != nil {
		t.Errorf("Failed to parse permissioning server key: %+v. "+
			"PermissioningKey is %+v", err, pk)
	}
	// Start registration server
	state, err := storage.NewState(pk, 8, "", region.GetCountryBins())
	if err != nil {
		t.Errorf("Unable to create state: %+v", err)
	}

	kill := make(chan chan struct{})

	go func() {
		err = Scheduler(configJson, state, kill)
		if err != nil {
			t.Errorf("Scheduler failed with error: %v", err)
		}
	}()

	nodeList := make([]*id.ID, testParams.TeamSize)
	for i := 0; i < int(testParams.TeamSize); i++ {
		nid := id.NewIdFromUInt(uint64(i), id.Node, t)
		nodeList[i] = nid
		nodIDBytes := make([]byte, id.ArrIDLen)
		nodIDBytes[0] = byte(i + 1)
		nodeID := id.NewIdFromBytes(nodIDBytes, t)
		nodeList[i] = nodeID

		err = state.GetNodeMap().AddNode(nodeID, strconv.Itoa(i), "", "", 0)
		if err != nil {
			t.Errorf("Failed to add node %d to map: %v", i, err)
		}
		ns := state.GetNodeMap().GetNode(nodeList[i])
		ns.SetLastPoll(time.Now(), t)
		ns.GetPollingLock().Lock()

		nun := node.UpdateNotification{
			Node:         nodeID,
			FromActivity: current.NOT_STARTED,
			ToActivity:   current.WAITING,
		}

		err = state.SendUpdateNotification(nun)
		if err != nil {
			t.Errorf("Failed to update node %d from %s to %s: %v",
				i, current.NOT_STARTED, current.WAITING, err)
		}
	}

	time.Sleep(1 * time.Second)

	roundInfo, err := state.GetUpdates(0)

	if err != nil {
		t.Errorf("Unexpected error retrieving round info: %v", err)
	}

	if len(roundInfo) == 0 {
		t.Errorf("Expected round to start. " +
			"Received no round info indicating this")
	}

	receivedNodeList, err := id.NewIDListFromBytes(roundInfo[0].Topology)
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
