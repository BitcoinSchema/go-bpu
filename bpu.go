package bpu

import (
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/bitcoinschema/go-bpu/util"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
)

var tape_i uint8 = 0
var cell_i uint8 = 0

/* Parse is the main transformation function for the bpu package */
func Parse(config ParseConfig) (bpuTx *BpuTx, err error) {
	bpuTx = new(BpuTx)
	err = bpuTx.fromTx(config)
	if err != nil {
		fmt.Println("Failed tx to bpu", err)
		return nil, err
	}
	return bpuTx, nil
}

var defaultTransform Transform = func(r Cell, c string) Cell {
	return r
}

// convert a raw tx to a bpu tx
func (b *BpuTx) fromTx(config ParseConfig) (err error) {
	if len(config.RawTxHex) > 0 {
		gene, err := bt.NewTxFromString(config.RawTxHex)
		if err != nil {
			return fmt.Errorf("Failed to parse tx: %e", err)
		}

		var inXputs []XPut

		inXputs, outXputs, err := collect(config, gene.Inputs, gene.Outputs)
		if err != nil {
			return err
		}

		// fmt.Println(fmt.Sprintf("%d inputs and %d outputs", len(inXputs), len(outXputs)))
		// fmt.Println(fmt.Sprintf("%d inputs and %d outputs", len(gene.Inputs), len(gene.Outputs)))

		// convert all of the xputs to inputs
		var inputs []Input
		for idx, inXput := range inXputs {
			geneInput := gene.Inputs[idx]
			var address *string
			if idx == 0 {
				fmt.Println("oh snap", geneInput.PreviousTxScript)
			}
			if geneInput.PreviousTxScript != nil {
				addresses, err := geneInput.PreviousTxScript.Addresses()
				if err != nil {
					return err
				}
				if len(addresses) > 0 {
					address = &addresses[0]
				}
			}
			prevTxid := string(geneInput.PreviousTxID())
			inXput.E = E{
				A: address,
				V: &geneInput.PreviousTxSatoshis,
				H: &prevTxid,
			}
			inputs = append(inputs, Input{
				XPut: inXput,
				Seq:  gene.Inputs[idx].SequenceNumber,
			})

		}
		var outputs []Output
		for idx, outXput := range outXputs {
			geneOutput := gene.Outputs[idx]
			var address *string

			addresses, err := geneOutput.LockingScript.Addresses()
			if err != nil {
				return err
			}
			if len(addresses) > 0 {
				address = &addresses[0]
			}

			outXput.E = E{
				A: address,
				V: &geneOutput.Satoshis,
				I: uint32(idx),
				H: nil,
			}
			outputs = append(outputs, Output{
				XPut: outXput,
			})
			fmt.Println("Testing", *outputs[0].Tape[0].Cell[0].Ops)
		}

		txid := gene.TxID()
		b.Tx = TxInfo{
			H: txid,
		}
		b.In = inputs
		b.Out = outputs
		b.Lock = gene.LockTime
	}
	return
}

func collect(config ParseConfig, inputs []*bt.Input, outputs []*bt.Output) (xputIns []XPut, xputOuts []XPut, err error) {
	if config.Transform == nil {
		config.Transform = &defaultTransform
	}
	xputIns = make([]XPut, 0)
	if inputs != nil {
		for idx, input := range inputs {
			var xput = new(XPut)
			script := input.UnlockingScript
			tape_i = 0
			cell_i = 0
			err := xput.fromScript(config, script, uint8(idx))
			if err != nil {
				return nil, nil, err
			}
			xputIns = append(xputIns, *xput)
		}
	}
	xputOuts = make([]XPut, 0)
	if outputs != nil {
		for idx, output := range outputs {
			var xput = new(XPut)
			tape_i = 0
			cell_i = 0
			script := output.LockingScript
			err := xput.fromScript(config, script, uint8(idx))
			if err != nil {
				return nil, nil, err
			}
			xputOuts = append(xputOuts, *xput)
		}
	}

	return xputIns, xputOuts, nil
}

