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
	client MQTT.Client
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

func ConnectMqtt(url url.URL) (mqttClient *MqttClient, err error) {
	userInfo := *url.User
	var password string
	var hasPassword bool
	if userInfo != nil {
		return nil, errors.New("mqtt url needs to have username and password")
	}
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
		SetOnConnectHandler(onConnect)

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
		client: client,
	}
	return
}

// sendSensorData sends sensor data to Home Assistant
func sendSensorData(client MQTT.Client, sensorID string, data map[string]interface{}) {
	discoveryPayload := DiscoveryMessage{
		Name:                sensorID,
		UniqueID:            sensorID,
		StateTopic:          fmt.Sprintf("homeassistant/sensor/%s/state", sensorID),
		AvailabilityTopic:   fmt.Sprintf("homeassistant/sensor/%s/availability", sensorID),
		PayloadAvailable:    "online",
		PayloadNotAvailable: "offline",
		Device: Device{
			Identifiers:  []string{sensorID},
			Name:         "Golang MQTT Sensor",
			Model:        "Golang Model",
			Manufacturer: "Golang Manufacturer",
		},
	}

	// Send the discovery message once
	sendMessage(client, fmt.Sprintf("homeassistant/sensor/%s/config", sensorID), discoveryPayload)

	// Send sensor's availability status
	sendMessage(client, fmt.Sprintf("homeassistant/sensor/%s/availability", sensorID), AvailabilityMessage{"online"})

	// Periodically send sensor data
	for key, value := range data {
		sensorPayload := map[string]interface{}{key: value}
		sendMessage(client, fmt.Sprintf("homeassistant/sensor/%s/state", sensorID), sensorPayload)
	}
}
