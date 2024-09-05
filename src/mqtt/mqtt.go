package mqtt

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
	uniqueDeviceId          string
	SensorConfigurationData map[string]DiscoveryMessage
	prefix                  string
}

func onConnectionLost(client MQTT.Client, err error) {
	logger().Infof("Connection lost: %v", err)
}

func (mqttClient *Client) sendMessage(topic string, payload interface{}) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		logger().Fatalf("Failed to serialize payload to %s", topic)
	}

	token := mqttClient.client.Publish(topic, 0, false, payloadBytes)
	token.Wait()
	if token.Error() != nil {
		logger().Errorf("Error publishing to topic %s: %v\n", topic, token.Error())
	} else {
		logger().Debugf("Message published to topic %s\n", topic)
	}
}

func ConnectMqtt(url url.URL, uniqueId string, prefix string) (mqttClient *Client, err error) {
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

	var onConnect MQTT.OnConnectHandler = func(client MQTT.Client) {
		logger().Infof("MQTT connection established")
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
	mqttClient = &Client{
		client:         client,
		uniqueDeviceId: uniqueId,
		prefix:         prefix,
	}

	return
}
