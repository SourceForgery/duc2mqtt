package mqtt

import (
	"encoding/json"
	"errors"
	"fmt"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rotisserie/eris"
	"github.com/sirupsen/logrus"
	"net/url"
)

type MqttClient struct {
	client   MQTT.Client
	uniqueId string
	prefix   string
	Discover []Sensors
}

func onConnectionLost(client MQTT.Client, err error) {
	logger().Infof("Connection lost: %v", err)
}

func (client *MqttClient) sendMessage(topic string, payload interface{}) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		logger().Fatalf("Failed to serialize payload to %s", topic)
	}

	token := client.client.Publish(topic, 0, false, payloadBytes)
	token.Wait()
	if token.Error() != nil {
		logger().Errorf("Error publishing to topic %s: %v\n", topic, token.Error())
	} else {
		logger().Debugf("Message published to topic %s\n", topic)
	}
}

func ConnectMqtt(url url.URL, uniqueId string, prefix string) (mqttClient *MqttClient, err error) {
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
	mqttClient = &MqttClient{
		client:   client,
		uniqueId: uniqueId,
		prefix:   prefix,
	}

	return
}

func (mqttClient *MqttClient) SubscribeToHomeAssistantStatus() {
	mqttClient.client.Subscribe("%s/status", 0, func(client MQTT.Client, msg MQTT.Message) {
		if string(msg.Payload()) == "online" {
			mqttClient.Available()
			mqttClient.Discovery
		}
	})
}

// sendSensorData sends sensor data to Home Assistant
func (client *MqttClient) SendSensorData(sensorStates []SensorState) {
	discoveryPayload := DiscoveryMessage{
		Name:                client.uniqueId,
		UniqueID:            client.uniqueId,
		StateTopic:          fmt.Sprintf("homeassistant/sensor/%s/state", client.uniqueId),
		AvailabilityTopic:   fmt.Sprintf("homeassistant/sensor/%s/availability", client.uniqueId),
		PayloadAvailable:    "online",
		PayloadNotAvailable: "offline",
		Device: Device{
			Identifiers:  []string{client.uniqueId},
			Name:         "duc2mqtt",
			Model:        "Golang Model",
			Manufacturer: "SourceForgery AB",
		},
	}

	// Send the discovery message once
	client.sendMessage(fmt.Sprintf("homeassistant/sensor/%s/config", client.uniqueId), discoveryPayload)

	// Send sensor's availability status
	client.sendMessage(fmt.Sprintf("homeassistant/sensor/%s/availability", client.uniqueId), AvailabilityMessage{"online"})

	// Periodically send sensor data
	for key, value := range sensorStates {
		sensorPayload := map[string]interface{}{key: value}
		client.sendMessage(fmt.Sprintf("homeassistant/sensor/%s/state", client.uniqueId), sensorPayload)
	}
}
