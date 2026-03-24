package logging

import (
	"net"
	"time"
)

// Action represents what happened with the query.
type Action int16

const (
	ActionAllowed Action = 0
	ActionBlocked Action = 1
	ActionCached  Action = 2
)

// BlockReason explains why a query was blocked.
type BlockReason int16

const (
	BlockReasonNone      BlockReason = 0
	BlockReasonBlacklist BlockReason = 1
	BlockReasonCategory  BlockReason = 2
	BlockReasonPolicy    BlockReason = 3
)

// Entry represents a single DNS query log event (RF07.1).
// This struct is sent through the async channel to the batch writer.
type Entry struct {
	QueriedAt   time.Time
	ClientIP    net.IP
	UserID      *int // nil if not authenticated
	GroupID     int
	Domain      string
	QueryType   uint16
	Action      Action
	BlockReason BlockReason
	CategoryID  *int16
	ResponseIP  net.IP
	ResponseMs  float32
	Upstream    string
}