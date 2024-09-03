package mqtt

import (
	"errors"
	"fmt"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"net/url"
	"time"
)

type MqttClient struct {
}

func ConnectMqtt(url url.URL) (mqttClient MqttClient, err error){
	userInfo := url.User
	if userInfo != nil {
		return nil, errors.New("mqtt url needs to have username and password")
	}
	var password string
	if password, hasPassword := userInfo.Password()

	Connect("tcp://broker.hivemq.com:1883", "go_mqtt_client", "username", "password")
}

func Connect(broker string, clientID string, username string, password string) (MQTT.Client, error) {
	opts := MQTT.NewClientOptions().AddBroker("tcp://broker.hivemq.com:1883")
	opts.SetClientID("go_mqtt_client")

	// Define the message handler
	var messagePubHandler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
	}

	// Assign the message handler
	opts.SetDefaultPublishHandler(messagePubHandler)

	// Create and start the client using the above options
	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	// Subscribe to a topic
	topic := "my/testtopic"
	if token := client.Subscribe(topic, 1, nil); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		return
	}
	fmt.Printf("Subscribed to topic: %s\n", topic)

	// Publish a test message
	token := client.Publish(topic, 0, false, "Hello MQTT")
	token.Wait()

	time.Sleep(3 * time.Second) // Wait for message to be received

	// Disconnect the client
	client.Disconnect(250)
}
