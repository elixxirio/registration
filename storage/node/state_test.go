package node

import (
	"gitlab.com/elixxir/primitives/current"
	"gitlab.com/elixxir/primitives/states"
	"gitlab.com/elixxir/registration/storage/round"
	"math"
	"strings"
	"testing"
	"time"
)

// tests that State update functions properly when the state it is updated
// to is not the one it is at
func TestNodeState_Update_Same(t *testing.T) {
	ns := State{
		activity: current.WAITING,
		lastPoll: time.Now(),
	}

	time.Sleep(10 * time.Millisecond)

	before := time.Now()

	updated, old, err := ns.Update(current.WAITING)
	timeDelta := ns.lastPoll.Sub(before)
	if timeDelta > (1*time.Millisecond) || timeDelta < 0 {
		t.Errorf("Time recorded is not between 0 and 1 ms from "+
			"checkpoint: %s", timeDelta)
	}

	if err != nil {
		t.Errorf("Node state update should not have errored: %s", err)
	}

	if updated == true {
		t.Errorf("Node state should not have updated")
	}

	if old != current.WAITING {
		t.Errorf("Node state returned the wrong old state")
	}

	if ns.activity != current.WAITING {
		t.Errorf("Internal node activity is not correct: "+
			"Expected: %s, Recieved: %s", current.WAITING, ns.activity)
	}
}

// tests that State update functions properly when the state it is updated
// to is not the one it is not at
func TestNodeState_Update_Invalid(t *testing.T) {
	ns := State{
		activity: current.WAITING,
		lastPoll: time.Now(),
	}

	time.Sleep(10 * time.Millisecond)

	before := time.Now()

	updated, old, err := ns.Update(current.COMPLETED)

	if err == nil {
		t.Errorf("Node state update returned no error on invalid state change")
	} else if !strings.Contains(err.Error(), "invalid transition") {
		t.Errorf("Node state update returned the wrong error on "+
			"invalid state change: %s", err)
	}

	timeDelta := ns.lastPoll.Sub(before)
	if timeDelta > (1*time.Millisecond) || timeDelta < 0 {
		t.Errorf("Time recorded is not between 0 and 1 ms from "+
			"checkpoint: %s", timeDelta)
	}

	if updated == true {
		t.Errorf("Node state should not have updated")
	}

	if old != current.WAITING {
		t.Errorf("Node state returned the wrong old state")
	}

	if ns.activity != current.WAITING {
		t.Errorf("Internal node activity is not correct: "+
			"Expected: %s, Recieved: %s", current.WAITING, ns.activity)
	}
}

// tests that State update functions properly when the state it is updated
// to is not the one it is not at
func TestNodeState_Update_Valid_RequiresRound_RoundNil(t *testing.T) {
	ns := State{
		activity: current.WAITING,
		lastPoll: time.Now(),
	}

	time.Sleep(10 * time.Millisecond)

	before := time.Now()

	updated, old, err := ns.Update(current.PRECOMPUTING)

	if err == nil {
		t.Errorf("Node state update returned no error on invalid state change")
	} else if !strings.Contains(err.Error(), "requires the node be assigned a round") {
		t.Errorf("Node state update returned the wrong error on "+
			"state change requiring round but without one: %s", err)
	}

	timeDelta := ns.lastPoll.Sub(before)
	if timeDelta > (1*time.Millisecond) || timeDelta < 0 {
		t.Errorf("Time recorded is not between 0 and 1 ms from "+
			"checkpoint: %s", timeDelta)
	}

	if updated == true {
		t.Errorf("Node state should not have updated")
	}

	if old != current.WAITING {
		t.Errorf("Node state returned the wrong old state")
	}

	if ns.activity != current.WAITING {
		t.Errorf("Internal node activity is not correct: "+
			"Expected: %s, Recieved: %s", current.WAITING, ns.activity)
	}
}

