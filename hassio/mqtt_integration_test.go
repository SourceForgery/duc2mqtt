package hassio

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type probeMessage struct {
	topic    string
	payload  string
	retained bool
}

func TestConnectMqtt_RequiresCredentials(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		url     string
		errorIs string
	}{
		{name: "missing user info", url: "tcp://127.0.0.1:1883", errorIs: "username and password"},
		{name: "missing password", url: "tcp://user@127.0.0.1:1883", errorIs: "username and password"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mqttURL, err := url.Parse(tc.url)
			if err != nil {
				t.Fatalf("failed to parse url: %v", err)
			}
			_, err = ConnectMqtt(*mqttURL, "", "device-1", "")
			if err == nil {
				t.Fatalf("expected error for %s", tc.url)
			}
			if !strings.Contains(err.Error(), tc.errorIs) {
				t.Fatalf("expected error containing %q, got %q", tc.errorIs, err.Error())
			}
		})
	}
}

func TestConnectMqtt_WithCredentials_DoesNotPanicAndConnects(t *testing.T) {
	broker := startDockerBroker(t)

	mqttURL, err := url.Parse(fmt.Sprintf("tcp://user:password@%s", broker.address))
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("ConnectMqtt panicked: %v", recovered)
		}
	}()

	client, err := ConnectMqtt(*mqttURL, "", "device-1", "")
	if err != nil {
		t.Fatalf("expected connection to succeed, got error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	client.client.Disconnect(250)
}

func TestSendAvailability_PublishesOneOnlinePerSensorType(t *testing.T) {
	broker := startDockerBroker(t)
	client := newConnectedClient(t, broker.address, "homeassistant", "device-availability")
	probeClient, probeMessages := newProbeSubscriber(t, broker.address, "probe-availability")
	defer probeClient.Disconnect(250)

	client.SensorConfigurationData = testSensorConfigs()

	if err := client.SendAvailability(); err != nil {
		t.Fatalf("SendAvailability failed: %v", err)
	}

	waitForMessage(t, probeMessages, 5*time.Second, func(msg probeMessage) bool {
		return msg.topic == "homeassistant/sensor/device-availability/availability" && msg.payload == "online"
	})
	waitForMessage(t, probeMessages, 5*time.Second, func(msg probeMessage) bool {
		return msg.topic == "homeassistant/binary_sensor/device-availability/availability" && msg.payload == "online"
	})
}

func TestSendLastWill_RetainedOfflineIsVisibleToNewSubscriber(t *testing.T) {
	broker := startDockerBroker(t)
	client := newConnectedClient(t, broker.address, "homeassistant", "device-lastwill")
	client.SensorConfigurationData = testSensorConfigs()

	if err := client.SendLastWill(); err != nil {
		t.Fatalf("SendLastWill failed: %v", err)
	}

	probeClient, probeMessages := newProbeSubscriber(t, broker.address, "probe-lastwill")
	defer probeClient.Disconnect(250)

	waitForMessage(t, probeMessages, 5*time.Second, func(msg probeMessage) bool {
		return msg.topic == "homeassistant/sensor/device-lastwill/availability" && msg.payload == "offline" && msg.retained
	})
	waitForMessage(t, probeMessages, 5*time.Second, func(msg probeMessage) bool {
		return msg.topic == "homeassistant/binary_sensor/device-lastwill/availability" && msg.payload == "offline" && msg.retained
	})
}

func TestSendConfigurationData_PublishesExpectedDiscoveryMessages(t *testing.T) {
	broker := startDockerBroker(t)
	client := newConnectedClient(t, broker.address, "homeassistant", "device-config")
	probeClient, probeMessages := newProbeSubscriber(t, broker.address, "probe-config")
	defer probeClient.Disconnect(250)

	client.Device = &Device{
		Identifiers:      []string{"device-config"},
		Name:             "Device Config",
		SWVersion:        "test",
		HWVersion:        "test",
		SerialNumber:     "test",
		Model:            "test",
		ModelID:          "test",
		Manufacturer:     "test",
		ConfigurationURL: "http://example.invalid/config",
	}
	client.SensorConfigurationData = map[string]SensorConfig{
		"power.main": NewFloatSensorConfig("power.main", "Main Power", "power", "W", "measurement"),
		"alarm.main": NewAlarmSensorConfig("alarm.main", "Main Alarm"),
	}

	if err := client.SendConfigurationData(); err != nil {
		t.Fatalf("SendConfigurationData failed: %v", err)
	}

	powerMessage := waitForMessage(t, probeMessages, 5*time.Second, func(msg probeMessage) bool {
		return msg.topic == "homeassistant/sensor/device-config/power_main/config"
	})
	alarmMessage := waitForMessage(t, probeMessages, 5*time.Second, func(msg probeMessage) bool {
		return msg.topic == "homeassistant/binary_sensor/device-config/alarm_main/config"
	})

	assertDiscoveryPayload(t, powerMessage.payload, "power.main", "Main Power", "homeassistant/sensor/device-config/state")
	assertDiscoveryPayload(t, alarmMessage.payload, "alarm.main", "Main Alarm", "homeassistant/binary_sensor/device-config/state")
}

func TestSendSensorData_PublishesJSONState(t *testing.T) {
	broker := startDockerBroker(t)
	client := newConnectedClient(t, broker.address, "customprefix", "device-state")
	probeClient, probeMessages := newProbeSubscriber(t, broker.address, "probe-state")
	defer probeClient.Disconnect(250)

	states := map[string]string{
		"power.main": "123.45",
		"power.aux":  "42.00",
	}
	if err := client.SendSensorData("sensor", states); err != nil {
		t.Fatalf("SendSensorData failed: %v", err)
	}

	message := waitForMessage(t, probeMessages, 5*time.Second, func(msg probeMessage) bool {
		return msg.topic == "customprefix/sensor/device-state/state"
	})

	decoded := map[string]string{}
	if err := json.Unmarshal([]byte(message.payload), &decoded); err != nil {
		t.Fatalf("failed to parse state payload %q: %v", message.payload, err)
	}

	for key, expected := range states {
		if decoded[key] != expected {
			t.Fatalf("expected state[%s]=%q, got %q", key, expected, decoded[key])
		}
	}
}

func TestSubscribeToHomeAssistantStatus_RepublishesOnOnline(t *testing.T) {
	broker := startDockerBroker(t)
	client := newConnectedClient(t, broker.address, "homeassistant", "device-subscribe")
	probeClient, probeMessages := newProbeSubscriber(t, broker.address, "probe-subscribe")
	defer probeClient.Disconnect(250)

	publisher := newRawClient(t, broker.address, "publisher-subscribe")
	defer publisher.Disconnect(250)

	client.Device = &Device{
		Identifiers:      []string{"device-subscribe"},
		Name:             "Subscribe Device",
		SWVersion:        "test",
		HWVersion:        "test",
		SerialNumber:     "test",
		Model:            "test",
		ModelID:          "test",
		Manufacturer:     "test",
		ConfigurationURL: "http://example.invalid/config",
	}
	client.SensorConfigurationData = map[string]SensorConfig{
		"power.main": NewFloatSensorConfig("power.main", "Main Power", "power", "W", "measurement"),
	}

	if err := client.SubscribeToHomeAssistantStatus(); err != nil {
		t.Fatalf("SubscribeToHomeAssistantStatus failed: %v", err)
	}

	drainMessages(probeMessages, 300*time.Millisecond)

	statusToken := publisher.Publish("homeassistant/status", 0, false, "online")
	statusToken.Wait()
	if statusToken.Error() != nil {
		t.Fatalf("failed to publish homeassistant status online: %v", statusToken.Error())
	}

	waitForMessage(t, probeMessages, 5*time.Second, func(msg probeMessage) bool {
		return msg.topic == "homeassistant/sensor/device-subscribe/availability" && msg.payload == "online"
	})
	waitForMessage(t, probeMessages, 5*time.Second, func(msg probeMessage) bool {
		return msg.topic == "homeassistant/sensor/device-subscribe/power_main/config"
	})
}

func newConnectedClient(t *testing.T, address string, prefix string, uniqueID string) *Client {
	t.Helper()

	rawClient := newRawClient(t, address, "hassio-client-"+uniqueID)
	t.Cleanup(func() {
		rawClient.Disconnect(250)
	})

	return &Client{
		client:                  rawClient,
		uniqueDeviceId:          uniqueID,
		SensorConfigurationData: map[string]SensorConfig{},
		prefix:                  prefix,
	}
}

func newRawClient(t *testing.T, address string, clientID string) MQTT.Client {
	t.Helper()

	opts := MQTT.NewClientOptions().
		AddBroker("tcp://" + address).
		SetClientID(clientID + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)).
		SetConnectTimeout(5 * time.Second).
		SetConnectRetry(false)

	client := MQTT.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if token.Error() != nil {
		t.Fatalf("failed to connect paho client to broker at %s: %v", address, token.Error())
	}
	return client
}

