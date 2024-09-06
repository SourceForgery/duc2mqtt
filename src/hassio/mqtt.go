package hassio

import (
	"encoding/json"
	"errors"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rotisserie/eris"
	"github.com/sirupsen/logrus"
	"net/url"
)

type Client struct {
	client                  MQTT.Client
	Device                  *Device
	uniqueDeviceId          string // optional. Duc's is used if not set
	SensorConfigurationData map[string]SensorConfig
	prefix                  string
}

func onConnectionLost(_ MQTT.Client, err error) {
	logger().Infof("Connection lost: %v", err)
}

func (hassioClient *Client) sendMessage(topic string, payload interface{}) (err error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		logger().Fatalf("Failed to serialize payload to %s", topic)
	}

	token := hassioClient.client.Publish(topic, 0, false, payloadBytes)
	token.Wait()
	if token.Error() != nil {
		return eris.Wrapf(token.Error(), "Error publishing to topic %s\n", topic)
	} else {
		logger().Debugf("Message published to topic %s\n", topic)
	}
	return nil
}

func ConnectMqtt(url url.URL, uniqueId string, prefix string) (hassioClient *Client, err error) {
	var password string
	var hasPassword bool
	if url.User == nil {
		return nil, errors.New("mqtt url needs to have username and password")
	}
	userInfo := *url.User
	if password, hasPassword = userInfo.Password(); !hasPassword {
		return nil, errors.New("mqtt url needs to have username and password")
	}

	url.User = nil

	hassioClient = &Client{
		uniqueDeviceId: uniqueId,
		prefix:         prefix,
	}

	var onConnect MQTT.OnConnectHandler = func(_ MQTT.Client) {
		logger().Infof("MQTT connection established")
		if hassioClient.Device != nil {
			hassioClient.SendLastWill()
			hassioClient.SendAvailability()
		}
	}
	opts := MQTT.NewClientOptions().AddBroker(url.String()).
		SetClientID("duc2mqtt").
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectionLostHandler(onConnectionLost).
		SetOnConnectHandler(onConnect).
		SetPassword(password).
		SetUsername(userInfo.Username())

	if logrus.GetLevel() >= logrus.DebugLevel {
		var messagePubHandler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
			logger().Debug("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
		}
		opts.DefaultPublishHandler = messagePubHandler
	}

	// Create and start the client using the above options
	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, eris.Wrapf(token.Error(), "failed to connect to %s", url.String())
	}
	hassioClient.client = client

	return
}