////////////////////////////////////////////////////////////////////////////////
// Copyright © 2019 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////
package cmd

import (
	"fmt"
	jww "github.com/spf13/jwalterweatherman"
	nodeComms "gitlab.com/elixxir/comms/node"
	"gitlab.com/elixxir/primitives/utils"
	"gitlab.com/elixxir/registration/storage"
	"gitlab.com/elixxir/registration/storage/node"
	"gitlab.com/elixxir/registration/testkeys"
	"os"
	"testing"
	"time"
)

var nodeAddr = "0.0.0.0:6900"
var nodeCert []byte
var nodeKey []byte
var permAddr = "0.0.0.0:5900"
var testParams Params
var gatewayCert []byte

var nodeComm *nodeComms.Comms

func TestMain(m *testing.M) {
	jww.SetStdoutThreshold(jww.LevelDebug)
	var err error
	nodeCert, err = utils.ReadFile(testkeys.GetNodeCertPath())
	if err != nil {
		fmt.Printf("Could not get node cert: %+v\n", err)
	}

	nodeKey, err = utils.ReadFile(testkeys.GetNodeKeyPath())
	if err != nil {
		fmt.Printf("Could not get node key: %+v\n", err)
	}

	gatewayCert, err = utils.ReadFile(testkeys.GetCACertPath())
	if err != nil {
		fmt.Printf("Could not get gateway cert: %+v\n", err)
	}

	testParams = Params{
		Address:                   permAddr,
		CertPath:                  testkeys.GetCACertPath(),
		KeyPath:                   testkeys.GetCAKeyPath(),
		NdfOutputPath:             testkeys.GetNDFPath(),
		publicAddress:             permAddr,
		maxRegistrationAttempts:   5,
		registrationCountDuration: time.Hour,
	}
	nodeComm = nodeComms.StartNode("tmp", nodeAddr, nodeComms.NewImplementation(), nodeCert, nodeKey)

	runFunc := func() int {
		code := m.Run()
		nodeComm.Shutdown()
		return code
	}

	os.Exit(runFunc())
}

//Error path: Test an insertion on an empty database
func TestEmptyDataBase(t *testing.T) {
	//Start the registration server
	testParams := Params{
		CertPath:                  testkeys.GetCACertPath(),
		KeyPath:                   testkeys.GetCAKeyPath(),
		maxRegistrationAttempts:   5,
		registrationCountDuration: time.Hour,
	}
	// Start registration server
	impl, err := StartRegistration(testParams)
	if err != nil {
		t.Errorf(err.Error())
	}

	storage.PermissioningDb, err = storage.NewDatabase("test", "password",
		"regCodes", "0.0.0.0:6969")
	if err != nil {
		t.Errorf("%+v", err)
	}

	//using node cert as gateway cert
	err = impl.RegisterNode([]byte("test"), nodeAddr, string(nodeCert),
		nodeAddr, string(nodeCert), "AAA")
	if err == nil {
		expectedErr := "Unable to insert node: unable to register node AAA"
		t.Errorf("Database was empty but allowed a reg code to go through. "+
			"Expected %s, Recieved: %+v", expectedErr, err)
		return
	}
	impl.Comms.Shutdown()

}

// Happy path: looking for a code that is in the database
func TestRegCodeExists_InsertRegCode(t *testing.T) {
	// Start registration server
	impl, err := StartRegistration(testParams)
	if err != nil {
		t.Errorf(err.Error())
	}
	impl.nodeCompleted = make(chan string, 1)
	storage.PermissioningDb, err = storage.NewDatabase("test", "password",
		"regCodes", "0.0.0.0:6969")
	if err != nil {
		t.Errorf("%+v", err)
	}
	// Load in a registration code
	applicationId := uint64(10)
	newNode := storage.Node{
		Code:          "TEST",
		Order:         "BLARG",
		ApplicationId: applicationId,
	}
	newApplication := storage.Application{Id: applicationId}
	err = storage.PermissioningDb.InsertApplication(newApplication, newNode)
	if err != nil {
		t.Errorf("Failed to insert client reg code %+v", err)
	}
	//Register a node with that regCode
	err = impl.RegisterNode([]byte("test"), nodeAddr, string(nodeCert),
		nodeAddr, string(nodeCert), newNode.Code)
	if err != nil {
		t.Errorf("Registered a node with a known reg code, but recieved the following error: %+v", err)
	}

	//Kill the connections for the next test
	impl.Comms.Shutdown()
}