func newProbeSubscriber(t *testing.T, address string, clientID string) (MQTT.Client, <-chan probeMessage) {
	t.Helper()

	messages := make(chan probeMessage, 200)
	handler := func(_ MQTT.Client, msg MQTT.Message) {
		messages <- probeMessage{topic: msg.Topic(), payload: string(msg.Payload()), retained: msg.Retained()}
	}
	opts := MQTT.NewClientOptions().
		AddBroker("tcp://" + address).
		SetClientID(clientID + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)).
		SetConnectTimeout(5 * time.Second).
		SetConnectRetry(false)

	client := MQTT.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if token.Error() != nil {
		t.Fatalf("failed to connect probe client: %v", token.Error())
	}

	subscribeToken := client.Subscribe("#", 0, handler)
	subscribeToken.Wait()
	if subscribeToken.Error() != nil {
		client.Disconnect(250)
		t.Fatalf("failed to subscribe probe client: %v", subscribeToken.Error())
	}
	return client, messages
}

func waitForMessage(t *testing.T, messages <-chan probeMessage, timeout time.Duration, predicate func(probeMessage) bool) probeMessage {
	t.Helper()

	deadline := time.After(timeout)
	seen := make([]probeMessage, 0, 8)
	for {
		select {
		case msg := <-messages:
			if len(seen) < 8 {
				seen = append(seen, msg)
			}
			if predicate(msg) {
				return msg
			}
		case <-deadline:
			t.Fatalf("timed out after %s waiting for expected mqtt message (seen %d messages: %+v)", timeout, len(seen), seen)
		}
	}
}

