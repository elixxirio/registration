////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Handles low level Database structures and interfaces

package storage

import (
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/registration/storage/node"
	"gitlab.com/xx_network/primitives/id"
	"sync"
	"time"
)

// Interface declaration for Storage methods
type database interface {
	// Permissioning methods
	UpsertState(state *State) error
	GetStateValue(key string) (string, error)
	InsertNodeMetric(metric *NodeMetric) error
	InsertRoundMetric(metric *RoundMetric, topology [][]byte) error
	InsertRoundError(roundId id.Round, errStr string) error
	GetLatestEphemeralLength() (*EphemeralLength, error)
	GetEphemeralLengths() ([]*EphemeralLength, error)
	InsertEphemeralLength(length *EphemeralLength) error
	GetEarliestRound(cutoff time.Duration) (id.Round, time.Time, error)
	getBins() ([]*GeoBin, error)

	// Node methods
	InsertApplication(application *Application, unregisteredNode *Node) error
	RegisterNode(id *id.ID, salt []byte, code, serverAddr, serverCert,
		gatewayAddress, gatewayCert string) error
	UpdateNodeAddresses(id *id.ID, nodeAddr, gwAddr string) error
	UpdateNodeSequence(id *id.ID, sequence string) error
	UpdateGeoIP(appId uint64, location, geoBin, gpsLocation string) error
	updateLastActive(ids [][]byte, lastActive time.Time) error
	GetNode(code string) (*Node, error)
	GetNodes() ([]*Node, error)
	GetNodeById(id *id.ID) (*Node, error)
	GetNodesByStatus(status node.Status) ([]*Node, error)
	GetActiveNodes() ([]*ActiveNode, error)
}

// Struct implementing the Database Interface with an underlying Map
type MapImpl struct {
	nodes             map[string]*Node
	applications      map[uint64]*Application
	nodeMetrics       map[uint64]*NodeMetric
	nodeMetricCounter uint64
	roundMetrics      map[uint64]*RoundMetric
	states            map[string]string
	ephemeralLengths  map[uint8]*EphemeralLength
	activeNodes       map[id.ID]*ActiveNode
	geographicBin     map[string]uint8
	mut               sync.Mutex
}

// Key-Value store used for persisting Permissioning State information
type State struct {
	Key   string `gorm:"primary_key"`
	Value string `gorm:"NOT NULL"`
}

// Enumerates Keys in the State table
const (
	// Used internally
	UpdateIdKey = "UpdateId"
	RoundIdKey  = "RoundId"
	EllipticKey = "EllipticKey"

	// Provided externally
	PrecompTimeout       = "timeouts_precomputation"
	RealtimeTimeout      = "timeouts_realtime"
	AdvertisementTimeout = "timeouts_advertisement"
	TeamSize             = "scheduling_team_size"
	BatchSize            = "scheduling_batch_size"
	MinDelay             = "scheduling_min_delay"
	PoolThreshold        = "scheduling_pool_threshold"

	// TODO: Client reg repo?
	MaxRegistrations   = "registration_max"
	RegistrationPeriod = "registration_period"
)

// Struct representing the Node's Application table in the Database
type Application struct {
	// The Application's unique ID
	Id uint64 `gorm:"primary_key;AUTO_INCREMENT:false"`
	// Each Application has one Node
	Node Node `gorm:"foreignkey:ApplicationId"`

	// Node information
	Name  string
	Url   string
	Blurb string
	Other string

	// Location string for the Node
	Location string
	// Geographic bin of the Node's location
	GeoBin string
	// GPS location of the Node
	GpsLocation string
	// Specifies the team the node was assigned
	Team string
	// Specifies which network the node is in
	Network string

	// Social media
	Forum     string
	Email     string
	Twitter   string
	Discord   string
	Instagram string
	Medium    string
}

// Struct representing the ActiveNode table in the Database
type ActiveNode struct {
	WalletAddress string `gorm:"primary_key"`
	Id            []byte `gorm:"NOT NULL;UNIQUE"`
}

// Struct representing the GeoBin table in the Database
type GeoBin struct {
	Country string `gorm:"primary_key"`
	Bin     uint8  `gorm:"NOT NULL"`
}

// Struct representing the Node table in the Database
type Node struct {
	// Registration code acts as the primary key
	Code string `gorm:"primary_key"`
	// Node order string, this is a tag used by the algorithm
	Sequence string

	// Unique Node ID
	Id []byte `gorm:"UNIQUE_INDEX;default: null"`
	// Salt used for generation of Node ID
	Salt []byte
	// Server IP address
	ServerAddress string
	// Gateway IP address
	GatewayAddress string
	// Node TLS public certificate in PEM string format
	NodeCertificate string
	// Gateway TLS public certificate in PEM string format
	GatewayCertificate string

	// Date/time that the node was registered
	DateRegistered time.Time
	// Date/time that the node was last active
	LastActive time.Time
	// Node's network status
	Status uint8 `gorm:"NOT NULL"`

	// Unique ID of the Node's Application
	ApplicationId uint64 `gorm:"UNIQUE_INDEX;NOT NULL;type:bigint REFERENCES applications(id)"`

	// Each Node has many Node Metrics
	NodeMetrics []NodeMetric `gorm:"foreignkey:NodeId;association_foreignkey:Id"`

	// Each Node participates in many Rounds
	Topologies []Topology `gorm:"foreignkey:NodeId;association_foreignkey:Id"`
}

