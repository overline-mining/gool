package rovers

import (
	"context"
	pb "github.com/overline-mining/gool/src/protos"
	"sync"
)

type RoverServer struct {
	pb.UnimplementedRoverServer

	lock sync.Mutex

	latestBlocks map[string]*pb.Block
}

func (s *RoverServer) Join(*pb.RoverIdent, pb.Rover_JoinServer) error {
	return nil
}

func (s *RoverServer) CollectBlock(pb.Rover_CollectBlockServer) error {
	return nil
}

func (s *RoverServer) ReportSyncStatus(*context.Context, *pb.RoverSyncStatus) (*pb.Null, error) {
	return nil, nil
}

func (s *RoverServer) ReportBlockRange(*context.Context, *pb.RoverMessage_RoverBlockRange) (*pb.Null, error) {
	return nil, nil
}

func (s *RoverServer) IsBeforeSettleHeight(*context.Context, *pb.SettleTxCheckReq) (*pb.SettleTxCheckResponse, error) {
	return nil, nil
}
