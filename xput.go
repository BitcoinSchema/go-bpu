package bpu

import (
	"encoding/base64"
	"encoding/hex"
	"unicode"

	"github.com/bsv-blockchain/go-sdk/script"
)

// "XPut" refers to "inut or output"
// and is a struct that contains the "tape"
// which is divided up according to the "splitConfig"
func (x *XPut) fromScript(config ParseConfig, scrpt *script.Script, idx uint8) error {
	var tapeI uint8
	var cellI uint8

	if scrpt != nil {

		parts, err := script.DecodeScript(*scrpt, script.DecodeOptionsParseOpReturn)
		if err != nil {
			return err
		}
		requireMet := make(map[int]bool)
		splitterRequirementMet := make(map[int]bool)
		var prevSplitter bool
		var isSplitter bool

		// // If we encounter an invalid opcode as the first byte
		// // of the script, skip processing the rest of the script
		// if len(parts) > 0 && len(parts[0]) == 1 {
		// 	// make sure it exists in the map
		// 	if util.OpCodeValues[parts[0][0]] == "OP_INVALIDOPCODE" || util.OpCodeValues[parts[0][0]] == "" {
		// 		return fmt.Errorf("script begins with invalid opcode: %x", parts[0][0])
		// 	}
		// }

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
						if c.Op == *req.Require || (len(c.Data) == 1 && c.Data[0] == *req.Require) {
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

func (x *XPut) processChunk(chunk *script.ScriptChunk, o ParseConfig, chunkIndex uint8,
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
	hexStr := hex.EncodeToString(chunk.Data)

	// Check for valid opcodes
	var opByte byte
	if chunk.Op == 0 || chunk.Op > script.OpPUSHDATA4 {
		opByte = chunk.Op
		if opCodeStr, ok := script.OpCodeValues[opByte]; ok {
			isOpType = true
			op = &opByte
			ops = &opCodeStr
		}
	}

	// Check if the byte is a printable ASCII character
	if !isOpType || unicode.IsPrint(rune(opByte)) {
		str := string(chunk.Data)
		s = &str
		b64 := base64.StdEncoding.EncodeToString(chunk.Data)
		b = &b64
		h = &hexStr
	}

	// Split config provided
	if o.SplitConfig != nil {
		for configIndex, setting := range o.SplitConfig {
			if ops != nil {
				// Check if this is a manual separator that happens to also be an opcode
				var splitOpStrPtr *string
				if splitOpByte, ok := script.OpCodeStrings[*ops]; ok {
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

	if o.Transform != nil {

		if isSplitter {
			return x.processSplitterChunk(o, splitter, cellI, isOpType, op, ops, s, h, b, chunkIndex, tapeI, hexStr)
		}

	}

	return x.processScriptChunk(o, isOpType, op, ops, s, h, b, chunkIndex, cellI, tapeI, hexStr)
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
	cellI uint8,
	tapeI uint8,
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
	return tapeI, cellI, false, nil
}

// process script chunk (splitter)
func (x *XPut) processSplitterChunk(
	o ParseConfig,
	splitter *IncludeType,
	cellI uint8,
	isOpType bool,
	op *uint8,
	ops *string,
	s *string,
	h *string,
	b *string,
	chunkIndex uint8,
	tapeI uint8,
	hexStr string,
) (uint8, uint8, bool, error) {
	var err error
	t := *o.Transform
	cell := make([]Cell, 0)
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
		//cellI++

		// if theres an existing tape, add item to it...
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
		// item, err := t(Cell{
		// 	Op:  op,
		// 	Ops: ops,
		// 	S:   s,
		// 	H:   h,
		// 	B:   b,
		// 	I:   cellI,
		// 	II:  chunkIndex,
		// }, hexStr)
		// if err != nil {
		// 	return tapeI, cellI, false, err
		// }

		// cell = []Cell{*item}
		cellI = 1
	} else if *splitter == IncludeR {
		outTapes := append(x.Tape, Tape{Cell: cell, I: tapeI})
		x.Tape = outTapes
		//tapeI++
		item, err := t(Cell{
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

		//cell = make([]Cell, 0)
		cellI = 0
	}

	return tapeI, cellI, true, nil

}
