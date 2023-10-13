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

// Parse is the main transformation function for the bpu package
func Parse(config ParseConfig) (bpuTx *BpuTx, err error) {
	bpuTx = new(BpuTx)
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
func (b *BpuTx) fromConfig(config ParseConfig) (err error) {
	var gene *bt.Tx
	if config.Tx != nil {
		gene = config.Tx
	} else {
		if config.RawTxHex == nil || len(*config.RawTxHex) == 0 {
			return errors.New("raw tx must be set")
		} else {
			gene, err = bt.NewTxFromString(*config.RawTxHex)
			if err != nil {
				return fmt.Errorf("failed to parse tx: %w", err)
			}
		}
	}

	var inXputs []XPut

	inXputs, outXputs, err := collect(config, gene.Inputs, gene.Outputs)
	if err != nil {
		return err
	}

	// convert all of the xputs to inputs
	inputs, err := processInputs(inXputs, gene.Inputs)
	if err != nil {
		return err
	}
	outputs, err := processOutputs(outXputs, gene.Outputs)
	if err != nil {
		return err
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

func processInputs(inXputs []XPut, geneInputs []*bt.Input) ([]Input, error) {
	var inputs []Input
	for idx, inXput := range inXputs {
		geneInput := geneInputs[idx]
		var address *string
		if geneInput.UnlockingScript != nil {
			gInScript := *geneInput.UnlockingScript

			// TODO: Remove this hack if libsv accepts this pr:
			// https://github.com/libsv/go-bt/pull/133
			// only a problem for input scripts

			parts, err := bscript.DecodeParts(gInScript)
			if err != nil {
				return nil, err
			}

			if len(parts) == 2 {
				partHex := hex.EncodeToString(parts[1])
				a, err := bscript.NewAddressFromPublicKeyString(partHex, true)
				if err != nil {
					return nil, err
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
			Seq:  geneInputs[idx].SequenceNumber,
		})

	}
	return inputs, nil
}

func processOutputs(outXputs []XPut, geneOutputs []*bt.Output) ([]Output, error) {
	var outputs []Output
	for idx, outXput := range outXputs {
		geneOutput := geneOutputs[idx]
		var address *string

		addresses, err := geneOutput.LockingScript.Addresses()
		if err != nil {
			return nil, err
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
	return outputs, nil
}

// splits inputs and outputs into tapes where delimeters are found (defined by parse config)
func collect(config ParseConfig, inputs []*bt.Input, outputs []*bt.Output) (xputIns []XPut, xputOuts []XPut, err error) {
	if config.Transform == nil {
		config.Transform = &defaultTransform
	}
	if inputs == nil {
		inputs = make([]*bt.Input, 0)
	}
	if outputs == nil {
		outputs = make([]*bt.Output, 0)
	}

	xputIns = make([]XPut, 0)

	for idx, input := range inputs {
		var xput = new(XPut)
		script := input.UnlockingScript
		err := xput.fromScript(config, script, uint8(idx))
		if err != nil {
			return nil, nil, err
		}
		xputIns = append(xputIns, *xput)
	}

	xputOuts = make([]XPut, 0)

	for idx, output := range outputs {
		var xput = new(XPut)
		script := output.LockingScript
		err := xput.fromScript(config, script, uint8(idx))
		if err != nil {
			return nil, nil, err
		}
		xputOuts = append(xputOuts, *xput)
	}

	return xputIns, xputOuts, nil
}

func (x *XPut) fromScript(config ParseConfig, script *bscript.Script, idx uint8) error {
	var tape_i uint8 = 0
	var cell_i uint8 = 0

	if script != nil {

		parts, err := bscript.DecodeParts(*script)
		if err != nil {
			return err
		}
		requireMet := make(map[int]bool)
		splitterRequirementMet := make(map[int]bool)
		var prevSplitter bool
		var isSplitter bool

		// If we encounter an invalid opcode as the first byte
		// of the script, skip processing the rest of the script
		if len(parts) > 0 && len(parts[0]) == 1 {
			// make sure it exists in the map
			if util.OpCodeValues[parts[0][0]] == "OP_INVALIDOPCODE" || util.OpCodeValues[parts[0][0]] == "" {
				return fmt.Errorf("script begins with invalid opcode: %x", parts[0][0])
			}
		}

		// In shallow mode we should take only the first N parts + the last N parts and concat them
		// this way we catch p2pkh prefix addresses etc, but we don't have to process the entire script
		if config.Mode != nil && *config.Mode != "deep" {
			// In shallow mode we truncate anything over 255 pushdatas (not bytes)
			if len(parts) > 255 {
				// take the first 128 parts and the last 128 parts and concat them
				parts = append(parts[:128], parts[len(parts)-128:]...)
			}
		}

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
				cell_i = 0
				// when multiple consecutive splits ocur this can be incremented too far before splitting
				// this will make sure tape is only incremented one past its length
				if len(x.Tape) > int(tape_i) {
					tape_i++
				}
				prevSplitter = false
			}
			tape_i, cell_i, isSplitter, err = x.processChunk(part, config, uint8(cIdx), idx, splitterRequirementMet, tape_i, cell_i)
			if err != nil {
				return err
			}
			prevSplitter = isSplitter
		}
	}
	return nil
}

// processes script part, identify splitters and assign to the appropriate tape
func (x *XPut) processChunk(chunk []byte, o ParseConfig, chunkIndex uint8, idx uint8, requireMet map[int]bool, tape_i, cell_i uint8) (uint8, uint8, bool, error) {

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
				// Check if this is a manual seperator that happens to also be an opcode
				var splitOpStrPtr *string
				if splitOpByte, ok := util.OpCodeStrings[*ops]; ok {
					splitOpStr := string([]byte{splitOpByte})
					splitOpStrPtr = &splitOpStr
				}
				// or an actual op splitter
				if setting.Token != nil && (setting.Token.Op != nil && *setting.Token.Op == *op) || (setting.Token.Ops != nil && setting.Token.Ops == ops) || (setting.Token.S != nil && splitOpStrPtr != nil && *setting.Token.S == *splitOpStrPtr) {
					if setting.Require == nil || requireMet[configIndex] {
						splitter = setting.Include
						isSplitter = true
					}
				}
			} else {
				// Script type
				if setting.Token != nil && (s != nil && setting.Token.S != nil && *setting.Token.S == *s) || (b != nil && setting.Token.B != nil && *setting.Token.B == *b) {
					if setting.Require == nil || requireMet[configIndex] {
						splitter = setting.Include

						isSplitter = true
					}
				}
			}
		}
	}

	if o.Transform != nil {

		if isSplitter {
			return x.processSplitterChunk(o, splitter, cell_i, isOpType, op, ops, s, h, b, chunkIndex, tape_i, hexStr)
		}

	}

	return x.processScriptChunk(o, isOpType, op, ops, s, h, b, chunkIndex, cell_i, tape_i, hexStr)
}

// process script chunk (non splitter)
func (x *XPut) processScriptChunk(
	o ParseConfig,
	isOpType bool,
	op *uint8,
	ops *string,
	s *string,
	h *string,
	b *string,
	chunkIndex uint8,
	cell_i uint8,
	tape_i uint8,
	hexStr string,
) (uint8, uint8, bool, error) {

	t := *o.Transform
	var err error
	var cell []Cell
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
		return tape_i, cell_i, false, err
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
				I:    tape_i,
				Cell: make([]Cell, 0),
			})
		}

		cell = append(x.Tape[tape_i].Cell, *item)

		// add the cell to the tape
		x.Tape[tape_i].Cell = cell
	}
	return tape_i, cell_i, false, nil
}

