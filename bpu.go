// Package bpu contains the main functionality of the bpu package
package bpu

import (
	_ "embed" // This was found and leaving for now
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"unicode"

	"github.com/bitcoinschema/go-bpu/util"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
)

// Parse is the main transformation function for the bpu package
func Parse(config ParseConfig) (bpuTx *Tx, err error) {
	bpuTx = new(Tx)
	err = bpuTx.fromConfig(config)
	if err != nil {
		return nil, err
	}
	return bpuTx, nil
}

var defaultTransform Transform = func(r Cell, c string) (to *Cell, err error) {
	return &r, nil
}

// convert a raw tx to a bpu tx
func (b *Tx) fromConfig(config ParseConfig) (err error) {
	var gene *bt.Tx
	if config.Tx != nil {
		gene = config.Tx
	} else {
		if config.RawTxHex == nil || len(*config.RawTxHex) == 0 {
			return errors.New("raw tx must be set")
		}
		gene, err = bt.NewTxFromString(*config.RawTxHex)
		if err != nil {
			return fmt.Errorf("failed to parse tx: %w", err)
		}
	}

	var inXputs []XPut

	inXputs, outXputs, err := collect(config, gene.Inputs, gene.Outputs)
	if err != nil {
		return err
	}

	// convert all the xputs to inputs
	// var inputs []Input
	inputs := make([]Input, 0)
	for idx, inXput := range inXputs {
		geneInput := gene.Inputs[idx]
		var address *string
		if geneInput.UnlockingScript != nil {
			gInScript := *geneInput.UnlockingScript

			// TODO: Remove this hack if libsv accepts this pr:
			// https://github.com/libsv/go-bt/pull/133
			// only a problem for input scripts

			var parts [][]byte
			parts, err = bscript.DecodeParts(gInScript)
			if err != nil {
				return err
			}

			if len(parts) == 2 {
				partHex := hex.EncodeToString(parts[1])
				var a *bscript.Address
				a, err = bscript.NewAddressFromPublicKeyString(partHex, true)
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
			I: geneInput.PreviousTxOutIndex,
		}
		inputs = append(inputs, Input{
			XPut: inXput,
			Seq:  gene.Inputs[idx].SequenceNumber,
		})

	}
	outputs := make([]Output, 0)
	for idx, outXput := range outXputs {
		geneOutput := gene.Outputs[idx]
		var address *string

		var addresses []string
		addresses, err = geneOutput.LockingScript.Addresses()
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

	return
}

func collect(config ParseConfig, inputs []*bt.Input, outputs []*bt.Output) (xputIns []XPut,
	xputOuts []XPut, err error) {
	if config.Transform == nil {
		config.Transform = &defaultTransform
	}
	xputIns = make([]XPut, 0)

	for idx, input := range inputs {
		var xput = new(XPut)
		script := input.UnlockingScript
		err = xput.fromScript(config, script, uint8(idx))
		if err != nil {
			return nil, nil, err
		}
		xputIns = append(xputIns, *xput)
	}

	xputOuts = make([]XPut, 0)

	for idx, output := range outputs {
		var xput = new(XPut)
		script := output.LockingScript
		err = xput.fromScript(config, script, uint8(idx))
		if err != nil {
			return nil, nil, err
		}
		xputOuts = append(xputOuts, *xput)
	}

	return xputIns, xputOuts, nil
}

func (x *XPut) fromScript(config ParseConfig, script *bscript.Script, idx uint8) error {
	var tapeI uint8
	var cellI uint8

	if script != nil {
		parts, err := bscript.DecodeParts(*script)
		if err != nil {
			return err
		}
		requireMet := make(map[int]bool)
		splitterRequirementMet := make(map[int]bool)
		var prevSplitter bool
		var isSplitter bool
		for cIdx, part := range parts {
			for configIndex, req := range config.SplitConfig {
				var reqOmitted = true
				if req.Require != nil {
					reqOmitted = false

					// Look through previous parts to see if the required token is found
					chunksToCheck := parts[:cIdx]

					for _, c := range chunksToCheck {
						if len(c) == 1 && c[0] == *req.Require {
							requireMet[configIndex] = true
							break
						}
					}
				}
				splitterRequirementMet[configIndex] = reqOmitted || requireMet[configIndex]
			}
			if prevSplitter {
				cellI = 0
				// when multiple consecutive splits ocur this can be incremented too far before splitting
				// this will make sure tape is only incremented one past its length
				if len(x.Tape) > int(tapeI) {
					tapeI++
				}
				// prevSplitter = false
			}
			tapeI, cellI, isSplitter, err = x.processChunk(
				part, config, uint8(cIdx), idx, splitterRequirementMet, tapeI, cellI,
			)
			if err != nil {
				return err
			}
			prevSplitter = isSplitter
		}
	}
	return nil
}

func (x *XPut) processChunk(chunk []byte, o ParseConfig, chunkIndex uint8,
	_ uint8, requireMet map[int]bool, tapeI, cellI uint8) (uint8, uint8, bool, error) {

	if x.Tape == nil {
		x.Tape = make([]Tape, 0)
	}
	isSplitter := false
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
		for configIndex, setting := range o.SplitConfig {
			if ops != nil {
				// Check if this is a manual separator that happens to also be an opcode
				var splitOpStrPtr *string
				if splitOpByte, ok := util.OpCodeStrings[*ops]; ok {
					splitOpStr := string([]byte{splitOpByte})
					splitOpStrPtr = &splitOpStr
				}
				// or an actual op splitter
				if setting.Token != nil && (setting.Token.Op != nil && *setting.Token.Op == *op) ||
					(setting.Token.Ops != nil && setting.Token.Ops == ops) ||
					(setting.Token.S != nil && splitOpStrPtr != nil && *setting.Token.S == *splitOpStrPtr) {
					if setting.Require == nil || requireMet[configIndex] {
						splitter = setting.Include
						isSplitter = true
					}
				}
			} else {
				// Script type
				if setting.Token != nil && (s != nil && setting.Token.S != nil && *setting.Token.S == *s) ||
					(b != nil && setting.Token.B != nil && *setting.Token.B == *b) {
					if setting.Require == nil || requireMet[configIndex] {
						splitter = setting.Include

						isSplitter = true
					}
				}
			}
		}
	}

	var cell []Cell
	var err error
	if isSplitter && o.Transform != nil {
		t := *o.Transform
		var item *Cell

		if splitter == nil {
			// Don't include the separator by default, just make a new tape and reset cell
			// cell = make([]Cell, 0)
			cellI = 0
			// tapeI++

		} else if *splitter == IncludeL {
			if isOpType {

				item, err = t(Cell{
					Op:  op,
					Ops: ops,
					S:   s,
					H:   h,
					B:   b,
					I:   cellI,
					II:  chunkIndex,
				}, hexStr)
			} else {
				item, err = t(Cell{
					S:  s,
					B:  b,
					H:  h,
					I:  cellI,
					II: chunkIndex,
				}, hexStr)
			}
			if err != nil {
				return tapeI, cellI, false, err
			}

			cell = append(cell, *item)
			// cellI++

			// if there's an existing tape, add item to it...
			if len(x.Tape) > 0 {
				x.Tape[len(x.Tape)-1].Cell = append(x.Tape[len(x.Tape)-1].Cell, cell...)
			} else {
				// otherwise make a new tape
				outTapes := append(x.Tape, Tape{Cell: cell, I: tapeI})
				x.Tape = outTapes
				// tapeI++
			}

			cellI = 0
			// cell = make([]Cell, 0)
		} else if *splitter == IncludeC {
			outTapes := append(x.Tape, Tape{Cell: cell, I: tapeI})
			x.Tape = outTapes
			//tapeI++
			_, err = t(Cell{
				Op:  op,
				Ops: ops,
				S:   s,
				H:   h,
				B:   b,
				I:   cellI,
				II:  chunkIndex,
			}, hexStr)
			if err != nil {
				return tapeI, cellI, false, err
			}

			// cell = []Cell{*item}
			cellI = 1
		} else if *splitter == IncludeR {
			outTapes := append(x.Tape, Tape{Cell: cell, I: tapeI})
			x.Tape = outTapes
			//tapeI++
			item, err = t(Cell{
				Op:  op,
				Ops: ops,
				S:   s,
				H:   h,
				B:   b,
				I:   cellI,
				II:  chunkIndex,
			}, hexStr)
			if err != nil {
				return tapeI, cellI, false, err
			}

			cell = []Cell{*item}
			outTapes = append(outTapes, Tape{Cell: cell, I: tapeI})
			x.Tape = outTapes

			// cell = make([]Cell, 0)
			cellI = 0
		}
	} else {
		if o.Transform != nil {
			t := *o.Transform

			var item *Cell
			if isOpType {
				item, err = t(
					Cell{Op: op, Ops: ops, S: s,
						H: h,
						B: b, II: chunkIndex, I: cellI},
					hexStr,
				)
			} else {
				item, err = t(
					Cell{B: b, S: s, H: h, II: chunkIndex, I: cellI},
					hexStr,
				)
			}

			if err != nil {
				return tapeI, cellI, false, err
			}

			cellI++
			if len(x.Tape) == 0 {
				// create a new tape including the cell
				cell = append(cell, *item)
				outTape := append(x.Tape, Tape{Cell: cell, I: cellI})
				x.Tape = outTape
			} else {

				// create new tape if needed
				if len(x.Tape) == int(tapeI) {
					x.Tape = append(x.Tape, Tape{
						I:    tapeI,
						Cell: make([]Cell, 0),
					})
				}

				cell = append(x.Tape[tapeI].Cell, *item)

				// add the cell to the tape
				x.Tape[tapeI].Cell = cell
			}

		}

	}
	return tapeI, cellI, isSplitter, nil
}
