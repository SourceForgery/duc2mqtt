package bastec

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rotisserie/eris"
	"github.com/sirupsen/logrus"
)

type PointConfig struct {
	Pid  string `json:"pid"`
	Desc string `json:"desc"`
	Acc  string `json:"acc"`
	Type string `json:"type"`
	Attr string `json:"attr,omitempty"`
}

type BrowseResponse struct {
	JsonRpc string `json:"json-rpc"`
	Result  struct {
		DevId  string        `json:"devid"`
		Points []PointConfig `json:"points"`
	} `json:"result"`
	Error string `json:"error"`
	Id    int    `json:"id"`
}

func (bastecClient *BastecClient) Browse() (valueResponse *BrowseResponse, err error) {
	var rpcRequest = JsonRpcRequest{
		JsonRpcVersion: "2.0",
		Method:         "pdb.browse",
	}

	response, err := bastecClient.jsonRpc(rpcRequest)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to execute jsonRPC")
	}

	var browseResponse BrowseResponse
	err = json.Unmarshal(response, &browseResponse)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to parse json")
	}

	if logger().Logger.Level >= logrus.TraceLevel {
		b, _ := json.MarshalIndent(browseResponse, "", "\t")
		logger().Trace(string(b))
	}
	for _, point := range browseResponse.Result.Points {
		logger().Debugf("Found sensor '%s' on device '%s' with ", point.Pid, browseResponse.Result.DevId)
	}
	if browseResponse.Error != "" {
		err = errors.New(fmt.Sprintf("browse error: %s", browseResponse.Error))
	}

	return &browseResponse, nil
}