func drainMessages(messages <-chan probeMessage, duration time.Duration) {
	deadline := time.After(duration)
	for {
		select {
		case <-messages:
		case <-deadline:
			return
		}
	}
}

func assertDiscoveryPayload(t *testing.T, payload string, expectedUniqueID string, expectedName string, expectedStateTopic string) {
	t.Helper()

	message := DiscoveryMessage{}
	if err := json.Unmarshal([]byte(payload), &message); err != nil {
		t.Fatalf("failed to parse discovery payload %q: %v", payload, err)
	}
	if message.UniqueID != expectedUniqueID {
		t.Fatalf("expected unique_id %q, got %q", expectedUniqueID, message.UniqueID)
	}
	if message.Name != expectedName {
		t.Fatalf("expected name %q, got %q", expectedName, message.Name)
	}
	if message.StateTopic != expectedStateTopic {
		t.Fatalf("expected state_topic %q, got %q", expectedStateTopic, message.StateTopic)
	}
}

func testSensorConfigs() map[string]SensorConfig {
	return map[string]SensorConfig{
		"power.main": NewFloatSensorConfig("power.main", "Main Power", "power", "W", "measurement"),
		"power.aux":  NewFloatSensorConfig("power.aux", "Aux Power", "power", "W", "measurement"),
		"alarm.main": NewAlarmSensorConfig("alarm.main", "Main Alarm"),
	}
}

type dockerBroker struct {
	containerID string
	address     string
}

func startDockerBroker(t *testing.T) *dockerBroker {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skipf("docker binary not available: %v", err)
	}

	configDir := t.TempDir()
	configPath := configDir + "/mosquitto.conf"
	config := "listener 1883\nallow_anonymous true\npersistence false\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("failed writing mosquitto config: %v", err)
	}

	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	runCommand := exec.CommandContext(
		runCtx,
		"docker",
		"run",
		"-d",
		"--rm",
		"-p",
		"0:1883",
		"-v",
		configPath+":/mosquitto/config/mosquitto.conf:ro",
		"eclipse-mosquitto:2",
	)
	runOutput, err := runCommand.CombinedOutput()
	if err != nil {
		t.Skipf("unable to start mqtt broker container: %v (%s)", err, strings.TrimSpace(string(runOutput)))
	}
	containerID := strings.TrimSpace(string(runOutput))
	containerID = extractContainerID(containerID)
	if containerID == "" {
		t.Skipf("unable to parse mqtt broker container id from output: %s", strings.TrimSpace(string(runOutput)))
	}

	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer stopCancel()
		_ = exec.CommandContext(stopCtx, "docker", "rm", "-f", containerID).Run()
	})

	port := dockerMappedPort(t, containerID)
	address := "127.0.0.1:" + port
	waitForTCP(t, address, 10*time.Second)

	return &dockerBroker{containerID: containerID, address: address}
}

func extractContainerID(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		fields := strings.Fields(line)
		for j := len(fields) - 1; j >= 0; j-- {
			field := strings.TrimSpace(fields[j])
			if len(field) < 12 {
				continue
			}
			if isLikelyContainerID(field) {
				return field
			}
		}
	}
	return ""
}

func isLikelyContainerID(value string) bool {
	for _, c := range value {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	return true
}

func dockerMappedPort(t *testing.T, containerID string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "port", containerID, "1883/tcp").CombinedOutput()
	if err != nil {
		t.Fatalf("failed resolving broker port: %v (%s)", err, strings.TrimSpace(string(out)))
	}

	mapping := strings.TrimSpace(string(out))
	parts := strings.Split(mapping, ":")
	if len(parts) < 2 {
		t.Fatalf("unexpected docker port output: %q", mapping)
	}
	port := strings.TrimSpace(parts[len(parts)-1])
	if port == "" {
		t.Fatalf("unexpected empty port in output: %q", mapping)
	}
	return port
}

func waitForTCP(t *testing.T, address string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("broker at %s did not become ready before %s", address, timeout)
}