func (x *XPut) fromScript(config ParseConfig, script *bscript.Script, idx uint8) error {
	if script != nil {
		asm, err := script.ToASM()
		if err != nil {
			return err
		}
		chunks := strings.Split(asm, " ")
		for cIdx, chunk := range chunks {
			_, err := x.processChunk(chunk, config, uint8(cIdx), idx)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

func (x *XPut) processChunk(chunk string, o ParseConfig, chunkIndex uint8, idx uint8) (isSplitter bool, err error) {

	if x.Tape == nil {
		x.Tape = make([]Tape, 0)
	}
	isSplitter = false
	var op uint16
	var ops string
	var isOpType = false
	var splitter *IncludeType
	var bytes []byte
	var h *string
	var s *string = nil
	var b *string

	// Is chunk an opcode?
	var opByte byte

	// Some OPCODES do not come through in asm. Get decimal value. If its < 255
	if opB, ok := util.OpCodeStrings[chunk]; ok {
		isOpType = true
		opByte = opB
	} else if len(chunk) <= 4 {
		// these will be in decimal in asm for some reason

		dec, err := strconv.ParseUint(chunk, 10, 10)
		if err != nil {
			fmt.Println("Failed to parse dec", chunk)
			return isSplitter, err
		}

		tempOpByte := make([]byte, 8)
		binary.LittleEndian.PutUint64(tempOpByte, dec)
		opByte = tempOpByte[0]
		str := string(opByte)
		s = &str

		if chunk, ok := util.OpCodeValues[opByte]; ok {
			fmt.Println("Converted hex to opcode", chunk)
			isOpType = true
		}

	}

	// for some reason the | is coming through as S "124" which is the hex value not string value
	// if this was not a recognizable opcode but still came through asm with 4 digits or less its been
	// treated like an opcode and converted into decimal in the asm representation.
	// we convert to decimal and look it up
	// if len(chunk) <= 4 {
	// 	dec, err := strconv.ParseUint(chunk, 10, 10)
	// 	if err != nil {
	// 		fmt.Println("Failed to parse dec", chunk)
	// 		return isSplitter, err
	// 	}
	// 	isOpType = true
	// 	op = uint16(dec)
	// 	if chunk, ok := util.OpCodeValues[bytes[0]]; ok {
	// 		fmt.Println("Converted hex to opcode", chunk)
	// 		isOpType = true
	// 		opByte = bytes[0]
	// 	}
	// 	ops = util.OpCodeValues[uint8(op)]
	// 	s = string(uint8(op))
	// 	// if len(chunk)%2 != 0 {
	// 	// 	chunk = fmt.Sprintf("%s0", chunk)
	// 	// }
	// }

	if isOpType {
		bytes = []byte{opByte}
		hexStr := hex.EncodeToString(bytes)
		h = &hexStr

		op = uint16(opByte)
		ops = chunk
	} else {

		bytes, err = hex.DecodeString(chunk)

		if err != nil {
			fmt.Println("Failed to decode hex", chunk)
			return isSplitter, err
		}
		b64 := base64.StdEncoding.EncodeToString(bytes)
		h = &chunk
		if s == nil {
			str := string(bytes)
			s = &str
		}
		b = &b64
	}

	// Split config provided
	if o.SplitConfig != nil {
		for _, setting := range o.SplitConfig {

			if opByte, ok := util.OpCodeStrings[chunk]; ok {
				opCodeNum := uint8(opByte)
				if setting.Token != nil && (setting.Token.Op != nil && *setting.Token.Op == opCodeNum) || (setting.Token.Ops != nil && *setting.Token.Ops == chunk) {
					splitter = setting.Include
					isSplitter = true
				}
			} else {

				// Script type
				if setting.Token != nil && (setting.Token.S != nil && *setting.Token.S == *s) || (setting.Token.B != nil && *setting.Token.B == *b) {
					splitter = setting.Include
					isSplitter = true
				}
			}
		}
	}

	var cell []Cell
	if isSplitter && o.Transform != nil {
		t := *o.Transform
		var item Cell

		if splitter == nil {
			// Don't include the seperator by default, just make a new tape and reset cell
			cell = make([]Cell, 0)
			cell_i = 0
			tape_i++
		} else if *splitter == IncludeL {
			if isOpType {
				item = t(Cell{
					Op:  &op,
					Ops: &ops,
					I:   cell_i,
					II:  chunkIndex,
				}, chunk)
			} else {
				item = t(Cell{
					S:  s,
					B:  b,
					H:  h,
					I:  cell_i,
					II: chunkIndex,
				}, chunk)
			}

			cell = append(cell, item)
			cell_i++

			outTapes := append(x.Tape, Tape{Cell: cell, I: tape_i})

			// x.Tape[tape_i].Cell[cell_i] = item
			x.Tape = outTapes
			tape_i++
			cell_i = 0
			// TODO: Make sure this doesnt kill cell above or if we need to do some kinda copy
			// TODO: This was commented out but might be needed
			cell = make([]Cell, 0)
		} else if *splitter == IncludeR {
			outTapes := append(x.Tape, Tape{Cell: cell, I: tape_i})
			x.Tape = outTapes
			tape_i++
			item := t(Cell{
				Op:  &op,
				Ops: &ops,
				I:   cell_i,
				II:  chunkIndex,
			}, chunk)

			cell = []Cell{item}
			cell_i = 1

		} else if *splitter == IncludeR {
			outTapes := append(x.Tape, Tape{Cell: cell, I: tape_i})
			x.Tape = outTapes
			tape_i++
			item := t(Cell{
				Op:  &op,
				Ops: &ops,
				I:   cell_i,
				II:  chunkIndex,
			}, chunk)

			cell = []Cell{item}
			outTapes = append(outTapes, Tape{Cell: cell, I: tape_i})
			x.Tape = outTapes

			cell = make([]Cell, 0)
			cell_i = 0
		}

	} else {
		if o.Transform != nil {
			t := *o.Transform

			var item Cell
			if isOpType {
				item = t(
					Cell{Op: &op, Ops: &ops, II: chunkIndex, I: cell_i},
					chunk,
				)
			} else {
				item = t(
					Cell{B: b, S: s, H: h, II: chunkIndex, I: cell_i},
					chunk,
				)
			}

			cell_i++
			if len(x.Tape) == 0 {
				// create a new tape including the cell
				cell = append(cell, item)
				outTape := append(x.Tape, Tape{Cell: cell, I: cell_i})
				x.Tape = outTape
			} else {

				// create new tape if needed
				if len(x.Tape) == int(tape_i) {
					x.Tape = append(x.Tape, Tape{
						I: tape_i,
					})
				}
				cell = append(x.Tape[tape_i].Cell, item)

				// add the cell to the tape
				x.Tape[tape_i].Cell = cell

				// reset cell
			}

		}

	}
	return isSplitter, nil
}
