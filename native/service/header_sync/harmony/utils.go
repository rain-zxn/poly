/*
 * Copyright (C) 2022 The poly network Authors
 * This file is part of The poly network library.
 *
 * The  poly network  is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The  poly network  is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 * You should have received a copy of the GNU Lesser General Public License
 * along with The poly network .  If not, see <http://www.gnu.org/licenses/>.
 */

package harmony

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
)

func verifyHeaderSigs(epoch *Epoch, header *HeaderWithSig) (err error) {
	pubKeys, err := epoch.Committee.BLSPublicKeys()
	if err != nil {
		return
	}

	sigBytes := bls.SerializedSignature{}
	copy(sigBytes[:], header.Sig)
	aggSig, mask, err := DecodeSigBitmap(sigBytes, []byte(header.Bitmap), pubKeys)
	if err != nil {
		return
	}

	isStaking := IsStaking(epoch.EpochID)
	qrVerifier, err := quorum.NewVerifier(epoch.Committee, big.NewInt(int64(epoch.EpochID)), isStaking)
	if err != nil {
		return
	}
	if !qrVerifier.IsQuorumAchievedByMask(mask) {
		return errors.New("not enough signature collected")
	}

	commitPayload := ConstructCommitPayload(
		isStaking, header.Header.Hash(), header.Header.Number().Uint64(), header.Header.ViewID().Uint64(),
	)
	if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
		return errors.New("Unable to verify aggregated signature for block")
	}

	return
}

// DecodeSigBitmap decode and parse the signature, bitmap with the given public keys
func DecodeSigBitmap(sigBytes bls.SerializedSignature, bitmap []byte, pubKeys []bls.PublicKeyWrapper) (*bls_core.Sign, *bls.Mask, error) {
	aggSig := bls_core.Sign{}
	err := aggSig.Deserialize(sigBytes[:])
	if err != nil {
		return nil, nil, errors.New("unable to deserialize multi-signature from payload")
	}
	mask, err := bls.NewMask(pubKeys, nil)
	if err != nil {
		return nil, nil, errors.New("unable to setup mask from payload")
	}
	if err := mask.SetMask(bitmap); err != nil {
		return nil, nil, errors.New("mask.SetMask failed")
	}
	return &aggSig, mask, nil
}

// ConstructCommitPayload returns the commit payload for consensus signatures.
func ConstructCommitPayload(
	isStaking bool, blockHash common.Hash, blockNum, viewID uint64,
) []byte {
	blockNumBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumBytes, blockNum)
	commitPayload := append(blockNumBytes, blockHash.Bytes()...)
	if !isStaking {
		return commitPayload
	}
	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDBytes, viewID)
	return append(commitPayload, viewIDBytes...)
}