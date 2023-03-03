package bpu

import (
	"encoding/hex"
	"fmt"
	"testing"
	"unicode"

	"github.com/bitcoinschema/go-bpu/test"
	"github.com/stretchr/testify/assert"
)

var sampleTx, bitchatTx, bpuBuster, boostTx string

func init() {
	sampleTx = test.GetTestHex("./test/data/98a5f6ef18eaea188bdfdc048f89a48af82627a15a76fd53584975f28ab3cc39.hex")
	bitchatTx = test.GetTestHex("./test/data/653947cee3268c26efdcc97ef4e775d990e49daf81ecd2555127bda22fe5a21f.hex")
	bpuBuster = test.GetTestHex("./test/data/58d8d8407ceb37c4a04bd76ea4c78c504c4692647ad5646ecfc9bd3187cb7266.hex")
	boostTx = test.GetTestHex("./test/data/c5c7248302683107aa91014fd955908a7c572296e803512e497ddf7d1f458bd3.hex")
}

var seperator = "|"
var l = IncludeL
var opReturn = uint8(106)
var opFalse = uint8(0)

var splitConfig = []SplitConfig{
	{
		Token: &Token{
			Op: &opReturn,
		},
		Include: &l,
	},
	{
		Token: &Token{
			Op: &opFalse,
		},
		Include: &l,
	},
	{
		Token: &Token{
			S: &seperator,
		},
		Require: &opReturn,
	},
}

var splitTransform Transform = func(o Cell, c string) (to *Cell, e error) {
	// if the buffer is larger than 512 bytes,
	// replace the key with "l" prepended attribute
	to = &o
	bytes, err := hex.DecodeString(c)
	if err != nil {
		return nil, err
	}
	if len(bytes) > 512 {
		to.LS = to.S
		to.LB = to.B
		to.S = nil
		to.B = nil
	}
	return to, nil
}

func TestTransform(t *testing.T) {
	t.Run("bpu.Transform", func(t *testing.T) {
		bpuTx, err := Parse(ParseConfig{RawTxHex: sampleTx, SplitConfig: splitConfig, Transform: &splitTransform})
		if err != nil {
			fmt.Println(err)
		}

		assert.Nil(t, err)
		assert.NotNil(t, bpuTx)
		// TODO: Test the transform actually worked with a large tx
	})
}

func TestBpu(t *testing.T) {

	t.Run("bpu.Parse", func(t *testing.T) {

		bpuTx, err := Parse(ParseConfig{RawTxHex: sampleTx, SplitConfig: splitConfig})
		if err != nil {
			fmt.Println(err)
		}
		assert.Nil(t, err)
		assert.NotNil(t, bpuTx)

		// Inputs
		assert.Equal(t, 1, len(bpuTx.In))
		assert.NotNil(t, bpuTx.In[0].Tape)
		assert.Equal(t, 1, len(bpuTx.In[0].Tape))
		assert.NotNil(t, len(bpuTx.In[0].Tape[0].Cell))
		assert.NotNil(t, bpuTx.In[0].Tape[0].Cell[0])
		assert.Nil(t, bpuTx.In[0].Tape[0].Cell[0].Op)
		assert.Equal(t, "MEUCIQC6inN+3xNzbLGYzO+Jf1fiQsO7byIsY38SBdgFDb0iOQIgBivsk7RvZJ9C+XFDia33fWyhkyEbiRI25i3k+I+a+6lB", *bpuTx.In[0].Tape[0].Cell[0].B)
		assert.Equal(t, uint8(0), bpuTx.In[0].Tape[0].Cell[0].I)
		assert.Equal(t, uint8(0), bpuTx.In[0].Tape[0].Cell[0].II)
		assert.NotNil(t, bpuTx.In[0].E)
		assert.NotNil(t, bpuTx.In[0].E.A)
		assert.Equal(t, "1LC16EQVsqVYGeYTCrjvNf8j28zr4DwBuk", *bpuTx.In[0].E.A)
		assert.Equal(t, "2b3067e50f92b7052571cd0d66c4e0071c1d79fc7248481980a454f5c6851e3a", *bpuTx.In[0].E.H)
		assert.Equal(t, uint32(1), bpuTx.In[0].E.I)
		assert.NotNil(t, bpuTx.Tx)
		assert.Equal(t, "98a5f6ef18eaea188bdfdc048f89a48af82627a15a76fd53584975f28ab3cc39", bpuTx.Tx.H)

		// Outputs
		assert.Equal(t, 2, len(bpuTx.Out))
		assert.Equal(t, uint8(0), bpuTx.Out[0].I)
		assert.NotNil(t, bpuTx.Out[0].Tape)
		assert.Equal(t, 3, len(bpuTx.Out[0].Tape))
		assert.NotNil(t, bpuTx.Out[0].Tape[0].Cell)
		assert.Equal(t, 1, len(bpuTx.Out[0].Tape[0].Cell))
		assert.NotNil(t, bpuTx.Out[0].Tape[0].Cell[0])
		assert.Equal(t, uint8(106), *bpuTx.Out[0].Tape[0].Cell[0].Op)
		assert.Equal(t, "OP_RETURN", *bpuTx.Out[0].Tape[0].Cell[0].Ops)
		assert.Equal(t, "1BAPSuaPnfGnSBM3GLV9yhxUdYe4vGbdMT", *bpuTx.Out[0].Tape[1].Cell[0].S)
		assert.NotNil(t, bpuTx.Out[0].Tape[1].Cell[3].S)
		assert.NotNil(t, bpuTx.Out[0].Tape[1].Cell[3].B)
		assert.Equal(t, "MA==", *bpuTx.Out[0].Tape[1].Cell[3].B)
		assert.Equal(t, uint8(4), bpuTx.Out[0].Tape[1].Cell[3].II)
		assert.Equal(t, uint8(3), bpuTx.Out[0].Tape[1].Cell[3].I)
	})
}

