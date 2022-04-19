package rpc

import (
	//"encoding/hex"
	"errors"
	"github.com/overline-mining/gool/src/blockchain"
	p2p_pb "github.com/overline-mining/gool/src/protos"
)

type BlockchainService struct {
	Chain *blockchain.OverlineBlockchain
}

func (s *BlockchainService) GetBlockByHash(hash string) (p2p_pb.BcBlock, error) {
	block, err := s.Chain.GetBlockByHash(hash)
	return *block, err
}

func (s *BlockchainService) GetBlockByHeight(height uint64) (p2p_pb.BcBlock, error) {
	block, err := s.Chain.DB.GetBlockByHeight(height)
	return *block, err
}

func (s *BlockchainService) Div(a, b int) (int, error) {
	if b == 0 {
		return 0, errors.New("divide by zero")
	}
	return a / b, nil
}
