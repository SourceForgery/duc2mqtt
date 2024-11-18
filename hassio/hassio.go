package hassio

import (
	"fmt"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rotisserie/eris"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"slices"
	"strings"
)

var lg *zerolog.Logger

func logger() *zerolog.Logger {
	if lg == nil {
		l := log.Logger.With().Str("logger", "mqtt").Logger()
		lg = &l
	}
	return lg
}

// AvailabilityMessage represents the MQTT availability message
type AvailabilityMessage struct {
	DeviceID string `json:"device_id"`
	Status   string `json:"status"`
}
type SensorConfig interface {
	DeviceClass() string
	Name() string
	UnitOfMeasurement() string
	SensorType() string
	ConvertValue(value float64) string
	ValueTemplate() string
	StateClass() string
}

type SensorConfigX struct {
	DeviceClass       string   `json:"device_class,omitempty"`
	Name              string   `json:"name"`
	UnitOfMeasurement string   `json:"unit_of_measurement"`
	Decimals          int      `json:"decimals"`
	MqttName          string   `json:"-"`
	EnumValues        []string `json:"-"`
}

// DiscoveryMessage represents the discovery payload to be sent to Home Assistant.
type DiscoveryMessage struct {
	Name              string  `json:"name"`
	DeviceClass       string  `json:"device_class"`
	UniqueID          string  `json:"unique_id"`               // The sensor id
	StateTopic        string  `json:"state_topic"`             // Shared by all devices
	CommandTopic      string  `json:"command_topic,omitempty"` // Not used by this device
	ValueTemplate     string  `json:"value_template"`          // Converts the sensor state payload to string, e.g. '{{ value_json.power_meter}}'
	UnitOfMeasurement string  `json:"unit_of_measurement,omitempty"`
	Device            *Device `json:"device"`
	StateClass        string  `json:"state_class,omitempty"`
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

func MqttName(sensorId string) string {
	return strings.ReplaceAll(sensorId, ".", "_")
}

func (hassioClient *Client) sensorTypes() []string {
	sensorTypes := make([]string, 0)
	for _, config := range hassioClient.SensorConfigurationData {
		if !slices.Contains(sensorTypes, config.SensorType()) {
			sensorTypes = append(sensorTypes, config.SensorType())
		}
	}
	return sensorTypes
}

func (hassioClient *Client) SendAvailability() (err error) {
	for _, sensorType := range hassioClient.sensorTypes() {
		err = hassioClient.sendMessage(fmt.Sprintf("%s/%s/%s/availability", hassioClient.prefix, sensorType, hassioClient.uniqueDeviceId), "online")
		if err != nil {
			return eris.Wrapf(err, "failed to send availability message\n")
		}
	}
	return
}

func (hassioClient *Client) SendConfigurationData() (err error) {
	for sensorId, config := range hassioClient.SensorConfigurationData {
		payload := DiscoveryMessage{
			Name:              config.Name(),
			DeviceClass:       config.DeviceClass(),
			UniqueID:          sensorId,
			StateTopic:        fmt.Sprintf("%s/%s/%s/state", hassioClient.prefix, config.SensorType(), hassioClient.uniqueDeviceId),
			ValueTemplate:     config.ValueTemplate(),
			UnitOfMeasurement: config.UnitOfMeasurement(),
			Device:            hassioClient.Device,
			StateClass:        config.StateClass(),
		}
		err = hassioClient.sendMessage(fmt.Sprintf("%s/%s/%s/%s/config", hassioClient.prefix, config.SensorType(), hassioClient.uniqueDeviceId, MqttName(sensorId)), payload)
		if err != nil {
			return
		}
	}
	return nil
}

func (hassioClient *Client) SendSensorData(sensorType string, sensorStates map[string]string) (err error) {
	err = hassioClient.sendMessage(fmt.Sprintf("%s/%s/%s/state", hassioClient.prefix, sensorType, hassioClient.uniqueDeviceId), sensorStates)
	if err != nil {
		return eris.Wrap(err, "Couldn't send sensor state\n")
	}
	return
}

func (hassioClient *Client) SendLastWill() (err error) {
	for _, sensorType := range hassioClient.sensorTypes() {
		token := hassioClient.client.Publish(fmt.Sprintf("%s/%s/%s/availability", hassioClient.prefix, sensorType, hassioClient.uniqueDeviceId), 0, true, "offline")
		if err = token.Error(); err != nil {
			return eris.Wrap(err, "Couldn't send last will message\n")
		}
	}
	return
}

func (hassioClient *Client) SubscribeToHomeAssistantStatus() (err error) {
	err = hassioClient.SendAvailability()
	if err == nil {
		err = hassioClient.SendLastWill()
	}
	if err == nil {
		err = hassioClient.SendConfigurationData()
	}
	if err == nil {
		err = hassioClient.client.Subscribe(fmt.Sprintf("%s/status", hassioClient.prefix), 0, func(client MQTT.Client, msg MQTT.Message) {
			if string(msg.Payload()) == "online" {
				if err := hassioClient.SendAvailability(); err != nil {
					logger().Error().Err(err).Msg("Failed to subscribe to Home Assistant status")
				}

				if err := hassioClient.SendConfigurationData(); err != nil {
					logger().Error().Err(err).Msg("Failed to subscribe to Home Assistant status")
				}
			}
		}).Error()
	}
	if err != nil {
		return eris.Wrap(err, "Couldn't subscribe to Home Assistant status\n")
	}
	return
}
