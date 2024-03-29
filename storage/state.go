////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Handles network state tracking and control

package storage

import (
	"crypto/rand"
	"encoding/base64"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network/dataStructures"
	"gitlab.com/elixxir/primitives/states"
	"gitlab.com/elixxir/registration/storage/node"
	"gitlab.com/elixxir/registration/storage/round"
	"gitlab.com/xx_network/comms/signature"
	"gitlab.com/xx_network/crypto/signature/ec"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/ndf"
	"gitlab.com/xx_network/primitives/region"
	"gitlab.com/xx_network/primitives/utils"
	"google.golang.org/protobuf/proto"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const updateBufferLength = 10000

// NetworkState structure used for keeping track of NDF and Round state.
type NetworkState struct {
	// NetworkState parameters
	rsaPrivateKey      *rsa.PrivateKey
	ellipticPrivateKey *ec.PrivateKey

	// Round state
	rounds       *round.StateMap
	roundUpdates *dataStructures.Updates
	roundData    *dataStructures.Data
	update       chan node.UpdateNotification // For triggering updates to top level

	// Node NetworkState
	nodes     *node.StateMap
	updateMux sync.Mutex

	// List of states of Nodes to be disabled
	disabledNodesStates *disabledNodes

	// Keep track of Country -> Bin mapping
	geoBins map[string]region.GeoBin

	// NDF state
	InternalNdfLock sync.RWMutex
	unprunedNdf     *ndf.NetworkDefinition
	pruneListMux    sync.RWMutex
	// Boolean determines whether Node is omitted from NDF
	pruneList map[id.ID]bool

	outputNdfLock sync.RWMutex
	partialNdf    *dataStructures.Ndf
	fullNdf       *dataStructures.Ndf

	// Address space size
	addressSpaceSize *uint32

	// Output path to the full ndf
	fullNdfOutputPath string

	// Output path to the signed partial ndf provided to client
	// by uploading the file
	signedPartialNdfOutputPath string

	// round adder buffer channel
	roundUpdatesToAddCh chan *dataStructures.Round

	// round states
	roundID  id.Round
	updateID uint64
}

// NewState returns a new NetworkState object.
func NewState(rsaPrivKey *rsa.PrivateKey, addressSpaceSize uint32,
	fullNdfOutputPath string, signedPartialNdfOutputPath string,
	geoBins map[string]region.GeoBin) (*NetworkState, error) {

	fullNdf, err := dataStructures.NewNdf(&ndf.NetworkDefinition{})
	if err != nil {
		return nil, err
	}
	partialNdf, err := dataStructures.NewNdf(&ndf.NetworkDefinition{})
	if err != nil {
		return nil, err
	}

	state := &NetworkState{
		rounds:                     round.NewStateMap(),
		roundUpdates:               dataStructures.NewUpdates(),
		update:                     make(chan node.UpdateNotification, updateBufferLength),
		nodes:                      node.NewStateMap(),
		fullNdf:                    fullNdf,
		partialNdf:                 partialNdf,
		rsaPrivateKey:              rsaPrivKey,
		addressSpaceSize:           &addressSpaceSize,
		unprunedNdf:                &ndf.NetworkDefinition{},
		pruneList:                  make(map[id.ID]bool),
		fullNdfOutputPath:          fullNdfOutputPath,
		signedPartialNdfOutputPath: signedPartialNdfOutputPath,
		roundUpdatesToAddCh:        make(chan *dataStructures.Round, 500),
		geoBins:                    geoBins,
	}

	//begin the thread that reads and adds round updates
	go state.RoundAdderRoutine()

	// Obtain round & update Id from Storage
	// Ignore not found in Storage errors, zero-value will be handled below
	state.updateID, err = state.GetUpdateID()
	if err != nil &&
		!strings.Contains(err.Error(), gorm.ErrRecordNotFound.Error()) {
		return nil, err
	}
	state.roundID, err = state.GetRoundID()
	if err != nil &&
		!strings.Contains(err.Error(), gorm.ErrRecordNotFound.Error()) {
		return nil, err
	}

	ellipticKey, err := state.getEcKey()
	if err != nil &&
		!strings.Contains(err.Error(), gorm.ErrRecordNotFound.Error()) {
		return nil, err
	}

	// Handle elliptic key storage, either creating a key if one
	// does not already exist or loading it into the object if it does
	if ellipticKey == "" {
		// Create a key if one doesn't exist
		ecPrivKey, err := ec.NewKeyPair(rand.Reader)
		if err != nil {
			return nil, err
		}
		err = state.storeEcKey(ecPrivKey.MarshalText())
		if err != nil {
			return nil, err
		}

		state.ellipticPrivateKey = ecPrivKey

	} else {
		state.ellipticPrivateKey, err = ec.LoadPrivateKey(ellipticKey)
		if err != nil {
			return nil, err
		}
	}

	// Updates are handled in the uint space, as a result, the designator for
	// update 0 also designates that no updates are known by the server. To
	// avoid this collision, permissioning will skip this update as well.
	if state.updateID == 0 {
		// Set update Id to start at 0
		err = state.setId(UpdateIdKey, 0)
		if err != nil {
			return nil, err
		}
		// Then insert a dummy and increment to 1
		err = state.AddRoundUpdate(&pb.RoundInfo{Timestamps: make([]uint64, states.FAILED)})
		if err != nil {
			return nil, err
		}
		// Wait for the above state to update (it is multithreaded)
		for state.roundUpdates.GetLastUpdateID() != 0 {
		}
	}
	if state.roundID == 0 {
		state.roundID = 1
		// Set round Id to start at 1
		err = state.setId(RoundIdKey, 1)
		if err != nil {
			return nil, err
		}
	}

	return state, nil
}

// CountActiveNodes returns a count of active nodes in the state
func (s *NetworkState) CountActiveNodes() int {
	s.pruneListMux.Lock()
	defer s.pruneListMux.Unlock()

	s.InternalNdfLock.RLock()
	defer s.InternalNdfLock.RUnlock()

	unpruned := s.GetUnprunedNdf()

	return len(unpruned.Nodes) - len(s.pruneList)
}

// Adds pruned nodes, used by disabledNodes
func (s *NetworkState) setPrunedNodesNoReset(ids []*id.ID) {
	s.pruneListMux.Lock()
	defer s.pruneListMux.Unlock()

	for _, i := range ids {
		// Disabled nodes will remain in NDF
		s.pruneList[*i] = false
	}
}

// Sets pruned Nodes, including disabled Nodes
// Used by node metrics tracker
func (s *NetworkState) SetPrunedNodes(prunedNodes map[id.ID]bool) {
	s.pruneListMux.Lock()
	defer s.pruneListMux.Unlock()

	s.pruneList = prunedNodes

	if s.disabledNodesStates != nil {
		disabled := s.disabledNodesStates.getDisabledNodes()
		for _, i := range disabled {
			// Disabled nodes will remain in NDF
			s.pruneList[*i] = false
		}
	}
}

// Sets a Node as pruned (to be removed from NDF)
// Used on startup
func (s *NetworkState) SetPrunedNode(id *id.ID) {
	s.pruneListMux.Lock()
	defer s.pruneListMux.Unlock()

	s.pruneList[*id] = true
}

func (s *NetworkState) IsPruned(node *id.ID) bool {
	s.pruneListMux.RLock()
	defer s.pruneListMux.RUnlock()

	_, exists := s.pruneList[*node]
	return exists
}

func (s *NetworkState) GetUnprunedNdf() *ndf.NetworkDefinition {
	return s.unprunedNdf
}

// GetFullNdf returns the full NDF.
func (s *NetworkState) GetFullNdf() *dataStructures.Ndf {
	s.outputNdfLock.RLock()
	defer s.outputNdfLock.RUnlock()
	return s.fullNdf
}

// GetPartialNdf returns the partial NDF.
func (s *NetworkState) GetPartialNdf() *dataStructures.Ndf {
	s.outputNdfLock.RLock()
	defer s.outputNdfLock.RUnlock()
	return s.partialNdf
}

// GetGeoBin returns the GeoBin map.
func (s *NetworkState) GetGeoBins() map[string]region.GeoBin {
	return s.geoBins
}

// GetUpdates returns all of the updates after the given ID.
func (s *NetworkState) GetUpdates(id int) ([]*pb.RoundInfo, error) {
	return s.roundUpdates.GetUpdates(id), nil
}

// AddRoundUpdate creates a copy of the round before inserting it into
// roundUpdates.
func (s *NetworkState) AddRoundUpdate(r *pb.RoundInfo) error {
	s.updateMux.Lock()
	defer s.updateMux.Unlock()

	roundCopy := round.CopyRoundInfo(r)
	updateID, err := s.IncrementUpdateID()
	if err != nil {
		return err
	}

	roundCopy.UpdateID = updateID

	go func() {
		err = signature.SignRsa(roundCopy, s.rsaPrivateKey)
		if err != nil {
			jww.FATAL.Panicf("Could not add round update %v "+
				"for round %v due to failed signature: %+v",
				roundCopy.UpdateID, roundCopy.ID, err)
		}

		err = signature.SignEddsa(roundCopy, s.GetEllipticPrivateKey())
		if err != nil {
			jww.FATAL.Panicf("Could not add round update %v "+
				"for round %v due to failed elliptic curve "+
				"signature: %+v", roundCopy.UpdateID,
				roundCopy.ID, err)
		}

		jww.TRACE.Printf("Round Info: %+v", roundCopy)

		jww.INFO.Printf("Round %v state updated to %s", r.ID,
			states.Round(roundCopy.State))

		rnd := dataStructures.NewVerifiedRound(roundCopy,
			s.rsaPrivateKey.GetPublic())
		s.roundUpdatesToAddCh <- rnd
	}()
	return nil
}

// RoundAdderRoutine monitors a channel and keeps track of pending round updates,
// adding them in order
func (s *NetworkState) RoundAdderRoutine() {
	futureRoundUpdates := make(map[uint64]*dataStructures.Round)
	nextID := uint64(0)
	for {
		// Add the next round update from the channel
		rnd := <-s.roundUpdatesToAddCh
		rndUpdateId := rnd.Get().UpdateID

		// Print the size of the future updates map so that potential memory leaks
		// as a result of the structure of this function can be noticed.
		if nextID%100 == 0 {
			jww.DEBUG.Printf("RoundAdderRoutine has %d future updates queued",
				len(futureRoundUpdates))
		}

		// If update is not current, process it immediately
		if rndUpdateId < nextID {
			err := s.roundUpdates.AddRound(rnd)
			if err != nil {
				jww.FATAL.Panicf("%+v", err)
			}
			continue
		}

		// if the next ID has not been set, then set it to the new ID
		if nextID == 0 {
			nextID = rndUpdateId
		}

		// Update comes from the future, add it for future processing
		futureRoundUpdates[rndUpdateId] = rnd

		// Sequentially process updates added earlier until a gap is reached
		for r, ok := futureRoundUpdates[nextID]; ok; r, ok = futureRoundUpdates[nextID] {
			err := s.roundUpdates.AddRound(r)
			if err != nil {
				jww.FATAL.Panicf("%+v", err)
			}
			// Clean up processed round
			delete(futureRoundUpdates, nextID)
			nextID++
		}
	}
}

// UpdateInternalNdf updates the unpruned internal NDF to the passed in NDF.
// This will be used for the output NDF next time it is updated.  Note that
// callers of this function should take s.InternalNdfLock as appropriate.
func (s *NetworkState) UpdateInternalNdf(newNdf *ndf.NetworkDefinition) {
	newNdf.Timestamp = time.Now()
	s.unprunedNdf = newNdf.DeepCopy()
}

// UpdateOutputNdf takes the current unprunedNdf and signs and outputs
// it to the full & partial ndf fields, along with writing it to disk.
func (s *NetworkState) UpdateOutputNdf() (err error) {
	s.outputNdfLock.Lock()
	defer s.outputNdfLock.Unlock()

	s.InternalNdfLock.RLock()
	loadedNdf := s.unprunedNdf.DeepCopy()
	s.InternalNdfLock.RUnlock()
	// Sanity checks on loaded ndf data
	if loadedNdf == nil {
		jww.WARN.Printf("No unpruned NDF stored to output, skipping update")
		return nil
	} else if s.fullNdf != nil && s.fullNdf.Get() != nil &&
		!loadedNdf.Timestamp.After(s.fullNdf.Get().Timestamp) {
		jww.WARN.Printf("Skipping update: Loaded unpruned NDF timestamp"+
			" %s is not later than current output NDF timestamp %s",
			loadedNdf.Timestamp.String(), s.fullNdf.Get().Timestamp.String())
		return nil
	}

	newNdf := loadedNdf.DeepCopy()

	s.pruneListMux.RLock()
	//prune the NDF
	for i := 0; i < len(newNdf.Nodes); i++ {
		nid, _ := id.Unmarshal(newNdf.Nodes[i].ID)

		// Prune nodes if in the prune list
		if isPruned, exists := s.pruneList[*nid]; exists {
			if isPruned {
				newNdf.Nodes = append(newNdf.Nodes[:i], newNdf.Nodes[i+1:]...)
				newNdf.Gateways = append(newNdf.Gateways[:i], newNdf.Gateways[i+1:]...)
				i--
			} else {
				newNdf.Nodes[i].Status = ndf.Stale
			}
		} else {
			newNdf.Nodes[i].Status = ndf.Active
		}

	}
	s.pruneListMux.RUnlock()

	// Build NDF comms messages
	fullNdfMsg := &pb.NDF{}
	fullNdfMsg.Ndf, err = newNdf.Marshal()
	if err != nil {
		return
	}
	partialNdfMsg := &pb.NDF{}
	partialNdfMsg.Ndf, err = newNdf.StripNdf().Marshal()
	if err != nil {
		return
	}

	// Sign NDF comms messages
	err = signature.SignRsa(fullNdfMsg, s.rsaPrivateKey)
	if err != nil {
		return
	}
	err = signature.SignRsa(partialNdfMsg, s.rsaPrivateKey)
	if err != nil {
		return
	}

	// Assign NDF comms messages
	err = s.fullNdf.Update(fullNdfMsg)
	if err != nil {
		return err
	}

	err = s.partialNdf.Update(partialNdfMsg)
	if err != nil {
		return err
	}

	// Output full NDF to file
	err = outputToJSON(newNdf, s.fullNdfOutputPath)
	if err != nil {
		jww.ERROR.Printf("unable to output full NDF JSON file: %+v", err)
	}

	// Marshal signed partial NDF
	signedPartialNdfMarshal, err := proto.Marshal(s.partialNdf.GetPb())
	if err != nil {
		jww.ERROR.Printf("unable to marshal partial ndf")
	}

	// Base64 encode the signed marshaled NDF
	signedPartialEncoded := base64.StdEncoding.EncodeToString(signedPartialNdfMarshal)

	// Output signed partial ndf to file
	err = utils.WriteFile(s.signedPartialNdfOutputPath,
		[]byte(signedPartialEncoded), utils.FilePerms, utils.DirPerms)
	if err != nil {
		jww.ERROR.Printf("unable to output signed partial NDF to file: %+v", err)
	}

	jww.INFO.Printf("Full NDF updated to: %s", base64.StdEncoding.EncodeToString(s.fullNdf.GetHash()))

	return nil
}

// GetPrivateKey returns the server's private key.
func (s *NetworkState) GetPrivateKey() *rsa.PrivateKey {
	return s.rsaPrivateKey
}

// Get the elliptic curve private key
func (s *NetworkState) GetEllipticPrivateKey() *ec.PrivateKey {
	return s.ellipticPrivateKey
}

// Get the elliptic curve public key
func (s *NetworkState) GetEllipticPublicKey() *ec.PublicKey {
	return s.ellipticPrivateKey.GetPublic()
}

// GetRoundMap returns the map of rounds.
func (s *NetworkState) GetRoundMap() *round.StateMap {
	return s.rounds
}

// GetNodeMap returns the map of nodes.
func (s *NetworkState) GetNodeMap() *node.StateMap {
	return s.nodes
}

// GetAddressSpaceSize returns the address space size
func (s *NetworkState) GetAddressSpaceSize() uint32 {
	return atomic.LoadUint32(s.addressSpaceSize)
}

// SetAddressSpaceSize sets the address space size.
func (s *NetworkState) SetAddressSpaceSize(size uint32) {
	atomic.StoreUint32(s.addressSpaceSize, size)
}

// NodeUpdateNotification sends a notification to the control thread of an
// update to a nodes state.
func (s *NetworkState) SendUpdateNotification(nun node.UpdateNotification) error {
	select {
	case s.update <- nun:
		return nil
	default:
		return errors.New("Could not send update notification")
	}
}

// GetNodeUpdateChannel returns a channel to receive node updates on.
func (s *NetworkState) GetNodeUpdateChannel() <-chan node.UpdateNotification {
	return s.update
}

// Helper to set the roundId or updateId value
func (s *NetworkState) setId(key string, newVal uint64) error {
	err := PermissioningDb.UpsertState(&State{
		Key:   key,
		Value: strconv.FormatUint(newVal, 10),
	})
	if err != nil {
		return errors.Errorf("Unable to update current round ID: %+v", err)
	}
	return nil
}

// Helper to return the RoundId or UpdateId depending on the given key
func (s *NetworkState) get(key string) (uint64, error) {
	roundIdStr, err := PermissioningDb.GetStateValue(key)
	if err != nil {
		return 0, errors.Errorf("Unable to obtain current %s: %+v", key, err)
	}

	roundId, err := strconv.ParseUint(roundIdStr, 10, 64)
	if err != nil {
		return 0, errors.Errorf("Unable to parse current %s: %+v", key, err)
	}
	return roundId, nil
}

// Helper to return the RoundId or UpdateId depending on the given key
func (s *NetworkState) getEcKey() (string, error) {
	ellipticKey, err := PermissioningDb.GetStateValue(EllipticKey)
	if err != nil {
		return "", errors.Errorf("Unable to obtain current %s: %+v", EllipticKey, err)
	}

	return ellipticKey, nil
}

// Helper to set the elliptic key into the state table
func (s *NetworkState) storeEcKey(newVal string) error {
	err := PermissioningDb.UpsertState(&State{
		Key:   EllipticKey,
		Value: newVal,
	})
	if err != nil {
		return errors.Errorf("Unable to update current round ID: %+v", err)
	}
	return nil
}

// IncrementRoundID increments the round ID
// THIS IS NOT THREAD SAFE. IT IS INTENDED TO ONLY BE CALLED BY THE SERIAL
// SCHEDULING THREAD
func (s *NetworkState) IncrementRoundID() (id.Round, error) {
	oldRoundID := s.roundID
	s.roundID = s.roundID + 1
	return oldRoundID, s.setId(RoundIdKey, uint64(s.roundID))
}

// IncrementUpdateID increments the update ID
// THIS IS NOT THREAD SAFE. IT IS INTENDED TO ONLY BE CALLED BY THE SERIAL
// SCHEDULING THREAD
func (s *NetworkState) IncrementUpdateID() (uint64, error) {
	oldUpdateID := s.updateID
	s.updateID = s.updateID + 1
	return oldUpdateID, s.setId(UpdateIdKey, s.updateID)
}

// GetRoundID returns the round ID
// THIS IS NOT THREAD SAFE. IT IS INTENDED TO ONLY BE CALLED BY THE SERIAL
// SCHEDULING THREAD
func (s *NetworkState) GetRoundID() (id.Round, error) {
	roundId, err := s.get(RoundIdKey)
	return id.Round(roundId), err
}

// GetRoundID returns the update ID
// THIS IS NOT THREAD SAFE. IT IS INTENDED TO ONLY BE CALLED BY THE SERIAL
// SCHEDULING THREAD
func (s *NetworkState) GetUpdateID() (uint64, error) {
	return s.get(UpdateIdKey)
}

// CreateDisabledNodes generates and sets a disabledNodes object that will track
// disabled Nodes list.
func (s *NetworkState) CreateDisabledNodes(path string, interval time.Duration) error {
	var err error
	s.disabledNodesStates, err = generateDisabledNodes(path, interval, s.setPrunedNodesNoReset)
	return err
}

// StartPollDisabledNodes starts the loop that polls for updates
func (s *NetworkState) StartPollDisabledNodes(quitChan chan struct{}) {
	s.disabledNodesStates.pollDisabledNodes(quitChan)
}

// outputNodeTopologyToJSON encodes the NodeTopology structure to JSON and
// outputs it to the specified file path. An error is returned if the JSON
// marshaling fails or if the JSON file cannot be created.
func outputToJSON(ndfData *ndf.NetworkDefinition, filePath string) error {
	// Generate JSON from structure
	data, err := ndfData.Marshal()
	if err != nil {
		return err
	}
	// Write JSON to file
	return utils.WriteFile(filePath, data, utils.FilePerms, utils.DirPerms)
}
