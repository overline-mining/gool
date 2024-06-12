package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/go-zeromq/zmq4"
	"go.uber.org/zap"
	"os"
	"time"

	"github.com/overline-mining/gool/src/common"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	ovl_pb "github.com/overline-mining/gool/src/protos"
)

type socket struct {
	zmq     zmq4.Socket
	theType zmq4.SocketType
}

type BtcRover struct {
	client    ovl_pb.RoverClient
	msgStream *ovl_pb.Rover_JoinClient
}

func (c *BtcRover) Join() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	joinMsg := &ovl_pb.RoverIdent{RoverName: "BTC"}

	msgStream, err := c.client.Join(ctx, joinMsg)
	if err != nil {
		zap.S().Fatal("BTC Rover failed to connect to node! (%v)", err)
	}
	c.msgStream = &msgStream
}

func (c *BtcRover) CollectBlockSubscription() {
	sck := socket{zmq: zmq4.NewSub(context.Background()), theType: zmq4.Sub}

	rpcConfig := &rpcclient.ConnConfig{
		Host:         "192.168.1.170:8332",
		User:         "gool",
		Pass:         "gool",
		HTTPPostMode: true,
		DisableTLS:   true,
	}
	rpcClient, err := rpcclient.New(rpcConfig, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer rpcClient.Shutdown()

	blockCount, err := rpcClient.GetBlockCount()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("blockcount -> %v\n", blockCount)

	fmt.Println("attaching to port 29000")
	sck.zmq.Dial("tcp://192.168.1.170:29000")
	fmt.Println("attached to port 29000")

	sck.zmq.SetOption(zmq4.OptionSubscribe, "rawblock")

	for {
		msg, err := sck.zmq.Recv()
		msgBytes := msg.Bytes()
		fmt.Printf("Received messaged of length %v, with error %v\n", len(msgBytes), err)
		if err == nil {
			blockBytes := msgBytes[8 : len(msgBytes)-4] // 'rawblock' | blockbytes | int32
			blockCount, err := rpcClient.GetBlockCount()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			block, err := btcutil.NewBlockFromBytes(blockBytes)
			if err == nil {
				block.SetHeight(int32(blockCount))
				// confirm the block height for blocks after BIP 34, this is the case for all blocks in modern era
				coinbaseTx := block.Transactions()[0].MsgTx()
				coinbaseBytes := make([]byte, 0, coinbaseTx.SerializeSize())
				cbBuf := bytes.NewBuffer(coinbaseBytes)
				coinbaseTx.Serialize(cbBuf)
				heightBytesLength := cbBuf.Bytes()[44]
				coinbaseHeightBytes := make([]byte, 4)
				copy(coinbaseHeightBytes, cbBuf.Bytes()[45:heightBytesLength+45])
				heightFromBIP34 := binary.LittleEndian.Uint32(coinbaseHeightBytes)
				if heightFromBIP34 != uint32(blockCount) {
					fmt.Printf("Notification blockheight from BIP34 does not match node blockheight, %v != %v", heightFromBIP34, uint32(blockCount))
					os.Exit(1)
				} else {
					block.SetHeight(int32(heightFromBIP34))
				}
				// get the block header and extract the timestamp
				var header wire.BlockHeader
				err := header.Deserialize(bytes.NewReader(blockBytes[:80]))
				if err != nil {
					fmt.Printf("Could not deserialized header for %v", block.Hash())
					os.Exit(1)
				}

				fmt.Printf("%v %v %v %v\n", len(blockBytes), block.Height(), block.Hash(), len(block.Transactions()))
				outBlock := &ovl_pb.Block{
					Blockchain:   "BTC",
					Hash:         block.Hash().String(),
					PreviousHash: header.PrevBlock.String(),
					Timestamp:    uint64(header.Timestamp.Unix()),
					Height:       uint64(block.Height()),
					MerkleRoot:   header.MerkleRoot.String(),
					//Difficulty: header.Bits,
					//Nonce: header.Nonce,
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				stream, err := c.client.CollectBlock(ctx)
				if err != nil {
					zap.S().Fatalf("%v.RecordRoute(_) = _, %v", c.client, err)
				}
				err = stream.Send(outBlock)
				if err != nil {
					zap.S().Fatalf("%v.Send(%v) = %v", stream, outBlock, err)
				}
				_, err = stream.CloseAndRecv()
				if err != nil {
					zap.S().Fatalf("%v.CloseAndRecv() got error %v, want %v", stream, err, nil)
				}
			} else {
				fmt.Println(err)
			}

		}
	}

}

var (
	logLevel = flag.String("log-level", "INFO", "Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
)

func main() {

	common.SetupLogger(*logLevel)

	zap.S().Infof("starting BTC rover!")

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.Dial("localhost:9424", opts...)
	if err != nil {
		zap.S().Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	zap.S().Info("created grpc connection, making rover")

	rover := BtcRover{client: ovl_pb.NewRoverClient(conn)}

	zap.S().Info("rover created")

	rover.Join()

	zap.S().Info("rover joined")

	go rover.CollectBlockSubscription()

	for {
		time.Sleep(1 * time.Second)
	}
}
