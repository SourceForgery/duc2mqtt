package bastec

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rotisserie/eris"
)

type Point struct {
	Pid           string  `json:"pid"`
	Value         float64 `json:"value"`
	Decimals      int     `json:"decimals"`
	DecimalsShown int     `json:"decimals_shown"`
}

type ValuesResponse struct {
	JsonRpc string `json:"json-rpc"`
	Result  struct {
		Timet  int64   `json:"timet"`
		Times  string  `json:"times"`
		Points []Point `json:"points"`
	} `json:"result"`
	Error string `json:"error"`
	Id    int    `json:"id"`
}

func (bastecClient *BastecClient) GetValues(values []string) (response *ValuesResponse, err error) {

	params := [][]string{values}

	var rpcRequest = JsonRpcRequest{
		JsonRpcVersion: "2.0",
		Method:         "pdb.getvalue",
		Params:         params,
	}

	jsonResponse, err := bastecClient.jsonRpc(rpcRequest)
	if err != nil {
		return nil, eris.Wrapf(err, "failed GetValues jsonRpc request")
	}
	logger().Debug(string(jsonResponse))
	err = json.Unmarshal(jsonResponse, &response)
	if err != nil {
		return
	}
	if response.Error != "" {
		err = errors.New(fmt.Sprintf("getValues error: %s", response.Error))
	}
	return
}
