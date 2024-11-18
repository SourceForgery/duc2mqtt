package hassio

import (
	"encoding/json"
	"errors"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rotisserie/eris"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/url"
)

type Client struct {
	client                  MQTT.Client
	Device                  *Device
	uniqueDeviceId          string // optional. Duc's is used if not set
	SensorConfigurationData map[string]SensorConfig
	prefix                  string
}

func onConnectionLost(_ MQTT.Client, err error) {
	logger().Info().Msg("Connection lost")
}

func (hassioClient *Client) sendMessage(topic string, payload interface{}) (err error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		logger().Fatal().Msgf("Failed to serialize payload to %s", topic)
	}

	token := hassioClient.client.Publish(topic, 0, false, payloadBytes)
	token.Wait()
	if token.Error() != nil {
		return eris.Wrapf(token.Error(), "Error publishing to topic %s\n", topic)
	} else {
		if log.Logger.GetLevel() <= zerolog.DebugLevel {
			logger().Trace().Str("body", string(payloadBytes)).Msgf("Message published to topic %s", topic)
		} else {
			logger().Debug().Msgf("Message published to topic %s", topic)
		}
	}
	return nil
}

func ConnectMqtt(url url.URL, amqpVhost string, uniqueId string, prefix string) (hassioClient *Client, err error) {
	var password string
	var hasPassword bool
	if url.User == nil {
		return nil, errors.New("mqtt url needs to have username and password")
	}
	userInfo := *url.User
	userName := userInfo.Username()
	if password, hasPassword = userInfo.Password(); !hasPassword {
		return nil, errors.New("mqtt url needs to have username and password")
	}
	if amqpVhost != "" {
		userName = amqpVhost + ":" + userName
	}
	urlCopy := url
	urlCopy.User = nil
	logger().Debug().Msgf("Connecting to mqtt server '%s'", urlCopy.String())

	url.User = nil

	hassioClient = &Client{
		uniqueDeviceId: uniqueId,
		prefix:         prefix,
	}
	if prefix == "" {
		hassioClient.prefix = "homeassistant"
	}

	var onConnect MQTT.OnConnectHandler = func(_ MQTT.Client) {
		logger().Info().Msg("MQTT connection established")
		if hassioClient.Device != nil {
			err := hassioClient.SendLastWill()
			if err != nil {
				logger().Error().Err(err).Msg("Failed to write last will")
				return
			}
			err = hassioClient.SendAvailability()
			if err != nil {
				logger().Error().Err(err).Msg("Failed to send availability")
				return
			}
		}
	}
	opts := MQTT.NewClientOptions().AddBroker(url.String()).
		SetClientID(url.User.Username()).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectionLostHandler(onConnectionLost).
		SetOnConnectHandler(onConnect).
		SetPassword(password).
		SetUsername(userName)

	if log.Logger.GetLevel() <= zerolog.DebugLevel {
		var messagePubHandler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
			logger().Debug().Msgf("Received message: %s from topic: %s\n", string(msg.Payload()), msg.Topic())
		}
		opts.DefaultPublishHandler = messagePubHandler
	}

	// Create and start the client using the above options
	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, eris.Wrapf(token.Error(), "failed to connect to %s", url.String())
	}
	hassioClient.client = client

	logger().Info().Msgf("Connected to mqtt server '%s'", urlCopy.String())

	return
}
