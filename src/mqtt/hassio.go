package mqtt

import (
	"fmt"
	MQTT "github.com/eclipse/paho.mqtt.golang"
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
	UniqueID            string  `json:"unique_id"`      // The sensor id
	StateTopic          string  `json:"state_topic"`    // Shared by all devices
	CommandTopic        string  `json:"command_topic"`  // Not used by this device
	ValueTemplate       string  `json:"value_template"` // Converts the sensor state payload to string, e.g. '{{ value_json.power_meter}}'
	UnitOfMeasurement   string  `json:"unit_of_measurement"`
	AvailabilityTopic   string  `json:"availability_topic"` // where the device says up/down
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
	uniqueId := mqttClient.Device.Identifiers[0]
	for id, config := range mqttClient.SensorConfigurationData {
		config.Device = mqttClient.Device
		mqttClient.sendMessage(fmt.Sprintf("homeassistant/sensor/mqttClient.uniqueDeviceId/config", sensorID), config)
	}
}

// sendSensorData sends sensor data to Home Assistant
func (mqttClient *Client) SendSensorData(sensorStates map[string]SensorState) {
	for key, value := range sensorStates {
		sensorPayload := map[string]interface{}{key: value}
		mqttClient.sendMessage(fmt.Sprintf("homeassistant/sensor/%s/state", mqttClient.uniqueDeviceId), sensorPayload)
	}
}

func (mqttClient *Client) SubscribeToHomeAssistantStatus() {
	mqttClient.client.Subscribe("%s/status", 0, func(client MQTT.Client, msg MQTT.Message) {
		if string(msg.Payload()) == "online" {
			mqttClient.SendAvailability()
			mqttClient.SendConfigurationData()
		}
	})
}
