package bpu

import (
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"unicode"

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

var defaultTransform Transform = func(r Cell, c string) (to *Cell, err error) {
	return &r, nil
}

// convert a raw tx to a bpu tx
func (b *BpuTx) fromTx(config ParseConfig) (err error) {
	if len(config.RawTxHex) > 0 {
		// make sure raw Tx is a valid hex

		gene, err := bt.NewTxFromString(config.RawTxHex)
		if err != nil {
			return fmt.Errorf("failed to parse tx: %e", err)
		}

		var inXputs []XPut

		inXputs, outXputs, err := collect(config, gene.Inputs, gene.Outputs)
		if err != nil {
			return err
		}

		// convert all of the xputs to inputs
		var inputs []Input
		for idx, inXput := range inXputs {
			geneInput := gene.Inputs[idx]
			var address *string
			if geneInput.UnlockingScript != nil {
				gInScript := *geneInput.UnlockingScript

				// TODO: Remove this hack if libsv accepts this pr:
				// https://github.com/libsv/go-bt/pull/133
				// only a problem for input scripts

				parts, err := bscript.DecodeParts(gInScript)
				if err != nil {
					return err
				}

				if len(parts) == 2 {
					partHex := hex.EncodeToString(parts[1])
					a, err := bscript.NewAddressFromPublicKeyString(partHex, true)
					if err != nil {
						return err
					}
					address = &a.AddressString
				}
			}
			prevTxid := hex.EncodeToString(geneInput.PreviousTxID())
			inXput.E = E{
				A: address,
				V: &geneInput.PreviousTxSatoshis,
				H: &prevTxid,
				I: uint32(geneInput.PreviousTxOutIndex),
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
		}

		txid := gene.TxID()
		b.Tx = TxInfo{
			H: txid,
		}
		b.In = inputs
		b.Out = outputs
		b.Lock = gene.LockTime
	} else {
		return errors.New("raw tx must be set")
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
		parts, err := bscript.DecodeParts(*script)
		if err != nil {
			return err
		}

		for cIdx, part := range parts {
			_, err := x.processChunk(part, config, uint8(cIdx), idx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (x *XPut) processChunk(chunk []byte, o ParseConfig, chunkIndex uint8, idx uint8) (isSplitter bool, err error) {

	if x.Tape == nil {
		x.Tape = make([]Tape, 0)
	}
	isSplitter = false
	var op *uint8
	var ops *string
	var isOpType = false
	var splitter *IncludeType

	var h *string
	var s *string
	var b *string
	hexStr := hex.EncodeToString(chunk)

	// Check for valid opcodes
	var opByte byte
	if len(chunk) == 1 {
		opByte = chunk[0]
		if opCodeStr, ok := util.OpCodeValues[opByte]; ok {
			isOpType = true
			op = &opByte
			ops = &opCodeStr
		}
	}

	// Check if the byte is a printable ASCII character
	if !isOpType || unicode.IsPrint(rune(opByte)) {
		str := string(chunk)
		s = &str
		b64 := base64.StdEncoding.EncodeToString(chunk)
		b = &b64
		h = &hexStr
	}

	// Split config provided
	if o.SplitConfig != nil {
		for _, setting := range o.SplitConfig {
			if ops != nil {
				// Check if this is a manual seperator that happens to also be an opcode
				var splitOpStrPtr *string
				if splitOpByte, ok := util.OpCodeStrings[*ops]; ok {
					splitOpStr := string([]byte{splitOpByte})
					splitOpStrPtr = &splitOpStr
				}
				// or an actual op splitter

				if setting.Token != nil && (setting.Token.Op != nil && *setting.Token.Op == *op) || (setting.Token.Ops != nil && setting.Token.Ops == ops) || (setting.Token.S != nil && splitOpStrPtr != nil && *setting.Token.S == *splitOpStrPtr) {
					splitter = setting.Include
					isSplitter = true
				}
			} else {
				// Script type
				if setting.Token != nil && (s != nil && setting.Token.S != nil && *setting.Token.S == *s) || (b != nil && setting.Token.B != nil && *setting.Token.B == *b) {
					splitter = setting.Include
					isSplitter = true
				}
			}
		}
	}

	var cell []Cell
	if isSplitter && o.Transform != nil {
		t := *o.Transform
		var item *Cell

		if splitter == nil {
			// Don't include the seperator by default, just make a new tape and reset cell
			cell = make([]Cell, 0)
			cell_i = 0
			tape_i++

		} else if *splitter == IncludeL {
			if isOpType {

				item, err = t(Cell{
					Op:  op,
					Ops: ops,
					S:   s,
					H:   h,
					B:   b,
					I:   cell_i,
					II:  chunkIndex,
				}, hexStr)
			} else {
				item, err = t(Cell{
					S:  s,
					B:  b,
					H:  h,
					I:  cell_i,
					II: chunkIndex,
				}, hexStr)
			}
			if err != nil {
				return false, err
			}

			cell = append(cell, *item)
			cell_i++

			// if theres an existing tape, add item to it...
			if len(x.Tape) > 0 {
				x.Tape[len(x.Tape)-1].Cell = append(x.Tape[len(x.Tape)-1].Cell, cell...)
			} else {
				// otherwise make a new tape
				outTapes := append(x.Tape, Tape{Cell: cell, I: tape_i})
				x.Tape = outTapes
				tape_i++
			}

			cell_i = 0
			cell = make([]Cell, 0)
		} else if *splitter == IncludeC {
			outTapes := append(x.Tape, Tape{Cell: cell, I: tape_i})
			x.Tape = outTapes
			tape_i++
			item, err := t(Cell{
				Op:  op,
				Ops: ops,
				S:   s,
				H:   h,
				B:   b,
				I:   cell_i,
				II:  chunkIndex,
			}, hexStr)
			if err != nil {
				return false, err
			}

			cell = []Cell{*item}
			cell_i = 1

		} else if *splitter == IncludeR {
			outTapes := append(x.Tape, Tape{Cell: cell, I: tape_i})
			x.Tape = outTapes
			tape_i++
			item, err := t(Cell{
				Op:  op,
				Ops: ops,
				S:   s,
				H:   h,
				B:   b,
				I:   cell_i,
				II:  chunkIndex,
			}, hexStr)
			if err != nil {
				return false, err
			}

			cell = []Cell{*item}
			outTapes = append(outTapes, Tape{Cell: cell, I: tape_i})
			x.Tape = outTapes

			cell = make([]Cell, 0)
			cell_i = 0
		}

	} else {
		if o.Transform != nil {
			t := *o.Transform

			var item *Cell
			if isOpType {
				item, err = t(
					Cell{Op: op, Ops: ops, S: s,
						H: h,
						B: b, II: chunkIndex, I: cell_i},
					hexStr,
				)
			} else {
				item, err = t(
					Cell{B: b, S: s, H: h, II: chunkIndex, I: cell_i},
					hexStr,
				)
			}

			if err != nil {
				return false, err
			}

			cell_i++
			if len(x.Tape) == 0 {
				// create a new tape including the cell
				cell = append(cell, *item)
				outTape := append(x.Tape, Tape{Cell: cell, I: cell_i})
				x.Tape = outTape
			} else {

				// create new tape if needed
				if len(x.Tape) == int(tape_i) {
					x.Tape = append(x.Tape, Tape{
						I: tape_i,
					})
				}
				cell = append(x.Tape[tape_i].Cell, *item)

				// add the cell to the tape
				x.Tape[tape_i].Cell = cell

				// reset cell
			}

		}

	}
	return isSplitter, nil
}
