package transactions

import (
	b64 "encoding/base64"
	"fmt"
	"github.com/overline-mining/gool/src/olhash"
	p2p_pb "github.com/overline-mining/gool/src/protos"
	"strings"
)

func BytesToJSFormatString(input []byte) string {
	asJSString := []string{}
	for _, byte := range input {
		asJSString = append(asJSString, fmt.Sprintf("%d", byte))
	}
	return strings.Join(asJSString, ",")
}

func TxHash(tx *p2p_pb.Transaction) string {
	toHash := fmt.Sprintf(
		"%d%s%s%d%d%d",
		tx.GetVersion(),
		tx.GetNonce(),
		tx.GetOverline(),
		tx.GetNinCount(),
		tx.GetNoutCount(),
		tx.GetLockTime(),
	)
	for _, input := range tx.GetInputs() {
		outPoint := input.GetOutPoint()
		toHash += fmt.Sprintf(
			"%s%s%d%d%s",
			b64.StdEncoding.EncodeToString(outPoint.GetValue()),
			outPoint.GetHash(),
			outPoint.GetIndex(),
			input.GetScriptLength(),
			b64.StdEncoding.EncodeToString(input.GetInputScript()),
		)
	}
	for _, output := range tx.GetOutputs() {
		toHash += fmt.Sprintf(
			"%s%s%d%s",
			b64.StdEncoding.EncodeToString(output.GetValue()),
			b64.StdEncoding.EncodeToString(output.GetUnit()),
			output.GetScriptLength(),
			b64.StdEncoding.EncodeToString(output.GetOutputScript()),
		)
	}
	return olhash.Blake2bl(olhash.Blake2bl(toHash))
}
