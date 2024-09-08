package hassio

import (
	"fmt"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rotisserie/eris"
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

type SensorConfig struct {
	DeviceClass       string `json:"device_class"`
	Name              string `json:"name"`
	UnitOfMeasurement string `json:"unit_of_measurement"`
	Decimals          int    `json:"decimals"`
}

// DiscoveryMessage represents the discovery payload to be sent to Home Assistant.
type DiscoveryMessage struct {
	Name              string  `json:"name"`
	DeviceClass       string  `json:"device_class"`
	UniqueID          string  `json:"unique_id"`      // The sensor id
	StateTopic        string  `json:"state_topic"`    // Shared by all devices
	CommandTopic      string  `json:"command_topic"`  // Not used by this device
	ValueTemplate     string  `json:"value_template"` // Converts the sensor state payload to string, e.g. '{{ value_json.power_meter}}'
	UnitOfMeasurement string  `json:"unit_of_measurement"`
	Device            *Device `json:"device"`
}

// Device represents the device information for Home Assistant.
type Device struct {
	Identifiers      []string `json:"identifiers"`
	Name             string   `json:"name"`
	SWVersion        string   `json:"sw_version"`
	HWVersion        string   `json:"hw_version"`
	SerialNumber     string   `json:"serial_number"`
	Model            string   `json:"model"`
	ModelID          string   `json:"model_id"`
	Manufacturer     string   `json:"manufacturer"`
	ConfigurationURL string   `json:"configuration_url"`
}

type SensorState struct {
	State string `json:"state"`
}

func (hassioClient *Client) SendAvailability() (err error) {
	err = hassioClient.sendMessage(fmt.Sprintf("%s/sensor/%s/availability", hassioClient.prefix, hassioClient.uniqueDeviceId), "online")
	if err != nil {
		return eris.Wrapf(err, "failed to send availability message\n")
	}
	return
}

func (hassioClient *Client) SendConfigurationData() (err error) {
	for sensorId, config := range hassioClient.SensorConfigurationData {
		payload := DiscoveryMessage{
			Name:              config.Name,
			DeviceClass:       config.DeviceClass,
			UniqueID:          sensorId,
			StateTopic:        fmt.Sprintf("%s/sensor/%s/config", hassioClient.prefix, hassioClient.uniqueDeviceId),
			ValueTemplate:     fmt.Sprintf("{{ value_json.%s / %d }}", sensorId, config.Decimals),
			UnitOfMeasurement: config.UnitOfMeasurement,
			Device:            hassioClient.Device,
		}
		return hassioClient.sendMessage(fmt.Sprintf("%s/sensor/%s/config", hassioClient.prefix, sensorId), payload)
	}
	return nil
}

func (hassioClient *Client) SendSensorData(sensorStates map[string]string) (err error) {
	err = hassioClient.sendMessage(fmt.Sprintf("%s/sensor/%s/state", hassioClient.prefix, hassioClient.uniqueDeviceId), sensorStates)
	if err != nil {
		return eris.Wrap(err, "Couldn't send sensor state\n")
	}
	return
}

func (hassioClient *Client) SendLastWill() (err error) {
	token := hassioClient.client.Publish(fmt.Sprintf("%s/sensor/%s/availability", hassioClient.prefix, hassioClient.uniqueDeviceId), 0, true, "offline")
	if err = token.Error(); err != nil {
		return eris.Wrap(err, "Couldn't send last will message\n")
	}
	return
}

func (hassioClient *Client) SubscribeToHomeAssistantStatus() (err error) {
	err = hassioClient.SendAvailability()
	if err == nil {
		err = hassioClient.SendLastWill()
	}
	if err == nil {
		err = hassioClient.client.Subscribe("%s/status", 0, func(client MQTT.Client, msg MQTT.Message) {
			if string(msg.Payload()) == "online" {
				if err := hassioClient.SendAvailability(); err != nil {
					logger().WithError(err).Errorf("Failed to subscribe to Home Assistant status: %s\n", err)
				}

				if err := hassioClient.SendConfigurationData(); err != nil {
					logger().WithError(err).Errorf("Failed to subscribe to Home Assistant status: %s\n", err)
				}
			}
		}).Error()
	}
	if err != nil {
		return eris.Wrap(err, "Couldn't subscribe to Home Assistant status\n")
	}
	return
}
