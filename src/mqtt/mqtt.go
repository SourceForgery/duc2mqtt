package mqtt

import (
	"errors"
	"fmt"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rotisserie/eris"
	"net/url"
)

type MqttClient struct {
	client *MQTT.Client
}

func onConnectionLost(client MQTT.Client, err error) {
	fmt.Println("Connection lost:", err)
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
	opts := MQTT.NewClientOptions().AddBroker(url.String()).
		SetClientID("duc2mqtt").
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectionLostHandler(onConnectionLost)

	// Define the message handler
	var messagePubHandler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
	}

	// Assign the message handler
	opts.SetDefaultPublishHandler(messagePubHandler)

	// Create and start the client using the above options
	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, eris.Wrapf(token.Error(), "failed to connect to %s", url.String())
	}
	mqttClient = &MqttClient{
		client: client,
	}
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