//Happy Path:  Insert a reg code along with a node
func TestRegCodeExists_RegUser(t *testing.T) {
	//Initialize an implementation and the permissioning server
	impl, err := StartRegistration(testParams)
	if err != nil {
		t.Errorf("Unable to start: %+v", err)
	}

	// Initialize the database
	storage.PermissioningDb, err = storage.NewDatabase("test", "password",
		"regCodes", "0.0.0.0:6969")
	if err != nil {
		t.Errorf("%+v", err)
	}

	//Insert regcodes into it
	err = storage.PermissioningDb.InsertClientRegCode("AAAA", 100)
	if err != nil {
		t.Errorf("Failed to insert client reg code %+v", err)
	}

	//Attempt to register a user
	sig, err := impl.RegisterUser("AAAA", string(nodeKey))

	if err != nil {
		t.Errorf("Failed to register a node when it should have worked: %+v", err)
	}

	if sig == nil {
		t.Errorf("Failed to sign public key, recieved %+v as a signature", sig)
	}
	impl.Comms.Shutdown()
}

//Attempt to register a node after the
func TestCompleteRegistration_HappyPath(t *testing.T) {
	// Initialize the database
	var err error
	storage.PermissioningDb, err = storage.NewDatabase("test", "password",
		"regCodes", "0.0.0.0:6969")
	if err != nil {
		t.Errorf("%+v", err)
	} //Insert a sample regCode
	infos := make([]node.Info, 0)
	infos = append(infos, node.Info{RegCode: "BBBB"})

	storage.PopulateNodeRegistrationCodes(infos)
	localParams := testParams
	localParams.minimumNodes = 1
	// Start registration server
	impl, err := StartRegistration(localParams)
	if err != nil {
		t.Errorf(err.Error())
	}
	RegParams = testParams

	err = impl.RegisterNode([]byte("test"), "0.0.0.0:6900", string(nodeCert),
		"0.0.0.0:6900", string(nodeCert), "BBBB")
	if err != nil {
		t.Errorf("Expected happy path, recieved error: %+v", err)
		return
	}

	beginScheduling := make(chan struct{}, 1)

	go func() {
		err = impl.nodeRegistrationCompleter(beginScheduling)
		if err != nil {
			t.Errorf("Expected happy path, recieved error: %+v", err)
		}
	}()

	select {
	case <-time.NewTimer(50 * time.Millisecond).C:
		t.Errorf("Registration failed to complete")
		t.FailNow()
	case <-beginScheduling:
	}

	//Kill the connections for the next test
	impl.Comms.Shutdown()
}

//Error path: test that trying to register with the same reg code fails
func TestDoubleRegistration(t *testing.T) {
	// Initialize the database
	var err error
	storage.PermissioningDb, err = storage.NewDatabase("test", "password",
		"regCodes", "0.0.0.0:6969")
	if err != nil {
		t.Errorf("%+v", err)
	}
	//Create reg codes and populate the database
	infos := make([]node.Info, 0)
	infos = append(infos, node.Info{RegCode: "AAAA"}, node.Info{RegCode: "BBBB"}, node.Info{RegCode: "CCCC"})
	storage.PopulateNodeRegistrationCodes(infos)
	RegParams = testParams

	// Start registration server
	impl, err := StartRegistration(testParams)
	if err != nil {
		t.Errorf(err.Error())
	}
	beginScheduling := make(chan<- struct{}, 1)
	go impl.nodeRegistrationCompleter(beginScheduling)

	//Create a second node to register
	nodeComm2 := nodeComms.StartNode("tmp", "0.0.0.0:6901", nodeComms.NewImplementation(), nodeCert, nodeKey)

	//Register 1st node
	err = impl.RegisterNode([]byte("test"), nodeAddr, string(nodeCert),
		nodeAddr, string(nodeCert), "BBBB")
	if err != nil {
		t.Errorf("Expected happy path, recieved error: %+v", err)
	}

	//Register 2nd node
	err = impl.RegisterNode([]byte("B"), "0.0.0.0:6901", string(nodeCert),
		"0.0.0.0:6901", string(nodeCert), "BBBB")
	//Kill the connections for the next test
	nodeComm2.Shutdown()
	impl.Comms.Shutdown()
	if err != nil {
		return
	}

	t.Errorf("Expected happy path, recieved error: %+v", err)
}

