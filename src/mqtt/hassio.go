package mqtt

import (
	"fmt"
	"github.com/sirupsen/logrus"
)

func logger() *logrus.Entry {
	return logrus.WithField("logger", "mqtt")
}

// AvailabilityMessage represents the availability payload for Home Assistant.
type AvailabilityMessage struct {
	Payload string `json:"payload"`
}

// DiscoveryMessage represents the discovery payload to be sent to Home Assistant.
type DiscoveryMessage struct {
	Name                string `json:"name"`
	UniqueID            string `json:"unique_id"`
	StateTopic          string `json:"state_topic"`
	AvailabilityTopic   string `json:"availability_topic"`
	PayloadAvailable    string `json:"payload_available"`
	PayloadNotAvailable string `json:"payload_not_available"`
	Device              Device `json:"device"`
}

// Device represents the device information for Home Assistant.
type Device struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Model        string   `json:"model"`
	Manufacturer string   `json:"manufacturer"`
}

func (client *MqttClient) avail() {
	availabilityPayload := AvailabilityMessage{"online"}
	client.sendMessage("homeassistant/sensor/sensor_12345/availability", availabilityPayload)
}

func (client *MqttClient) sendSensorData(sensorID string, data map[string]interface{}) {
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

	client.sendMessage(fmt.Sprintf("homeassistant/sensor/%s/config", sensorID), discoveryPayload)
	client.sendMessage(fmt.Sprintf("homeassistant/sensor/%s/availability", sensorID), AvailabilityMessage{"online"})

	for key, value := range data {
		sensorPayload := map[string]interface{}{key: value}
		client.sendMessage(fmt.Sprintf("homeassistant/sensor/%s/state", sensorID), sensorPayload)
	}
}