func TestUnicode(t *testing.T) {
	char := "供"
	assert.Equal(t, true, unicode.IsPrint(rune(char[0])))
}

func TestBpuBitchat(t *testing.T) {

	t.Run("bpu.Parse bitchat", func(t *testing.T) {
		bpuTx, err := Parse(ParseConfig{RawTxHex: bitchatTx, SplitConfig: splitConfig})
		if err != nil {
			fmt.Println(err)
		}
		assert.Nil(t, err)

		fmt.Printf("bpuTx %+v\n", *bpuTx)
		assert.NotNil(t, bpuTx)

		assert.Equal(t, 1, len(bpuTx.In))
		assert.Equal(t, 2, len(bpuTx.Out))

		assert.NotNil(t, bpuTx.Out[0].Tape)
		assert.NotNil(t, bpuTx.Out[0].XPut.Tape)
		assert.Equal(t, 2, len(bpuTx.Out[0].Tape[0].Cell))
	})
}

func TestBpuBuster(t *testing.T) {

	t.Run("bpu.Parse bpu buster", func(t *testing.T) {
		bpuTx, err := Parse(ParseConfig{RawTxHex: bpuBuster, SplitConfig: splitConfig})
		if err != nil {
			fmt.Println(err)
		}
		assert.Nil(t, err)

		fmt.Printf("BpuTx %+v\n", *bpuTx)
		assert.NotNil(t, bpuTx)

		assert.Equal(t, 1, len(bpuTx.In))
		assert.Equal(t, 1, len(bpuTx.Out))

		assert.NotNil(t, bpuTx.Out[0].Tape)
		assert.NotNil(t, bpuTx.Out[0].XPut.Tape)
		assert.Equal(t, 2, len(bpuTx.Out[0].Tape[0].Cell))
		assert.Equal(t, 3, len(bpuTx.Out[0].Tape[1].Cell))
	})
}

func TestBoost(t *testing.T) {

	t.Run("bpu.Parse boost", func(t *testing.T) {
		bpuTx, err := Parse(ParseConfig{RawTxHex: boostTx, SplitConfig: splitConfig})
		if err != nil {
			fmt.Println(err)
		}
		assert.Nil(t, err)

		fmt.Printf("Boost Tx %+v\n", *bpuTx)
		assert.NotNil(t, bpuTx)

		assert.Equal(t, 1, len(bpuTx.In))
		assert.Equal(t, 2, len(bpuTx.Out))

		assert.NotNil(t, bpuTx.Out[0].Tape)
		assert.NotNil(t, bpuTx.Out[0].XPut.Tape)
		assert.Equal(t, 1, len(bpuTx.Out[0].Tape))
		assert.Equal(t, 89, len(bpuTx.Out[0].Tape[0].Cell))
	})
}

// TODO: Split tests
