package bastec

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rotisserie/eris"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var lg *zerolog.Logger

func logger() *zerolog.Logger {
	if lg == nil {
		l := log.Logger.With().Str("logger", "bastec").Logger()
		lg = &l
	}
	return lg
}

//goland:noinspection GoNameStartsWithPackageName
type BastecClient struct {
	sessionId          string
	RequestURL         url.URL
	serial             int
	DisallowedPrefixes []string
}

type JsonRpcRequest struct {
	JsonRpcVersion string     `json:"json-rpc"`
	Method         string     `json:"method"`
	Params         [][]string `json:"params,omitempty"`
	Id             int        `json:"id"`
}

func Connect(url url.URL) (bastecClient *BastecClient, err error) {
	if url.Path != "" {
		return nil, errors.New("invalid url path. It must be empty")
	}
	if url.User == nil {
		err = errors.New("missing user & password in url")
		return
	}
	if _, hasPassword := url.User.Password(); !hasPassword {
		err = errors.New("missing password in url")
		return
	}
	user := strings.ToUpper(url.User.Username())
	password, _ := url.User.Password()
	requesterURL := *url.JoinPath("if/login.js")
	query := requesterURL.Query()
	query.Add("username", user)
	requesterURL.RawQuery = query.Encode()
	requesterURL.User = nil
	logger().Debug().Msgf("connecting to bastec '%s'", requesterURL.String())
	saltResponse, err := getSalts(requesterURL)
	if err != nil {
		return
	}

	sessionId, err := login(requesterURL, password, saltResponse)
	if err != nil {
		return
	}

	rpcURL := requesterURL
	rpcURL.Path = "if/json_rpc.js"
	rpcURL.RawQuery = ""

	logger().Info().Msgf("Connected to bastec duc '%s'", requesterURL.String())

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
	}

	response, err = bastecClient.jsonRpc(rpcRequest)
	if err != nil {
		return nil, eris.Wrap(err, "failed to execute GetVersion jsonRPC")
	}
	return

}

func (bastecClient *BastecClient) jsonRpc(request JsonRpcRequest) (body []byte, err error) {
	bastecClient.serial++
	request.Id = bastecClient.serial

	jsonBody, err := json.Marshal(request)
	if err != nil {
		logger().Fatal().Err(err).Msg("Failed to create json request")
	}
	reader := bytes.NewReader(jsonBody)
	logger().Trace().Msgf("jsonRpc request body: %s", string(jsonBody))

	requestUrl := bastecClient.RequestURL.String()
	req, err := http.NewRequest(http.MethodPost, requestUrl, reader)
	if err != nil {
		logger().
			Error().
			Err(eris.Wrapf(err, "failed to create new request")).
			Msg("failed to create new request")
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
	responseBody, err := io.ReadAll(res.Body)

	if err != nil {
		err = eris.Wrapf(err, "failed to read response body")
		return
	}
	body = responseBody
	logger().Trace().Msgf("jsonRpc response body: %s", string(body))
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
