package rover

import (
	"context"
	"errors"
	"fmt"
	"io"

	"sync"

	ovl_pb "github.com/overline-mining/gool/src/protos"
	"go.uber.org/zap"
)

func NewServer() *Server {
	out := new(Server)
	out.incomingBlocks = make(chan *ovl_pb.Block, 10)
	out.requestLock.Lock()
	out.roverRequests = make(map[string]chan *ovl_pb.RoverMessage)
	out.requestLock.Unlock()
	out.syncLock.Lock()
	out.syncStatus = make(map[string]bool)
	out.syncLock.Unlock()
	return out
}

type Server struct {
	ovl_pb.UnimplementedRoverServer

	requestLock, blockLock, syncLock sync.Mutex

	incomingBlocks chan *ovl_pb.Block
	roverRequests  map[string]chan *ovl_pb.RoverMessage

	syncStatus map[string]bool
}

func (s *Server) Join(joinReq *ovl_pb.RoverIdent, stream ovl_pb.Rover_JoinServer) error {
	name := joinReq.GetRoverName()
	var requests chan *ovl_pb.RoverMessage
	s.requestLock.Lock()
	if _, ok := s.roverRequests[name]; !ok {
		requests = make(chan *ovl_pb.RoverMessage, 10)
		s.roverRequests[name] = requests
	} else {
		zap.S().Errorf("Rover %v is already in list of joined rovers, dropping join request!", name)
	}
	s.requestLock.Unlock()
	s.syncLock.Lock()
	s.syncStatus[name] = false
	s.syncLock.Unlock()

	for {
		oneRequest, more := <-requests
		zap.S().Infof("Got rover request: %v", oneRequest)
		if err := stream.Send(oneRequest); err != nil {
			zap.S().Error(err)
			close(requests)
			return err
		}
		if more {
			zap.S().Warnf("Rover message request channel for %v has been closed!", name)
			return nil
		}
	}
	return nil
}

func (s *Server) CollectBlock(stream ovl_pb.Rover_CollectBlockServer) error {
	for {
		block, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(new(ovl_pb.Null))
		}
		if err != nil {
			return err
		}
		s.incomingBlocks <- block
	}
	return nil
}

func (s *Server) ReportSyncStatus(ctx *context.Context, status *ovl_pb.RoverSyncStatus) (*ovl_pb.Null, error) {
	name := status.GetRoverName()
	if _, ok := s.syncStatus[name]; ok {
		s.syncLock.Lock()
		s.syncStatus[name] = status.GetStatus()
		s.syncLock.Unlock()
	} else {
		zap.S().Errorf("Unknown rover: %v", name)
		return nil, errors.New(fmt.Sprintf("Unknown rover: %v", name))
	}
	return &ovl_pb.Null{}, nil
}

func (s *Server) ReportBlockRange(*context.Context, *ovl_pb.RoverMessage_RoverBlockRange) (*ovl_pb.Null, error) {
	return nil, nil
}

func (s *Server) IsBeforeSettleHeight(*context.Context, *ovl_pb.SettleTxCheckReq) (*ovl_pb.SettleTxCheckResponse, error) {
	return nil, nil
}

func (s *Server) PushRoverRequest(rover string, req *ovl_pb.RoverMessage) {
	s.requestLock.Lock()
	if reqs, ok := s.roverRequests[rover]; ok {
		reqs <- req
	}
	s.requestLock.Unlock()
}
