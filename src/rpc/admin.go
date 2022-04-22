package rpc

import (
	"fmt"
	"github.com/overline-mining/gool/src/common"
	"runtime"
	//"encoding/hex"
	//"errors"
	//"github.com/overline-mining/gool/src/blockchain"
	//p2p_pb "github.com/overline-mining/gool/src/protos"
	//"go.uber.org/zap"
)

type AdminService struct {
}

func (s *AdminService) NodeInfo() (interface{}, error) {
	return map[string]interface{}{
		"version": fmt.Sprintf("%v/%v/%v/%v", common.GetVersion(), runtime.GOOS, runtime.GOARCH, runtime.Version()),
	}, nil
}
