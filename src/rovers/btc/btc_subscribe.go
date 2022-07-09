package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/go-zeromq/zmq4"
	"os"
)

type socket struct {
	zmq     zmq4.Socket
	theType zmq4.SocketType
}

func main() {
	sck := socket{zmq: zmq4.NewSub(context.Background()), theType: zmq4.Sub}

	rpcConfig := &rpcclient.ConnConfig{
		Host:         "localhost:8332",
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
	sck.zmq.Dial("tcp://localhost:29000")
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
				fmt.Printf("%v %v %v %v\n", len(blockBytes), block.Height(), block.Hash(), len(block.Transactions()))
			} else {
				fmt.Println(err)
			}

		}
	}

}
