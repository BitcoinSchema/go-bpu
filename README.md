# Go-BPU

> Bitcoin Processing Unit

Transform Bitcoin Transactions into Virtual Procedure Call Units.

![bpu](./bpu.png)

Transforms raw transactions to BOB format. Port from the original [bpu](https://github.com/interplanaria/bpu) library bu [unwriter](https://github.com/unwriter)

Since this is intended to be used by low level transactoin parsers dependencies are kept to a bare minimum. It does not include the RPC client functionality that connects to a node to get a raw tx. Its designed to be a fast raw tx to BOB processor.

There is also a [Typescript version](https://github.con/rohenaz/bpu-ts) which does include the originally RPC functionality.

# Usage

## Split Config
```go
var seperator = "|"
var l = bpu.IncludeL
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
	},
}

bpuTx, err := Parse(ParseConfig{RawTxHex: sampleTx, SplitConfig: splitConfig})
if err != nil {
  fmt.Println(err)
}
```

## Transform Function
You can pass an optional Transform function to bpu.Parse. Function should look something like this:

```go
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
```

# Usage details
See the [Typescript library](https://github.com/rohenaz/bpu-ts) for more examples of split configuration options, transformation, and a look at the output.

# Errata
The original BPU library used bsv (javascript) v1.5 to determine if a script chunk was a valid opcode. At the time, the bsv library supported a limited number of OP codes (inherited from limitations uimposed by Bitcoin core). In this version all opcodes are recognized which surfaces a new issue where fields previously available would be missing if the data is now recognized as an opcode. 

Previously, BPU would omit the op and ops fields for non opcode data, while recognized opcodes would omit the s, b and h fields. To solve the issue of missing fields that happen to be opcodes, all keys are included if the recognized pushdata is also in the Printable ASCII range.