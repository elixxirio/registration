////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles creating client registration callbacks for hooking into comms library

package cmd

import (
	"crypto/rand"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/registration/storage"
	"gitlab.com/xx_network/crypto/signature/rsa"
)

var rateLimitErr = errors.New("Too many client registrations. Try again later")

// Handle registration attempt by a Client
// Returns rsa signature and error
func (m *RegistrationImpl) RegisterUser(regCode string, pubKey string, receptionKey string) ([]byte, []byte, error) {
	// Check for pre-existing registration for this public key first
	if user, err := storage.PermissioningDb.GetUser(pubKey); err == nil && user != nil {
		jww.WARN.Printf("Previous registration found for %s", pubKey)
	} else if regCode != "" {
		// Fail early for non-valid reg codes
		err = storage.PermissioningDb.UseCode(regCode)
		if err != nil {
			jww.WARN.Printf("RegisterUser error: %+v", err)
			return nil, nil, err
		}
	} else if regCode == "" && !m.registrationLimiting.Add(1) {
		// Rate limited, fail early
		jww.WARN.Printf("RegisterUser error: %+v", rateLimitErr)
		return nil, nil, rateLimitErr
	}

	// Use hardcoded keypair to sign Client-provided public key
	//Create a hash, hash the pubKey and then truncate it
	h, _ := hash.NewCMixHash()
	h.Write([]byte(pubKey))
	transmissionSig, err := rsa.Sign(rand.Reader, m.State.GetPrivateKey(), hash.CMixHash, h.Sum(nil), nil)
	if err != nil {
		jww.WARN.Printf("RegisterUser error: can't sign pubkey")
		return make([]byte, 0), make([]byte, 0), errors.Errorf(
			"Unable to sign client public key: %+v", err)
	}

	h.Reset()
	h.Write([]byte(receptionKey))
	receptionSig, err := rsa.Sign(rand.Reader, m.State.GetPrivateKey(), hash.CMixHash, h.Sum(nil), nil)
	// Record the user public key for duplicate registration support
	err = storage.PermissioningDb.InsertUser(pubKey, receptionKey)
	if err != nil {
		jww.WARN.Printf("Unable to store user: %+v",
			errors.New(err.Error()))
	}

	// Return signed public key to Client
	jww.DEBUG.Printf("RegisterUser for code [%s] complete!", regCode)
	return transmissionSig, receptionSig, nil
}
