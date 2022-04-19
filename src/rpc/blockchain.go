package rpc

import (
	//"encoding/hex"
	"errors"
	"github.com/overline-mining/gool/src/blockchain"
	p2p_pb "github.com/overline-mining/gool/src/protos"
	//"go.uber.org/zap"
)

type BlockchainService struct {
	Chain *blockchain.OverlineBlockchain
}

func (s *BlockchainService) GetBlockByHash(hash string) (p2p_pb.BcBlock, error) {
	block, err := s.Chain.GetBlockByHash(hash)
	if err != nil {
		block = new(p2p_pb.BcBlock)
	}
	return *block, err
}

func (s *BlockchainService) GetBlockByHeight(height uint64) (p2p_pb.BcBlock, error) {
	block, err := s.Chain.GetBlockByHeight(height)
	if err != nil {
		block = new(p2p_pb.BcBlock)
	}
	return *block, err
}

func (s *BlockchainService) Div(a, b int) (int, error) {
	if b == 0 {
		return 0, errors.New("divide by zero")
	}
	return a / b, nil
}
