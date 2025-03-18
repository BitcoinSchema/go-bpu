// Package bpu contains the main functionality of the bpu package
package bpu

import (
	_ "embed" // This was found and leaving for now
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
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
	var gene *transaction.Transaction
	if config.Tx != nil {
		gene = config.Tx
	} else {
		if config.RawTxHex == nil || len(*config.RawTxHex) == 0 {
			return errors.New("raw tx must be set")

		}
		gene, err = transaction.NewTransactionFromHex(*config.RawTxHex)
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
		H: txid.String(),
	}
	b.In = inputs
	b.Out = outputs
	b.Lock = gene.LockTime

	return
}

func processInputs(inXputs []XPut, geneInputs []*transaction.TransactionInput) ([]Input, error) {
	inputs := make([]Input, 0, len(geneInputs))

	for idx, inXput := range inXputs {
		geneInput := geneInputs[idx]
		var address *string
		if geneInput.UnlockingScript != nil {
			gInScript := *geneInput.UnlockingScript

			// TODO: Remove this hack if libsv accepts this pr:
			// https://github.com/libsv/go-bt/pull/133
			// only a problem for input scripts

			parts, err := script.DecodeScript(gInScript)
			if err != nil {
				return nil, err
			}

			if len(parts) == 2 || (len(parts) >= 2 && len(parts[1].Data) == 33) {
				partHex := hex.EncodeToString(parts[1].Data)
				var a *script.Address
				a, err = script.NewAddressFromPublicKeyString(partHex, true)
				if err != nil {
					return nil, err
				}
				address = &a.AddressString
			}
		}
		prevTxid := geneInput.SourceTXID.String()
		inXput.E = E{
			A: address,
			V: geneInput.SourceTxSatoshis(),
			H: &prevTxid,
			I: geneInput.SourceTxOutIndex,
		}
		inputs = append(inputs, Input{
			XPut: inXput,
			Seq:  geneInputs[idx].SequenceNumber,
		})

	}

	return inputs, nil
}

func processOutputs(outXputs []XPut, geneOutputs []*transaction.TransactionOutput) ([]Output, error) {
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
func collect(config ParseConfig, inputs []*transaction.TransactionInput, outputs []*transaction.TransactionOutput) (
	xputIns []XPut, xputOuts []XPut, err error) {
	if config.Transform == nil {
		config.Transform = &defaultTransform
	}
	if inputs == nil {
		inputs = make([]*transaction.TransactionInput, 0)
	}
	if outputs == nil {
		outputs = make([]*transaction.TransactionOutput, 0)
	}

	// preallocate memory
	xputIns = make([]XPut, len(inputs))

	for idx, input := range inputs {
		var xput = new(XPut)
		s := input.UnlockingScript
		err := xput.fromScript(config, s, uint8(idx))
		if err != nil {
			return nil, nil, err
		}
		xputIns[idx] = *xput
	}

	// preallocate memory
	xputOuts = make([]XPut, len(outputs))

	for idx, output := range outputs {
		var xput = new(XPut)
		s := output.LockingScript
		err := xput.fromScript(config, s, uint8(idx))
		if err != nil {
			return nil, nil, err
		}

		xputOuts[idx] = *xput
	}

	return xputIns, xputOuts, nil
}
