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
	"net/url"
	"strings"
)

func logger() *logrus.Entry {
	return logrus.WithField("logger", "bastec")
}

type Salts struct {
	SaltA []byte `json:"salt_a"`
	SaltB []byte `json:"salt_b"`
}

//goland:noinspection GoNameStartsWithPackageName
type BastecClient struct {
	sessionId  string
	RequestURL url.URL
}

type Session struct {
	Name    string `json:"name"`
	UserId  string `json:"userid"`
	Company string `json:"company"`
	City    string `json:"city"`
}

type BrowseResponse struct {
	JsonRpc string `json:"json-rpc"`
	Result  struct {
		DevId  string `json:"devid"`
		Points []struct {
			Pid           string `json:"pid"`
			Desc          string `json:"desc"`
			Acc           string `json:"acc"`
			Type          string `json:"type"`
			DecimalsShown string `json:"decimals_shown,omitempty"`
			Decimals      int    `json:"decimals,omitempty"`
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

func Connect(url url.URL) (bastecClient *BastecClient, err error) {
	if url.Path != "" {
		return nil, errors.New("invalid url path. It must be empty")
	}
	if url.User == nil {
		return nil, errors.New("missing user & password in url")
	}
	if _, hasPassword := url.User.Password(); !hasPassword {
		return nil, errors.New("missing password in url")
	}
	user := url.User.Username()
	password, _ := url.User.Password()
	requesterURL := *url.JoinPath("if/login.js")
	query := requesterURL.Query()
	query.Add("username", user)
	requesterURL.RawQuery = query.Encode()
	requesterURL.User = nil
	logger().Debugf("connecting to bastec '%s'", requesterURL.String())
	res, err := http.Get(requesterURL.String())
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

	hash := generateBastecHash(password, saltResponse)

	loginUrl := requesterURL
	x := requesterURL.Query()
	x.Add("hash", hash)
	loginUrl.RawQuery = x.Encode()
	loginResponse, err := http.Get(loginUrl.String())
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

	var sessionId string
	var cookies = loginResponse.Cookies()
	for i := 0; i < len(cookies); i++ {
		var cookie = cookies[i]
		if cookie.Name == "SESSION_ID" {
			sessionId = cookie.Value
			break
		}
	}

	rpcURL := requesterURL
	rpcURL.RawQuery = ""

	logger().Infof("Connected to bastec duc '%s'", requesterURL.String())

	bastecClient = &BastecClient{
		sessionId:  sessionId,
		RequestURL: rpcURL,
	}
	return
}

func (bastecClient *BastecClient) GetVersion() (response []byte, err error) {
	var rpcRequest = JsonRpcRequest{
		JsonRpcVersion: "2.0",
		Method:         "pdb.version",
		Id:             1,
	}

	response, err = bastecClient.jsonRpc(rpcRequest)
	if err != nil {
		return nil, eris.Wrap(err, "failed to execute GetVersion jsonRPC")
	}
	return

}

func (bastecClient *BastecClient) Browse() (valueResponse *BrowseResponse, err error) {
	var rpcRequest = JsonRpcRequest{
		JsonRpcVersion: "2.0",
		Method:         "pdb.browse",
		Id:             2,
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
		logger().Debugf("Found sensor '%s' on device '%s' with ", browseResponse.Result.DevId, point.Pid)
	}
	return &browseResponse, nil
}

func (bastecClient *BastecClient) GetValues(values []string) (response *ValuesResponse, err error) {

	params := [][]string{values}

	var rpcRequest = JsonRpcRequest{
		JsonRpcVersion: "2.0",
		Method:         "pdb.getvalue",
		Params:         params,
		Id:             3,
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
	return
}

func (bastecClient *BastecClient) jsonRpc(request JsonRpcRequest) (body []byte, err error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		logger().WithError(err).Fatal(eris.Wrapf(err, "failed to create json request"))
	}
	log.Println(string(jsonBody))
	reader := bytes.NewReader(jsonBody)
	requestUrl := bastecClient.RequestURL.String()
	req, err := http.NewRequest(http.MethodPost, requestUrl, reader)
	if err != nil {
		logger().WithError(eris.Wrapf(err, "failed to create new request")).Error("failed to create new request")
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Cookie", fmt.Sprintf("SESSION_ID=%s", bastecClient.sessionId))

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
