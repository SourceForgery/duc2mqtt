package bastec

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rotisserie/eris"
	"github.com/sirupsen/logrus"
	"io"
	"log"
	"net/http"
	"strings"
)

var logger = logrus.WithField("logger", "bastec")

type BastecConfig struct {
	User   string
	Passwd string
	Host   string
}

type Salts struct {
	SaltA []byte `json:"salt_a"`
	SaltB []byte `json:"salt_b"`
}

type BastecClient struct {
	session Session
	host    string
}

type Session struct {
	Name      string `json:"name"`
	UserId    string `json:"userid"`
	Company   string `json:"company"`
	City      string `json:"city"`
	SessionId string
}

type BrowseResponse struct {
	JsonRpc string `json:"json-rpc"`
	Result  struct {
		Devid  string `json:"devid"`
		Points []struct {
			Pid           string `json:"pid"`
			Desc          string `json:"desc"`
			Acc           string `json:"acc"`
			Type          string `json:"type"`
			DecimalsShown string `json:"decimals_shown,omitempty"`
			Decimals      string `json:"decimals,omitempty"`
			Attr          string `json:"attr,omitempty"`
		} `json:"points"`
	} `json:"result"`
	Error string `json:"error"`
	Id    int    `json:"id"`
}

type ValuesResponse struct {
	JsonRpc string `json:"json-rpc"`
	Result  struct {
		Timet  int64  `json:"timet"`
		Times  string `json:"times"`
		Points []struct {
			Pid           string  `json:"pid"`
			Value         float64 `json:"value"`
			Decimals      int     `json:"decimals"`
			DecimalsShown int     `json:"decimals_shown"`
		} `json:"points"`
	} `json:"result"`
	Error interface{} `json:"error"`
	Id    int         `json:"id"`
}

type JsonRpcRequest struct {
	JsonRpcVersion string     `json:"json-rpc"`
	Method         string     `json:"method"`
	Params         [][]string `json:"params"`
	Id             int        `json:"id"`
}

func Connect(config BastecConfig) (bastecClient *BastecClient, err error) {
	requestUrl := fmt.Sprintf("http://%s/if/login.js?username=%s", config.Host, config.User)
	res, err := http.Get(requestUrl)
	if err != nil {
		return
	}

	if res.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("http error code %d", res.StatusCode))
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return
	}

	var saltResponse Salts
	err = json.Unmarshal(body, &saltResponse)
	if err != nil {
		return
	}

	hash := generateBastecHash(config.Passwd, saltResponse)

	loginUrl := fmt.Sprintf("http://%s/if/login.js?username=%s&hash=%s", config.Host, config.User, hash)
	loginResponse, err := http.Get(loginUrl)
	if err != nil {
		return
	}

	if loginResponse.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("http error code %d", res.StatusCode))
	}

	loginBody, err := io.ReadAll(loginResponse.Body)
	if err != nil {
		return
	}

	var session Session
	err = json.Unmarshal(loginBody, &session)
	if err != nil {
		return
	}

	var cookies = loginResponse.Cookies()
	for i := 0; i < len(cookies); i++ {
		var cookie = cookies[i]
		if cookie.Name == "SESSION_ID" {
			session.SessionId = cookie.Value
			break
		}
	}

	bastecClient = &BastecClient{session: session}
	return
}

func (bastecClient *BastecClient) GetVersion() {
	var rpcRequest = JsonRpcRequest{
		JsonRpcVersion: "2.0",
		Method:         "pdb.version",
		Id:             1,
	}

	// SERIALIZING should always work
	response, err := bastecClient.jsonRpc(rpcRequest)
	if err != nil {
		logger.Fatal(eris.Wrap(err, "failed to execute jsonRPC"))
	}
	log.Println(string(response))

}

func (bastecClient *BastecClient) Browse() error {
	var rpcRequest = JsonRpcRequest{
		JsonRpcVersion: "2.0",
		Method:         "pdb.browse",
		Id:             2,
	}

	response, err := bastecClient.jsonRpc(rpcRequest)
	if err != nil {
		return eris.Wrapf(err, "failed to execute jsonRPC")
	}

	var browseResponse BrowseResponse
	err = json.Unmarshal(response, &browseResponse)
	if err != nil {
		return eris.Wrapf(err, "failed to parse json")
	}

	if logger.Level >= logrus.TraceLevel {
		b, _ := json.MarshalIndent(browseResponse, "", "\t")
		logger.Trace(string(b))
	}
	return nil
}

func (bastecClient *BastecClient) GetValues(values []string) (*ValuesResponse, error) {

	params := [][]string{values}

	var rpcRequest = JsonRpcRequest{
		JsonRpcVersion: "2.0",
		Method:         "pdb.getvalue",
		Params:         params,
		Id:             3,
	}

	response, err := bastecClient.jsonRpc(rpcRequest)
	if err != nil {
		return nil, eris.Wrapf(err, "failed jsonRpc request")
	}
	logger.Debug(string(response))
	var valuesResponse ValuesResponse
	err = json.Unmarshal(response, &valuesResponse)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to unmarshal json")
	}
	return &valuesResponse, nil
}

func (bastecClient *BastecClient) jsonRpc(request JsonRpcRequest) (body []byte, err error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		logger.Fatal(eris.Wrapf(err, "failed to create json request"))
	}
	log.Println(string(jsonBody))
	reader := bytes.NewReader(jsonBody)
	requestUrl := fmt.Sprintf("http://%s/if/json_rpc.js", bastecClient.host)
	req, err := http.NewRequest(http.MethodPost, requestUrl, reader)
	if err != nil {
		logger.Fatal(eris.Wrapf(err, "failed to create new request"))
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Cookie", fmt.Sprintf("SESSION_ID=%s", bastecClient.session.SessionId))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("http error code %d", res.StatusCode))
	}
	body, err = io.ReadAll(res.Body)
	return
}

func generateBastecHash(passwd string, salts Salts) string {

	// Calculate the MD5 hash for pass
	md5hash := md5.New()
	md5hash.Write([]byte(passwd))
	md5hash.Write(salts.SaltA)
	temp := md5hash.Sum(nil)

	md5hash.Reset()

	bah := make([]byte, len(temp)+len(salts.SaltB))
	copy(bah, temp)
	copy(bah[len(temp):], salts.SaltB)

	md5hash.Write(bah)
	out := md5hash.Sum(nil)

	// Convert the final MD5 hash to a hex string has to be upper case
	return strings.ToUpper(hex.EncodeToString(out))
}