// process script chunk (splitter)
func (x *XPut) processSplitterChunk(
	o ParseConfig,
	splitter *IncludeType,
	cell_i uint8,
	isOpType bool,
	op *uint8,
	ops *string,
	s *string,
	h *string,
	b *string,
	chunkIndex uint8,
	tape_i uint8,
	hexStr string,
) (uint8, uint8, bool, error) {
	var err error
	t := *o.Transform
	cell := make([]Cell, 0)
	var item *Cell
	if splitter == nil {
		// Don't include the seperator by default, just make a new tape and reset cell
		// cell = make([]Cell, 0)
		cell_i = 0
		// tape_i++

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
			return tape_i, cell_i, false, err
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
			// tape_i++
		}

		cell_i = 0
		// cell = make([]Cell, 0)
	} else if *splitter == IncludeC {
		outTapes := append(x.Tape, Tape{Cell: cell, I: tape_i})
		x.Tape = outTapes
		//tape_i++
		// item, err := t(Cell{
		// 	Op:  op,
		// 	Ops: ops,
		// 	S:   s,
		// 	H:   h,
		// 	B:   b,
		// 	I:   cell_i,
		// 	II:  chunkIndex,
		// }, hexStr)
		// if err != nil {
		// 	return tape_i, cell_i, false, err
		// }

		// cell = []Cell{*item}
		cell_i = 1
	} else if *splitter == IncludeR {
		outTapes := append(x.Tape, Tape{Cell: cell, I: tape_i})
		x.Tape = outTapes
		//tape_i++
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
			return tape_i, cell_i, false, err
		}

		cell = []Cell{*item}
		outTapes = append(outTapes, Tape{Cell: cell, I: tape_i})
		x.Tape = outTapes

		//cell = make([]Cell, 0)
		cell_i = 0
	}

	return tape_i, cell_i, true, nil

}
