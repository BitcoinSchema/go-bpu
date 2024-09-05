package bpu

import (
	"encoding/hex"
	"fmt"
	"testing"
	"unicode"

	"github.com/bitcoin-sv/go-sdk/script"
	"github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoinschema/go-bpu/test"
	"github.com/stretchr/testify/assert"
)

var sampleTx, bitchatTx, bpuBuster, boostTx, bigOrdTx, testnetInvalidOpcode, testnetInvalid2 string

func init() {
	sampleTx = test.GetTestHex("./test/data/98a5f6ef18eaea188bdfdc048f89a48af82627a15a76fd53584975f28ab3cc39.hex")
	bitchatTx = test.GetTestHex("./test/data/653947cee3268c26efdcc97ef4e775d990e49daf81ecd2555127bda22fe5a21f.hex")
	bpuBuster = test.GetTestHex("./test/data/58d8d8407ceb37c4a04bd76ea4c78c504c4692647ad5646ecfc9bd3187cb7266.hex")
	boostTx = test.GetTestHex("./test/data/c5c7248302683107aa91014fd955908a7c572296e803512e497ddf7d1f458bd3.hex")
	bigOrdTx = test.GetTestHex("./test/data/c8cd6ff398d23e12e65ab065757fe6caf2d74b5e214b638365d61583030aa069.hex")
	testnetInvalidOpcode = test.GetTestHex("./test/data/9d49d0bdeef143efc7fae97e04a752fee1307249de8c217f86bf92c24e71afdf.hex")
	testnetInvalid2 = test.GetTestHex("./test/data/8304cff75bdcc3a73cbd06f76008522b4f13d3544435c656a958b380a8d1063c.hex")
}

var separator = "|"
var l = IncludeL
var opReturn = uint8(106)

var splitConfig = []SplitConfig{
	{
		Token: &Token{
			Op: &opReturn,
		},
		Include: &l,
	},
	{
		Token: &Token{
			S: &separator,
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
		bpuTx, err := Parse(ParseConfig{RawTxHex: &sampleTx, SplitConfig: splitConfig, Transform: &splitTransform})
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

		bpuTx, err := Parse(ParseConfig{RawTxHex: &sampleTx, SplitConfig: splitConfig})
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
		// assert.Equal(t, 3, len(bpuTx.Out[0].Tape))
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
		bpuTx, err := Parse(ParseConfig{RawTxHex: &bitchatTx, SplitConfig: splitConfig})
		if err != nil {
			fmt.Println(err)
		}
		assert.Nil(t, err)

		// fmt.Printf("bpuTx %+v\n", *bpuTx)
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
		bpuTx, err := Parse(ParseConfig{RawTxHex: &bpuBuster, SplitConfig: splitConfig})
		if err != nil {
			fmt.Println(err)
		}
		assert.Nil(t, err)

		// fmt.Printf("Tx %+v\n", *bpuTx)
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
		bpuTx, err := Parse(ParseConfig{RawTxHex: &boostTx, SplitConfig: splitConfig})
		if err != nil {
			fmt.Println(err)
		}
		assert.Nil(t, err)

		// fmt.Printf("Boost Tx %+v\n", *bpuTx)
		assert.NotNil(t, bpuTx)

		assert.Equal(t, 1, len(bpuTx.In))
		assert.Equal(t, 2, len(bpuTx.Out))

		assert.NotNil(t, bpuTx.Out[0].Tape)
		assert.NotNil(t, bpuTx.Out[0].XPut.Tape)
		assert.Equal(t, 1, len(bpuTx.Out[0].Tape))
		assert.Equal(t, 89, len(bpuTx.Out[0].Tape[0].Cell))
	})
}

func TestOrd(t *testing.T) {

	t.Run("bpu.Parse ord", func(t *testing.T) {
		bpuTx, err := Parse(ParseConfig{RawTxHex: &bigOrdTx, SplitConfig: splitConfig})
		if err != nil {
			fmt.Println(err)
		}
		assert.Nil(t, err)

		assert.NotNil(t, bpuTx)

		assert.Equal(t, 1, len(bpuTx.In))
		assert.Equal(t, 1437, len(bpuTx.Out))

		assert.NotNil(t, bpuTx.Out[0].Tape)
		assert.NotNil(t, bpuTx.Out[0].XPut.Tape)
		assert.Equal(t, 2, len(bpuTx.Out[0].Tape))
		assert.Equal(t, 14, len(bpuTx.Out[0].Tape[0].Cell))
		assert.Equal(t, 8, len(bpuTx.Out[0].Tape[1].Cell))
	})
}

func TestDecodeParts(t *testing.T) {
	gene, err := transaction.NewTransactionFromHex(testnetInvalidOpcode)
	assert.Nil(t, err)
	scr := gene.Outputs[0].LockingScript
	parts, err := script.DecodeScript(*scr)
	assert.Nil(t, err)
	assert.Equal(t, 999640, len(parts))
}

// func TestTestnetInvalidOpcodes(t *testing.T) {

// 	t.Run("testnet invalid opcodes", func(t *testing.T) {
// 		bpuTx, err := Parse(ParseConfig{RawTxHex: &testnetInvalidOpcode, SplitConfig: splitConfig})
// 		if err != nil {
// 			fmt.Println(err)
// 		}
// 		assert.Nil(t, bpuTx)
// 		assert.NotNil(t, err)
// 	})
// }

func TestTestnetInvalid2(t *testing.T) {

	t.Run("testnet invalid opcodes 2", func(t *testing.T) {
		shallow := Shallow
		bpuTx, err := Parse(ParseConfig{RawTxHex: &testnetInvalid2, SplitConfig: splitConfig, Mode: &shallow})
		assert.NotNil(t, bpuTx)
		assert.Nil(t, err)
	})
}

// TODO: Split tests

// ExampleNewFromTx example using NewFromTx()
func TestExampleNew(t *testing.T) {
	t.Run("bpu.Parse example", func(t *testing.T) {

		exampleRawTx := "010000000001000000000000000033006a07707265666978310c6578616d706c652064617461021337017c07707265666978320e6578616d706c652064617461203200000000"

		b, err := Parse(ParseConfig{RawTxHex: &exampleRawTx, SplitConfig: splitConfig})

		assert.Nil(t, err)
		assert.NotNil(t, b)

		fmt.Printf("found tx: %s", b.Tx.H)
	})
	// Output:found tx: f94e4adeac0cee5e9ff9985373622db9524e9f98d465dc024f85aec8acfeaf16
}