// tests that State update functions properly when the state it is updated
// to is not the one it is not at
func TestNodeState_Update_Valid_RequiresRound_Round_InvalidState(t *testing.T) {
	ns := State{
		activity:     current.WAITING,
		lastPoll:     time.Now(),
		currentRound: round.NewState_Testing(42, states.FAILED, t),
	}

	time.Sleep(10 * time.Millisecond)

	before := time.Now()

	updated, old, err := ns.Update(current.PRECOMPUTING)

	if err == nil {
		t.Errorf("Node state update returned no error on invalid state change")
	} else if !strings.Contains(err.Error(), "requires the node's be assigned a round to be in the") {
		t.Errorf("Node state update returned the wrong error on "+
			"state change requiring round in teh correct state but in wrong one: %s", err)
	}

	timeDelta := ns.lastPoll.Sub(before)
	if timeDelta > (1*time.Millisecond) || timeDelta < 0 {
		t.Errorf("Time recorded is not between 0 and 1 ms from "+
			"checkpoint: %s", timeDelta)
	}

	if updated == true {
		t.Errorf("Node state should not have updated")
	}

	if old != current.WAITING {
		t.Errorf("Node state returned the wrong old state")
	}

	if ns.activity != current.WAITING {
		t.Errorf("Internal node activity is not correct: "+
			"Expected: %s, Recieved: %s", current.WAITING, ns.activity)
	}
}

// tests that State update functions properly when the state it is updated
// to is not the one it is not at
func TestNodeState_Update_Valid_RequiresRound_Round_ValidState(t *testing.T) {
	ns := State{
		activity:     current.WAITING,
		lastPoll:     time.Now(),
		currentRound: round.NewState_Testing(42, states.PRECOMPUTING, t),
	}

	time.Sleep(10 * time.Millisecond)

	before := time.Now()

	updated, old, err := ns.Update(current.PRECOMPUTING)

	if err != nil {
		t.Errorf("Node state update returned no error on valid state change: %s", err)
	}

	timeDelta := ns.lastPoll.Sub(before)
	if timeDelta > (1*time.Millisecond) || timeDelta < 0 {
		t.Errorf("Time recorded is not between 0 and 1 ms from "+
			"checkpoint: %s", timeDelta)
	}

	if updated == false {
		t.Errorf("Node state should have updated")
	}

	if old != current.WAITING {
		t.Errorf("Node state returned the wrong old state")
	}

	if ns.activity != current.PRECOMPUTING {
		t.Errorf("Internal node activity is not correct: "+
			"Expected: %s, Recieved: %s", current.PRECOMPUTING, ns.activity)
	}
}

// tests that State update functions properly when the state it is updated
// to is not the one it is not at
func TestNodeState_Update_Valid_RequiresNoRound_HasRound(t *testing.T) {
	ns := State{
		activity:     current.COMPLETED,
		lastPoll:     time.Now(),
		currentRound: round.NewState_Testing(42, states.PRECOMPUTING, t),
	}

	time.Sleep(10 * time.Millisecond)

	before := time.Now()

	updated, old, err := ns.Update(current.WAITING)

	if err == nil {
		t.Errorf("Node state update returned no error on invalid state change")
	} else if !strings.Contains(err.Error(), "requires the node not be assigned a round") {
		t.Errorf("Node state update returned the wrong error on "+
			"state change requiring no round but has one: %s", err)
	}

	timeDelta := ns.lastPoll.Sub(before)
	if timeDelta > (1*time.Millisecond) || timeDelta < 0 {
		t.Errorf("Time recorded is not between 0 and 1 ms from "+
			"checkpoint: %s", timeDelta)
	}

	if updated == true {
		t.Errorf("Node state should not have updated")
	}

	if old != current.COMPLETED {
		t.Errorf("Node state returned the wrong old state")
	}

	if ns.activity != current.COMPLETED {
		t.Errorf("Internal node activity is not correct: "+
			"Expected: %s, Recieved: %s", current.COMPLETED, ns.activity)
	}
}

