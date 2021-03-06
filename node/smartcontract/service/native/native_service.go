/*
 * Copyright (C) 2018 The ontology Authors
 * This file is part of The ontology library.
 *
 * The ontology is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The ontology is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The ontology.  If not, see <http://www.gnu.org/licenses/>.
 */

package native

import (
	"encoding/hex"
	"fmt"
	"github.com/ontio/ontology-crypto/keypair"
	"github.com/ontio/layer2/node/common/config"
	"github.com/ontio/layer2/node/common/log"
	"github.com/ontio/layer2/node/merkle"

	"github.com/ontio/layer2/node/common"
	"github.com/ontio/layer2/node/core/types"
	"github.com/ontio/layer2/node/errors"
	"github.com/ontio/layer2/node/smartcontract/context"
	"github.com/ontio/layer2/node/smartcontract/event"
	"github.com/ontio/layer2/node/smartcontract/states"
	sstates "github.com/ontio/layer2/node/smartcontract/states"
	"github.com/ontio/layer2/node/smartcontract/storage"
)

type (
	Handler         func(native *NativeService) ([]byte, error)
	RegisterService func(native *NativeService)
)

var (
	Contracts = make(map[common.Address]RegisterService)
)

// Native service struct
// Invoke a native smart contract, new a native service
type NativeService struct {
	CacheDB       *storage.CacheDB
	ServiceMap    map[string]Handler
	Notifications []*event.NotifyEventInfo
	InvokeParam   sstates.ContractInvokeParam
	Input         []byte
	Tx            *types.Transaction
	Height        uint32
	Time          uint32
	BlockHash     common.Uint256
	ContextRef    context.ContextRef
	PreExec       bool
	CrossHashes   []common.Uint256
	Operator      bool
}

func (this *NativeService) Register(methodName string, handler Handler) {
	this.ServiceMap[methodName] = handler
}

func (this *NativeService) Invoke() ([]byte, error) {
	contract := this.InvokeParam
	services, ok := Contracts[contract.Address]
	operatorPublicKeyBytes,_ := hex.DecodeString(config.DefConfig.Genesis.SOLO.Bookkeepers[0])
	operatorPublicKey,_ := keypair.DeserializePublicKey(operatorPublicKeyBytes)
	operatorAddress := types.AddressFromPubKey(operatorPublicKey)
	player := this.Tx.Payer.ToBase58()
	log.Infof("player: %s, operator: %s", player, operatorAddress.ToBase58())
	if player == operatorAddress.ToBase58() {
		this.Operator = true
	}
	if !ok {
		return BYTE_FALSE, fmt.Errorf("Native contract address %x haven't been registered.", contract.Address)
	}
	services(this)
	service, ok := this.ServiceMap[contract.Method]
	if !ok {
		return BYTE_FALSE, fmt.Errorf("Native contract %x doesn't support this function %s.",
			contract.Address, contract.Method)
	}
	args := this.Input
	this.Input = contract.Args
	this.ContextRef.PushContext(&context.Context{ContractAddress: contract.Address})
	notifications := this.Notifications
	this.Notifications = []*event.NotifyEventInfo{}
	hashes := this.CrossHashes
	this.CrossHashes = []common.Uint256{}
	result, err := service(this)
	if err != nil {
		return result, errors.NewDetailErr(err, errors.ErrNoCode, "[Invoke] Native serivce function execute error!")
	}
	this.ContextRef.PopContext()
	this.ContextRef.PushNotifications(this.Notifications)
	this.Notifications = notifications
	this.Input = args
	this.CrossHashes = hashes
	return result, nil
}

func (this *NativeService) NativeCall(address common.Address, method string, args []byte) ([]byte, error) {
	c := states.ContractInvokeParam{
		Address: address,
		Method:  method,
		Args:    args,
	}
	this.InvokeParam = c
	return this.Invoke()
}

func (this *NativeService) PushCrossState(data []byte) {
	this.CrossHashes = append(this.CrossHashes, merkle.HashLeaf(data))
}