//Happy path: attempt to register 2 nodes
func TestTopology_MultiNodes(t *testing.T) {
	// Initialize the database
	var err error
	storage.PermissioningDb, err = storage.NewDatabase("test", "password",
		"regCodes", "0.0.0.0:6969")
	if err != nil {
		t.Errorf("%+v", err)
	}
	//Create reg codes and populate the database
	infos := make([]node.Info, 0)
	infos = append(infos, node.Info{RegCode: "AAAA"}, node.Info{RegCode: "BBBB"}, node.Info{RegCode: "CCCC"})
	storage.PopulateNodeRegistrationCodes(infos)

	localParams := testParams
	localParams.minimumNodes = 2

	// Start registration server
	impl, err := StartRegistration(localParams)
	if err != nil {
		t.Errorf(err.Error())
	}

	//Create a second node to register
	nodeComm2 := nodeComms.StartNode("tmp", "0.0.0.0:6901", nodeComms.NewImplementation(), nodeCert, nodeKey)

	//Register 1st node
	err = impl.RegisterNode([]byte("A"), nodeAddr, string(nodeCert),
		nodeAddr, string(nodeCert), "BBBB")
	if err != nil {
		t.Errorf("Expected happy path, recieved error: %+v", err)
	}

	//Register 2nd node
	err = impl.RegisterNode([]byte("B"), "0.0.0.0:6901", string(gatewayCert),
		"0.0.0.0:6901", string(gatewayCert), "CCCC")
	if err != nil {
		t.Errorf("Expected happy path, recieved error: %+v", err)
	}
	beginScheduling := make(chan struct{}, 1)

	go func() {
		err = impl.nodeRegistrationCompleter(beginScheduling)
		if err != nil {
			t.Errorf(err.Error())
		}
	}()

	select {
	case <-time.NewTimer(250 * time.Millisecond).C:
		t.Errorf("Registration failed to complete")
	case <-beginScheduling:
	}

	//Kill the connections for the next test
	nodeComm2.Shutdown()
	impl.Comms.Shutdown()
}

func TestRegistrationImpl_GetCurrentClientVersion(t *testing.T) {
	impl, err := StartRegistration(testParams)
	if err != nil {
		t.Errorf(err.Error())
	}
	testVersion := "0.0.0a"
	setClientVersion(testVersion)
	version, err := impl.GetCurrentClientVersion()
	if err != nil {
		t.Error(err)
	}
	if version != testVersion {
		t.Errorf("Version was %+v, expected %+v", version, testVersion)
	}
}

// Test a case that should pass validation
func TestValidateClientVersion_Success(t *testing.T) {
	err := validateVersion("0.0.0a")
	if err != nil {
		t.Errorf("Unexpected error from validateVersion: %+v", err.Error())
	}
}

// Test some cases that shouldn't pass validation
func TestValidateClientVersion_Failure(t *testing.T) {
	err := validateVersion("")
	if err == nil {
		t.Error("Expected error for empty version string")
	}
	err = validateVersion("0")
	if err == nil {
		t.Error("Expected error for version string with one number")
	}
	err = validateVersion("0.0")
	if err == nil {
		t.Error("Expected error for version string with two numbers")
	}
	err = validateVersion("a.4.0")
	if err == nil {
		t.Error("Expected error for version string with non-numeric major version")
	}
	err = validateVersion("4.a.0")
	if err == nil {
		t.Error("Expected error for version string with non-numeric minor version")
	}
}

// Happy Path: Inserts users until the max is reached, waits until the timer has
// cleared the number of allowed registrations and inserts another user.
func TestRegCodeExists_RegUser_Timer(t *testing.T) {

	testParams2 := Params{
		Address:                   "0.0.0.0:5905",
		CertPath:                  testkeys.GetCACertPath(),
		KeyPath:                   testkeys.GetCAKeyPath(),
		NdfOutputPath:             testkeys.GetNDFPath(),
		publicAddress:             "0.0.0.0:5905",
		maxRegistrationAttempts:   4,
		registrationCountDuration: 3 * time.Second,
	}

	// Start registration server
	impl, err := StartRegistration(testParams2)
	if err != nil {
		t.Errorf(err.Error())
	}
	beginScheduling := make(chan<- struct{}, 1)
	go impl.nodeRegistrationCompleter(beginScheduling)

	// Initialize the database
	storage.PermissioningDb, err = storage.NewDatabase("test", "password",
		"regCodes", "0.0.0.0:6969")
	if err != nil {
		t.Errorf("%+v", err)
	}

	// Attempt to register a user
	_, err = impl.RegisterUser("b", "B")
	if err != nil {
		t.Errorf("Failed to register a user when it should have worked: %+v", err)
	}

	// Attempt to register a user
	_, err = impl.RegisterUser("c", "C")
	if err != nil {
		t.Errorf("Failed to register a user when it should have worked: %+v", err)
	}

	// Attempt to register a user
	_, err = impl.RegisterUser("d", "D")
	if err != nil {
		t.Errorf("Failed to register a user when it should have worked: %+v", err)
	}

	// Attempt to register a user
	_, err = impl.RegisterUser("e", "E")
	if err != nil {
		t.Errorf("Failed to register a user when it should have worked: %+v", err)
	}

	// Attempt to register a user
	_, err = impl.RegisterUser("f", "F")
	if err == nil {
		t.Errorf("Did not fail to register a user when it should not have worked: %+v", err)
	}

	time.Sleep(testParams2.registrationCountDuration)
	// Attempt to register a user
	_, err = impl.RegisterUser("g", "G")
	if err != nil {
		t.Errorf("Failed to register a user when it should have worked: %+v", err)
	}
}
