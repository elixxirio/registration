////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles the Map backend for the permissioning server

package storage

import (
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/registration/storage/node"
)

// If Node registration code is valid, add Node information
func (m *MapImpl) InsertNode(id []byte, code, serverCert, serverAddress,
	gatewayAddress, gatewayCert string) error {
	m.mut.Lock()
	jww.INFO.Printf("Attempting to register node with code: %s", code)
	if info := m.node[code]; info != nil {
		info.Id = id
		info.ServerAddress = serverAddress
		info.GatewayCertificate = gatewayCert
		info.GatewayAddress = gatewayAddress
		info.NodeCertificate = serverCert
		m.mut.Unlock()
		return nil
	}
	m.mut.Unlock()
	return errors.Errorf("unable to register node %s", code)

}

// Insert Node registration code into the database
func (m *MapImpl) InsertNodeRegCode(info node.Info) error {
	m.mut.Lock()
	jww.INFO.Printf("Adding node registration code: %s with Order Info: %s",
		info.RegCode, info.Order)

	// Enforce unique registration code
	if m.node[info.RegCode] != nil {
		m.mut.Unlock()
		return errors.Errorf("node registration code %s already exists",
			info.RegCode)
	}

	m.node[info.RegCode] =
		&NodeInformation{Code: info.RegCode, Order: info.Order}
	m.mut.Unlock()
	return nil
}

// Get Node information for the given Node registration code
func (m *MapImpl) GetNode(code string) (*NodeInformation, error) {
	m.mut.Lock()
	info := m.node[code]
	if info == nil {
		m.mut.Unlock()
		return nil, errors.Errorf("unable to get node %s", code)
	}
	m.mut.Unlock()
	return info, nil
}
