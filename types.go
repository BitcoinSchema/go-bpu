package bpu

import (
	"github.com/bsv-blockchain/go-sdk/transaction"
)

// IncludeType is the type of include
type IncludeType string

// Include types
const (
	IncludeL IncludeType = "l"
	IncludeR IncludeType = "r"
	IncludeC IncludeType = "c"
)

// Transform is a function to transform a cell
type Transform func(o Cell, c string) (to *Cell, err error)

// Token is a token to split on
type Token struct {
	S   *string `json:"s" bson:"s"`
	B   *string `json:"b" bson:"b"`
	Op  *uint8  `json:"op" bson:"op"`
	Ops *string `json:"ops" bson:"ops"`
}

// SplitConfig is the configuration for splitting a transaction
type SplitConfig struct {
	Token   *Token       `json:"token,omitempty" bson:"token,omitempty"`
	Include *IncludeType `json:"include,omitempty" bson:"include,omitempty"`
	Require *uint8       `json:"require,omitempty" bson:"require,omitempty"`
}

// Mode is either deep or shallow
type Mode string

const (
	// Deep mode evalurates every pushdata regardless of quantity
	Deep Mode = "deep"
	// Shallow mode only evaluates the first 128 pushdata and the last 128 pushdatas
	Shallow Mode = "shallow"
)

// ParseConfig is the configuration for parsing a transaction
type ParseConfig struct {
	Tx          *transaction.Transaction `json:"tx" bson:"tx"`
	RawTxHex    *string                  `json:"rawTx" bson:"rawTx"`
	SplitConfig []SplitConfig            `json:"split,omitempty" bson:"split,omitempty"`
	Transform   *Transform               `json:"transform,omitempty" bson:"transform,omitempty"`
	Mode        *Mode                    `json:"mode,omitempty" bson:"mode,omitempty"`
}

// E has address and value information
type E struct {
	A *string `json:"a,omitempty" bson:"a,omitempty"`
	V *uint64 `json:"v,omitempty" bson:"v,omitempty"`
	I uint32  `json:"i" bson:"i"`
	H *string `json:"h,omitempty" bson:"h,omitempty"`
}

// Cell is a single OP_RETURN protocol
type Cell struct {
	H   *string `json:"h,omitempty" bson:"h,omitempty"`
	B   *string `json:"b,omitempty" bson:"b,omitempty"`
	LB  *string `json:"lb,omitempty" bson:"lb,omitempty"`
	S   *string `json:"s,omitempty" bson:"s,omitempty"`
	LS  *string `json:"ls,omitempty" bson:"ls,omitempty"`
	I   uint8   `json:"i" bson:"i"`
	II  uint8   `json:"ii" bson:"ii"`
	Op  *uint8  `json:"op,omitempty" bson:"op,omitempty"`
	Ops *string `json:"ops,omitempty" bson:"ops,omitempty"`
}

// XPut is a transaction input or output
type XPut struct {
	I    uint8  `json:"i"`
	Tape []Tape `json:"tape"`
	E    E      `json:"e,omitempty"`
}

// Input is a transaction input
type Input struct {
	XPut
	Seq uint32 `json:"seq" bson:"seq"`
}

// Output is a transaction output
type Output struct {
	XPut
}

// Tape is a tape
type Tape struct {
	Cell []Cell `json:"cell"`
	I    uint8  `json:"i"`
}

// Blk contains the block info
type Blk struct {
	I uint32 `json:"i"`
	T uint32 `json:"t"`
}

// TxInfo contains the transaction info
type TxInfo struct {
	H string `json:"h"`
}

// Tx is a BOB formatted Bitcoin transaction
type Tx struct {
	In   []Input  `json:"in"`
	Out  []Output `json:"out"`
	ID   string   `json:"_id"`
	Tx   TxInfo   `json:"tx"`
	Blk  Blk      `json:"blk"`
	Lock uint32   `json:"lock"`
}
