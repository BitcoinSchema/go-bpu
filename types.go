package bpu

type IncludeType string

const (
	IncludeL IncludeType = "l"
	IncludeR IncludeType = "r"
	IncludeC IncludeType = "c"
)

type Transform func(o Cell, c string) (to *Cell, err error)

type Token struct {
	S   *string `json:"s" bson:"s"`
	B   *string `json:"b" bson:"b"`
	Op  *uint8  `json:"op" bson:"op"`
	Ops *string `json:"ops" bson:"ops"`
}
type SplitConfig struct {
	Token   *Token       `json:"token,omitempty" bson:"token,omitempty"`
	Include *IncludeType `json:"include,omitempty" bson:"include,omitempty"`
	Require *uint8       `json:"require,omitempty" bson:"require,omitempty"`
}

type ParseConfig struct {
	RawTxHex    string        `json:"tx" bson:"tx"`
	SplitConfig []SplitConfig `json:"split,omitempty" bson:"split,omitempty"`
	Transform   *Transform    `json:"transform,omitempty" bson:"transform,omitempty"`
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
//
// DO NOT CHANGE ORDER - aligned for memory optimization (malign)
type BpuTx struct {
	In   []Input  `json:"in"`
	Out  []Output `json:"out"`
	ID   string   `json:"_id"`
	Tx   TxInfo   `json:"tx"`
	Blk  Blk      `json:"blk"`
	Lock uint32   `json:"lock"`
}
