////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles creating polling callbacks for hooking into comms library

package cmd

import (
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/comms/connect"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/current"
	"gitlab.com/elixxir/primitives/ndf"
	"gitlab.com/elixxir/primitives/version"
	"sync/atomic"
)

// Server->Permissioning unified poll function
func (m *RegistrationImpl) Poll(msg *pb.PermissioningPoll,
	auth *connect.Auth) (response *pb.PermissionPollResponse, err error) {

	// Initialize the response
	response = &pb.PermissionPollResponse{}

	// Check for correct version
	err = checkVersion(m.params.minGatewayVersion, m.params.minServerVersion,
		msg)
	if err != nil {
		return nil, err
	}

	// Ensure the NDF is ready to be returned
	regComplete := atomic.LoadUint32(m.NdfReady)
	if regComplete != 1 {
		return response, errors.New(ndf.NO_NDF)
	}

	// Ensure client is properly authenticated
	if !auth.IsAuthenticated || auth.Sender.IsDynamicHost() {
		return response, connect.AuthError(auth.Sender.GetId())
	}

	// Return updated NDF if provided hash does not match current NDF hash
	if isSame := m.State.GetFullNdf().CompareHash(msg.Full.Hash); !isSame {
		jww.DEBUG.Printf("Returning a new NDF to a back-end server!")

		// Return the updated NDFs
		response.FullNDF = m.State.GetFullNdf().GetPb()
		response.PartialNDF = m.State.GetPartialNdf().GetPb()
	}

	// Fetch latest round updates
	response.Updates, err = m.State.GetUpdates(int(msg.LastUpdate))
	if err != nil {
		return
	}

	// Commit updates reported by the node if node involved in the current round
	jww.DEBUG.Printf("Updating state for node %s: %+v",
		auth.Sender.GetId(), msg)

	//get the nodeState and update
	nid := auth.Sender.GetId()

	n := m.State.GetNodeMap().GetNode(nid)
	if n == nil {
		err = errors.Errorf("node %s could not be found in internal state tracker", nid)
		return
	}

	// when a node poll is received, the nodes polling lock is taken here. If
	// there is no update, it is released in this endpoint, otherwise it is
	// released in the scheduling algorithm which blocks all future polls until
	// processing completes
	n.GetPollingLock().Lock()

	// update does edge checking. It ensures the state change recieved was a
	// valid one and the state fo the node and
	// any associated round allows for that change. If the change was not
	// acceptable, it is not recorded and an error is returned, which is
	// propagated to the node
	update, oldActivity, err := n.Update(current.Activity(msg.Activity))

	//if an update ocured, report it to the control thread
	if update {
		err = m.State.NodeUpdateNotification(nid, oldActivity, current.Activity(msg.Activity))
	} else {
		n.GetPollingLock().Unlock()
	}

	return
}

// PollNdf handles the client polling for an updated NDF
func (m *RegistrationImpl) PollNdf(theirNdfHash []byte, auth *connect.Auth) ([]byte, error) {

	// Ensure the NDF is ready to be returned
	regComplete := atomic.LoadUint32(m.NdfReady)
	if regComplete != 1 {
		return nil, errors.New(ndf.NO_NDF)
	}

	// Handle client request
	if !auth.IsAuthenticated || auth.Sender.IsDynamicHost() {
		// Do not return NDF if client hash matches
		if isSame := m.State.GetPartialNdf().CompareHash(theirNdfHash); isSame {
			return nil, nil
		}

		// Send the json of the client
		jww.DEBUG.Printf("Returning a new NDF to client!")
		return m.State.GetPartialNdf().Get().Marshal()
	}

	// Do not return NDF if backend hash matches
	if isSame := m.State.GetFullNdf().CompareHash(theirNdfHash); isSame {
		return nil, nil
	}

	//Send the json of the ndf
	jww.DEBUG.Printf("Returning a new NDF to a back-end server!")
	return m.State.GetFullNdf().Get().Marshal()
}

// checkVersion checks if the PermissioningPoll message server and gateway
// versions are compatible with the required version.
func checkVersion(requiredGateway, requiredServer version.Version,
	msg *pb.PermissioningPoll) error {

	// Parse the gateway version string
	gatewayVersion, err := version.ParseVersion(msg.GetGatewayVersion())
	if err != nil {
		return errors.Errorf("Failed to parse gateway version %#v: %+v",
			gatewayVersion, err)
	}

	// Parse the server version string
	serverVersion, err := version.ParseVersion(msg.GetServerVersion())
	if err != nil {
		return errors.Errorf("Failed to parse server version %#v: %+v",
			serverVersion, err)
	}

	// Check that the gateway version is compatible with the required version
	if !version.IsCompatible(requiredGateway, gatewayVersion) {
		return errors.Errorf("The gateway version %#v is incompatible with "+
			"the required version %#v.", gatewayVersion, requiredGateway)
	}

	// Check that the server version is compatible with the required version
	if !version.IsCompatible(requiredServer, serverVersion) {
		return errors.Errorf("The server version %#v is incompatible with "+
			"the required version %#v.", serverVersion, requiredServer)
	}

	return nil
}
