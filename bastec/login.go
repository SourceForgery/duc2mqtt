package bastec

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Salts struct {
	SaltA []byte `json:"salt_a"`
	SaltB []byte `json:"salt_b"`
}

type Session struct {
	Name    string `json:"name"`
	UserId  string `json:"userid"`
	Company string `json:"company"`
	City    string `json:"city"`
}

func login(requesterURL url.URL, password string, saltResponse Salts) (sessionId string, err error) {
	hash := generateBastecHash(password, saltResponse)

	loginUrl := requesterURL
	x := requesterURL.Query()
	x.Add("hash", hash)
	loginUrl.RawQuery = x.Encode()
	loginResponse, err := http.Get(loginUrl.String())
	logger().Trace("loginUrl: ", loginUrl.String())
	if err != nil {
		return
	}

	if loginResponse.StatusCode != 200 {
		loginBody, _ := io.ReadAll(loginResponse.Body)
		logger().Debugf("login response (status = %s) failed with body: %s", loginResponse.Status, string(loginBody))
		err = errors.New(fmt.Sprintf("http error code %d", loginResponse.StatusCode))
		return
	}

	loginBody, err := io.ReadAll(loginResponse.Body)
	if err != nil {
		return
	}

	logger().Trace("login response body: ", string(loginBody))

	var session Session
	err = json.Unmarshal(loginBody, &session)
	if err != nil {
		return
	}

	if session.UserId == "" {
		err = errors.New(fmt.Sprintf("login failed, no user found: %s", string(loginBody)))
		return
	}

	var cookies = loginResponse.Cookies()
	for i := 0; i < len(cookies); i++ {
		var cookie = cookies[i]
		if cookie.Name == "SESSION_ID" {
			sessionId = cookie.Value
			break
		}
	}
	if sessionId == "" {
		err = errors.New("login failed: no session id found in cookies")
		return
	}
	return
}

func getSalts(requesterURL url.URL) (saltResponse Salts, err error) {
	res, err := http.Get(requesterURL.String())
	if err != nil {
		return
	}

	if res.StatusCode != 200 {
		loginBody, _ := io.ReadAll(res.Body)
		logger().Debugf("login response (status = %s) failed with body: %s", res.Status, string(loginBody))
		err = errors.New(fmt.Sprintf("http error code %d", res.StatusCode))
		return
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(body, &saltResponse)
	if err != nil {
		return
	}
	return
}
