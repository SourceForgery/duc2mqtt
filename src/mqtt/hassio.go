package mqtt

import (
	"fmt"
	"github.com/sirupsen/logrus"
)

func logger() *logrus.Entry {
	return logrus.WithField("logger", "mqtt")
}

// AvailabilityMessage represents the MQTT availability message
type AvailabilityMessage struct {
	DeviceID string `json:"device_id"`
	Status   string `json:"status"`
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

type SensorState struct {
	State string `json:"state"`
}

func (client *MqttClient) Available() {
	client.sendMessage(fmt.Sprintf("%s/sensor/%s/availability", client.prefix, client.uniqueId), "online")
}

//func (client *MqttClient) SendSensorData(sensorID string, data SensorState) {
//	discoveryPayload := DiscoveryMessage{
//		Name:                sensorID,
//		UniqueID:            sensorID,
//		StateTopic:          fmt.Sprintf("homeassistant/sensor/%s/state", sensorID),
//		AvailabilityTopic:   fmt.Sprintf("homeassistant/sensor/%s/availability", sensorID),
//		PayloadAvailable:    "online",
//		PayloadNotAvailable: "offline",
//		Device: Device{
//			Identifiers:  []string{sensorID},
//			Name:         "Golang MQTT Sensor",
//			Model:        "Golang Model",
//			Manufacturer: "Golang Manufacturer",
//		},
//	}
//
//	client.sendMessage(fmt.Sprintf("homeassistant/sensor/%s/config", sensorID), discoveryPayload)
//	client.sendMessage(fmt.Sprintf("homeassistant/sensor/%s/availability", sensorID), AvailabilityMessage{"online"})
//
//	for key, value := range data {
//		sensorPayload :=
//			client.sendMessage(fmt.Sprintf("homeassistant/sensor/%s/state", sensorID), sensorPayload)
//	}
//}
