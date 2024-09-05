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
	Name                string  `json:"name"`
	UniqueID            string  `json:"unique_id"`
	StateTopic          string  `json:"state_topic"`
	AvailabilityTopic   string  `json:"availability_topic"`
	PayloadAvailable    string  `json:"payload_available"`
	PayloadNotAvailable string  `json:"payload_not_available"`
	Device              *Device `json:"device"`
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

func (mqttClient *Client) SendAvailability() {
	mqttClient.sendMessage(fmt.Sprintf("%s/sensor/%s/availability", mqttClient.prefix, mqttClient.uniqueDeviceId), "online")
}

func (mqttClient *Client) SendConfigurationData() {
	for id, config := range mqttClient.SensorConfigurationData {
		sensorID := fmt.Sprintf("%s/%s", mqttClient.uniqueDeviceId, id)
		config.Device = mqttClient.device
		mqttClient.sendMessage(fmt.Sprintf("homeassistant/sensor/%s/config", sensorID), config)
	}
}

// sendSensorData sends sensor data to Home Assistant
func (mqttClient *Client) SendSensorData(sensorStates map[string]SensorState) {
	for key, value := range sensorStates {
		sensorPayload := map[string]interface{}{key: value}
		mqttClient.sendMessage(fmt.Sprintf("homeassistant/sensor/%s/state", mqttClient.uniqueDeviceId), sensorPayload)
	}
}
