// Package bpu contains the main functionality of the bpu package
package bpu

import (
	_ "embed" // This was found and leaving for now
	"encoding/hex"
	"errors"
	"fmt"

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
	inputs := make([]Input, 0, len(geneInputs))

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
				var a *bscript.Address
				a, err = bscript.NewAddressFromPublicKeyString(partHex, true)
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
			I: geneInput.PreviousTxOutIndex,
		}
		inputs = append(inputs, Input{
			XPut: inXput,
			Seq:  geneInputs[idx].SequenceNumber,
		})

	}

	return inputs, nil
}

func processOutputs(outXputs []XPut, geneOutputs []*bt.Output) ([]Output, error) {
	outputs := make([]Output, 0, len(geneOutputs))

	for idx, outXput := range outXputs {
		geneOutput := geneOutputs[idx]
		var address *string

		var addresses []string
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

// splits inputs and outputs into tapes wherever
// delimeters are found (defined by parse config)
func collect(config ParseConfig, inputs []*bt.Input, outputs []*bt.Output) (
	xputIns []XPut, xputOuts []XPut, err error) {
	if config.Transform == nil {
		config.Transform = &defaultTransform
	}
	if inputs == nil {
		inputs = make([]*bt.Input, 0)
	}
	if outputs == nil {
		outputs = make([]*bt.Output, 0)
	}

	// preallocate memory
	xputIns = make([]XPut, len(inputs))

	for idx, input := range inputs {
		var xput = new(XPut)
		script := input.UnlockingScript
		err := xput.fromScript(config, script, uint8(idx))
		if err != nil {
			return nil, nil, err
		}
		xputIns[idx] = *xput
	}

	// preallocate memory
	xputOuts = make([]XPut, len(outputs))

	for idx, output := range outputs {
		var xput = new(XPut)
		script := output.LockingScript
		err := xput.fromScript(config, script, uint8(idx))
		if err != nil {
			return nil, nil, err
		}

		xputOuts[idx] = *xput
	}

	return xputIns, xputOuts, nil
}