// Struct representing Node Metrics table in the Database
type NodeMetric struct {
	// Auto-incrementing primary key (Do not set)
	Id uint64 `gorm:"primary_key;AUTO_INCREMENT:true"`
	// Node has many NodeMetrics
	NodeId []byte `gorm:"INDEX;NOT NULL;type:bytea REFERENCES nodes(Id)"`
	// Start time of monitoring period
	StartTime time.Time `gorm:"NOT NULL"`
	// End time of monitoring period
	EndTime time.Time `gorm:"NOT NULL"`
	// Number of pings responded to during monitoring period
	NumPings uint64 `gorm:"NOT NULL"`
}

// Junction table for the many-to-many relationship between Nodes & RoundMetrics
type Topology struct {
	// Composite primary key
	NodeId        []byte `gorm:"primary_key;type:bytea REFERENCES nodes(Id)"`
	RoundMetricId uint64 `gorm:"INDEX;primary_key;type:bigint REFERENCES round_metrics(Id)"`

	// Order in the topology of a Node for a given Round
	Order uint8 `gorm:"NOT NULL"`
}

// Struct representing Round Metrics table in the Database
type RoundMetric struct {
	// Unique ID of the round as assigned by the network
	Id uint64 `gorm:"primary_key;AUTO_INCREMENT:false"`

	// Round timestamp information
	PrecompStart  time.Time `gorm:"NOT NULL"`
	PrecompEnd    time.Time `gorm:"NOT NULL;INDEX;"`
	RealtimeStart time.Time `gorm:"NOT NULL"`
	RealtimeEnd   time.Time `gorm:"NOT NULL;INDEX;"`                        // Index for TPS calc
	RoundEnd      time.Time `gorm:"NOT NULL;INDEX;default:to_timestamp(0)"` // Index for TPS calc
	BatchSize     uint32    `gorm:"NOT NULL"`

	// Each RoundMetric has many Nodes participating in each Round
	Topologies []Topology `gorm:"foreignkey:RoundMetricId;association_foreignkey:Id"`

	// Each RoundMetric can have many Errors in each Round
	RoundErrors []RoundError `gorm:"foreignkey:RoundMetricId;association_foreignkey:Id"`
}

// Struct representing Round Errors table in the Database
type RoundError struct {
	// Auto-incrementing primary key (Do not set)
	Id uint64 `gorm:"primary_key;AUTO_INCREMENT:true"`

	// ID of the round for a given run of the network
	RoundMetricId uint64 `gorm:"INDEX;NOT NULL;type:bigint REFERENCES round_metrics(Id)"`

	// String of error that occurred during the Round
	Error string `gorm:"NOT NULL"`
}

// Struct represegnting the validity period of an ephemeral ID length
type EphemeralLength struct {
	Length    uint8     `gorm:"primary_key;AUTO_INCREMENT:false"`
	Timestamp time.Time `gorm:"NOT NULL;UNIQUE"`
}

// Struct representing Round Metrics table in the Database
// This table exists to enable creating the round_metrics table using sqlite.
// The default on the main table uses a postgres_only function, so it cannot be used.
type RoundMetricAlt struct {
	// Unique ID of the round as assigned by the network
	Id uint64 `gorm:"primary_key;AUTO_INCREMENT:false"`

	// Round timestamp information
	PrecompStart  time.Time `gorm:"NOT NULL"`
	PrecompEnd    time.Time `gorm:"NOT NULL;INDEX;"`
	RealtimeStart time.Time `gorm:"NOT NULL"`
	RealtimeEnd   time.Time `gorm:"NOT NULL;INDEX;"` // Index for TPS calc
	RoundEnd      time.Time `gorm:"NOT NULL;INDEX;"` // Index for TPS calc
	BatchSize     uint32    `gorm:"NOT NULL"`

	// Each RoundMetric has many Nodes participating in each Round
	Topologies []Topology `gorm:"foreignkey:RoundMetricId;association_foreignkey:Id"`

	// Each RoundMetric can have many Errors in each Round
	RoundErrors []RoundError `gorm:"foreignkey:RoundMetricId;association_foreignkey:Id"`
}

// Interface method which overrides the name of the table when created with gorm
func (RoundMetricAlt) TableName() string { return "round_metrics" }

// Adds Node registration codes to the Database
func PopulateNodeRegistrationCodes(infos []node.Info) {
	// TODO: This will eventually need to be updated to intake applications too
	i := 1
	for _, info := range infos {
		err := PermissioningDb.InsertApplication(&Application{
			Id: uint64(i),
		}, &Node{
			Code:          info.RegCode,
			Sequence:      info.Order,
			ApplicationId: uint64(i),
		})
		if err != nil {
			jww.ERROR.Printf("Unable to populate Node registration code: %+v",
				err)
		}
		i++
	}
}