// tests that State update functions properly when the state it is updated
// to is not the one it is not at
func TestNodeState_Update_Valid_RequiresNoRound_NoRound(t *testing.T) {
	ns := State{
		activity: current.COMPLETED,
		lastPoll: time.Now(),
	}

	time.Sleep(10 * time.Millisecond)

	before := time.Now()

	updated, old, err := ns.Update(current.WAITING)

	if err != nil {
		t.Errorf("Node state update returned error on valid state change: %s", err)
	}

	timeDelta := ns.lastPoll.Sub(before)
	if timeDelta > (1*time.Millisecond) || timeDelta < 0 {
		t.Errorf("Time recorded is not between 0 and 1 ms from "+
			"checkpoint: %s", timeDelta)
	}

	if updated == false {
		t.Errorf("Node state should  have updated")
	}

	if old != current.COMPLETED {
		t.Errorf("Node state returned the wrong old state")
	}

	if ns.activity != current.WAITING {
		t.Errorf("Internal node activity is not correct: "+
			"Expected: %s, Recieved: %s", current.WAITING, ns.activity)
	}
}

//tests that GetActivity returns the correct activity
func TestNodeState_GetActivity(t *testing.T) {
	for i := 0; i < 10; i++ {
		ns := State{
			activity: current.Activity(i),
		}

		a := ns.GetActivity()

		if a != current.Activity(i) {
			t.Errorf("returned curent activity not as set"+
				"Expected: %v, Recieved: %v", a, i)
		}
	}
}

//tests that GetActivity returns the correct activity
func TestNodeState_GetLastPoll(t *testing.T) {
	ns := State{}
	for i := 0; i < 10; i++ {
		before := time.Now()
		ns.lastPoll = before
		lp := ns.GetLastPoll()

		if lp.Sub(before) != 0 {
			t.Errorf("Last Poll returned the wrong datetime")
		}
	}
}

//tests that GetActivity returns the correct activity
func TestNodeState_GetCurrentRound_Set(t *testing.T) {
	r := round.NewState_Testing(42, 0, t)
	ns := State{
		currentRound: r,
	}

	success, rnd := ns.GetCurrentRound()

	if !success {
		t.Errorf("No round is set when one should be")
	}

	if *rnd.GetRoundID() != *r.GetRoundID() {
		t.Errorf("Returned round is not correct: "+
			"Expected: %v, Recieved: %v", *r.GetRoundID(), *rnd.GetRoundID())
	}
}

//tests that GetActivity returns the correct activity
func TestNodeState_GetCurrentRound_NotSet(t *testing.T) {
	ns := State{}

	success, rnd := ns.GetCurrentRound()

	if success {
		t.Errorf("round returned when none is set")
	}

	if rnd != nil {
		t.Errorf("Returned round is not error valuve: "+
			"Expected: %v, Recieved: %v", uint64(math.MaxUint64), rnd)
	}
}

//tests that clear round sets the tracked roundID to nil
func TestNodeState_ClearRound(t *testing.T) {
	r := round.State{}

	ns := State{
		currentRound: &r,
	}

	ns.ClearRound()

	if ns.currentRound != nil {
		t.Errorf("The curent round was not nilled")
	}
}

//tests that clear round sets the tracked roundID to nil
func TestNodeState_SetRound_Valid(t *testing.T) {
	r := round.NewState_Testing(42, 2, t)

	ns := State{
		currentRound: nil,
	}

	err := ns.SetRound(r)

	if err != nil {
		t.Errorf("SetRound returned an error which it should be "+
			"sucesfull: %s", err)
	}

	if ns.currentRound == nil {
		t.Errorf("Round not updated")
	}
}

//tests that clear round does not set the tracked roundID errors when one is set
func TestNodeState_SetRound_Invalid(t *testing.T) {
	r := round.NewState_Testing(42, 0, t)
	storedR := round.NewState_Testing(69, 0, t)

	ns := State{
		currentRound: storedR,
	}

	err := ns.SetRound(r)

	if err == nil {
		t.Errorf("SetRound did not an error which it should have failed")
	} else if !strings.Contains(err.Error(), "could not set the Node's "+
		"round when it is already set") {
		t.Errorf("Incorrect error returned from failed SetRound: %s", err)
	}

	if ns.currentRound.GetRoundID() != 69 {
		t.Errorf("Round not updated to the correct value; "+
			"Expected: %v, Recieved: %v", 69, ns.currentRound.GetRoundID())
	}
}
