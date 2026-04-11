package hassio

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type fakeToken struct {
	err error
}

func (t *fakeToken) Wait() bool                       { return true }
func (t *fakeToken) WaitTimeout(_ time.Duration) bool { return true }
func (t *fakeToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (t *fakeToken) Error() error { return t.err }

type publishCall struct {
	topic    string
	qos      byte
	retained bool
	payload  interface{}
}

type subscribeCall struct {
	topic string
	qos   byte
}

type fakeMQTTClient struct {
	publishCalls      []publishCall
	subscribeCalls    []subscribeCall
	subscribeHandlers []MQTT.MessageHandler
	publishErr        error
	subscribeErr      error
	publishErrByTopic map[string]error
}

func (f *fakeMQTTClient) IsConnected() bool      { return true }
func (f *fakeMQTTClient) IsConnectionOpen() bool { return true }
func (f *fakeMQTTClient) Connect() MQTT.Token    { return &fakeToken{} }
func (f *fakeMQTTClient) Disconnect(_ uint)      {}

func (f *fakeMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) MQTT.Token {
	f.publishCalls = append(f.publishCalls, publishCall{topic: topic, qos: qos, retained: retained, payload: payload})
	if f.publishErrByTopic != nil {
		if err, ok := f.publishErrByTopic[topic]; ok {
			return &fakeToken{err: err}
		}
	}
	return &fakeToken{err: f.publishErr}
}

func (f *fakeMQTTClient) Subscribe(topic string, qos byte, callback MQTT.MessageHandler) MQTT.Token {
	f.subscribeCalls = append(f.subscribeCalls, subscribeCall{topic: topic, qos: qos})
	f.subscribeHandlers = append(f.subscribeHandlers, callback)
	return &fakeToken{err: f.subscribeErr}
}

func (f *fakeMQTTClient) SubscribeMultiple(_ map[string]byte, _ MQTT.MessageHandler) MQTT.Token {
	return &fakeToken{}
}

func (f *fakeMQTTClient) Unsubscribe(_ ...string) MQTT.Token {
	return &fakeToken{}
}

func (f *fakeMQTTClient) AddRoute(_ string, _ MQTT.MessageHandler) {}

func (f *fakeMQTTClient) OptionsReader() MQTT.ClientOptionsReader {
	return MQTT.ClientOptionsReader{}
}

type fakeMessage struct {
	topic    string
	payload  []byte
	retained bool
}

func (m fakeMessage) Duplicate() bool   { return false }
func (m fakeMessage) Qos() byte         { return 0 }
func (m fakeMessage) Retained() bool    { return m.retained }
func (m fakeMessage) Topic() string     { return m.topic }
func (m fakeMessage) MessageID() uint16 { return 0 }
func (m fakeMessage) Payload() []byte   { return m.payload }
func (m fakeMessage) Ack()              {}

func payloadString(t *testing.T, payload interface{}) string {
	t.Helper()
	switch value := payload.(type) {
	case []byte:
		return string(value)
	case string:
		return value
	default:
		t.Fatalf("expected []byte or string payload, got %T", payload)
		return ""
	}
}

func newUnitClient(fake *fakeMQTTClient) *Client {
	return &Client{
		client:                  fake,
		uniqueDeviceId:          "device-123",
		SensorConfigurationData: map[string]SensorConfig{},
		prefix:                  "homeassistant",
	}
}

func TestMqttName(t *testing.T) {
	if got := MqttName("a.b.c"); got != "a_b_c" {
		t.Fatalf("expected a_b_c, got %q", got)
	}
}

func TestFloatSensorConfig(t *testing.T) {
	cfg := NewFloatSensorConfig("power.main", "Main Power", "power", "W", "measurement")
	if cfg.DeviceClass() != "power" {
		t.Fatalf("unexpected device class: %q", cfg.DeviceClass())
	}
	if cfg.Name() != "Main Power" {
		t.Fatalf("unexpected name: %q", cfg.Name())
	}
	if cfg.UnitOfMeasurement() != "W" {
		t.Fatalf("unexpected unit: %q", cfg.UnitOfMeasurement())
	}
	if cfg.SensorType() != "sensor" {
		t.Fatalf("unexpected sensor type: %q", cfg.SensorType())
	}
	if cfg.ConvertValue(12.34) != "12.340000" {
		t.Fatalf("unexpected converted value: %q", cfg.ConvertValue(12.34))
	}
	if cfg.ValueTemplate() != "{{ value_json['power.main'] | float }}" {
		t.Fatalf("unexpected template: %q", cfg.ValueTemplate())
	}
	if cfg.StateClass() != "measurement" {
		t.Fatalf("unexpected state class: %q", cfg.StateClass())
	}
	if cfg.SensorId() != "power.main" {
		t.Fatalf("unexpected sensor id: %q", cfg.SensorId())
	}
}

func TestAlarmSensorConfig(t *testing.T) {
	cfg := NewAlarmSensorConfig("alarm.main", "Main Alarm")
	if cfg.DeviceClass() != "problem" {
		t.Fatalf("unexpected device class: %q", cfg.DeviceClass())
	}
	if cfg.Name() != "Main Alarm" {
		t.Fatalf("unexpected name: %q", cfg.Name())
	}
	if cfg.UnitOfMeasurement() != "" {
		t.Fatalf("unexpected unit: %q", cfg.UnitOfMeasurement())
	}
	if cfg.SensorType() != "binary_sensor" {
		t.Fatalf("unexpected sensor type: %q", cfg.SensorType())
	}
	if cfg.SensorId() != "alarm.main" {
		t.Fatalf("unexpected sensor id: %q", cfg.SensorId())
	}
	if cfg.ConvertValue(1) != "ON" {
		t.Fatalf("expected ON for positive value")
	}
	if cfg.ConvertValue(0) != "OFF" {
		t.Fatalf("expected OFF for zero value")
	}
	if cfg.ValueTemplate() != "{{ value_json['alarm.main'] }}" {
		t.Fatalf("unexpected template: %q", cfg.ValueTemplate())
	}
	if cfg.StateClass() != "measurement" {
		t.Fatalf("unexpected state class: %q", cfg.StateClass())
	}
}

func TestSensorTypes_DeduplicatesTypes(t *testing.T) {
	client := &Client{
		SensorConfigurationData: map[string]SensorConfig{
			"power.main": NewFloatSensorConfig("power.main", "Main Power", "power", "W", "measurement"),
			"power.aux":  NewFloatSensorConfig("power.aux", "Aux Power", "power", "W", "measurement"),
			"alarm.main": NewAlarmSensorConfig("alarm.main", "Main Alarm"),
		},
	}

	types := client.sensorTypes()
	got := map[string]bool{}
	for _, sensorType := range types {
		got[sensorType] = true
	}

	expected := map[string]bool{"sensor": true, "binary_sensor": true}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("unexpected sensor type set: %#v", got)
	}
}

func TestSendAvailability(t *testing.T) {
	fake := &fakeMQTTClient{}
	client := newUnitClient(fake)
	client.SensorConfigurationData = map[string]SensorConfig{
		"power.main": NewFloatSensorConfig("power.main", "Main Power", "power", "W", "measurement"),
		"alarm.main": NewAlarmSensorConfig("alarm.main", "Main Alarm"),
	}

	if err := client.SendAvailability(); err != nil {
		t.Fatalf("SendAvailability failed: %v", err)
	}
	if len(fake.publishCalls) != 2 {
		t.Fatalf("expected 2 publish calls, got %d", len(fake.publishCalls))
	}

	seen := map[string]string{}
	for _, call := range fake.publishCalls {
		seen[call.topic] = payloadString(t, call.payload)
	}
	if seen["homeassistant/sensor/device-123/availability"] != "online" {
		t.Fatalf("unexpected sensor availability payload: %q", seen["homeassistant/sensor/device-123/availability"])
	}
	if seen["homeassistant/binary_sensor/device-123/availability"] != "online" {
		t.Fatalf("unexpected binary sensor availability payload: %q", seen["homeassistant/binary_sensor/device-123/availability"])
	}
}

func TestSendAvailability_PropagatesPublishError(t *testing.T) {
	fake := &fakeMQTTClient{publishErr: errors.New("publish failed")}
	client := newUnitClient(fake)
	client.SensorConfigurationData = map[string]SensorConfig{
		"power.main": NewFloatSensorConfig("power.main", "Main Power", "power", "W", "measurement"),
	}

	err := client.SendAvailability()
	if err == nil || !strings.Contains(err.Error(), "failed to send availability") {
		t.Fatalf("expected wrapped availability error, got: %v", err)
	}
}

func TestSendConfigurationData(t *testing.T) {
	fake := &fakeMQTTClient{}
	client := newUnitClient(fake)
	client.Device = &Device{Name: "Unit Device"}
	client.SensorConfigurationData = map[string]SensorConfig{
		"power.main": NewFloatSensorConfig("power.main", "Main Power", "power", "W", "measurement"),
	}

	if err := client.SendConfigurationData(); err != nil {
		t.Fatalf("SendConfigurationData failed: %v", err)
	}
	if len(fake.publishCalls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(fake.publishCalls))
	}
	call := fake.publishCalls[0]
	if call.topic != "homeassistant/sensor/device-123/power_main/config" {
		t.Fatalf("unexpected discovery topic: %q", call.topic)
	}

	msg := DiscoveryMessage{}
	if err := json.Unmarshal([]byte(payloadString(t, call.payload)), &msg); err != nil {
		t.Fatalf("failed to parse discovery payload: %v", err)
	}
	if msg.UniqueID != "power.main" {
		t.Fatalf("unexpected unique_id: %q", msg.UniqueID)
	}
	if msg.StateTopic != "homeassistant/sensor/device-123/state" {
		t.Fatalf("unexpected state_topic: %q", msg.StateTopic)
	}
}

func TestSendSensorData(t *testing.T) {
	fake := &fakeMQTTClient{}
	client := newUnitClient(fake)

	states := map[string]string{"power.main": "123.0"}
	if err := client.SendSensorData("sensor", states); err != nil {
		t.Fatalf("SendSensorData failed: %v", err)
	}
	if len(fake.publishCalls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(fake.publishCalls))
	}
	call := fake.publishCalls[0]
	if call.topic != "homeassistant/sensor/device-123/state" {
		t.Fatalf("unexpected state topic: %q", call.topic)
	}

	decoded := map[string]string{}
	if err := json.Unmarshal([]byte(payloadString(t, call.payload)), &decoded); err != nil {
		t.Fatalf("failed to decode state payload: %v", err)
	}
	if !reflect.DeepEqual(decoded, states) {
		t.Fatalf("unexpected state payload: %#v", decoded)
	}
}

func TestSendSensorData_PropagatesError(t *testing.T) {
	fake := &fakeMQTTClient{publishErr: errors.New("publish failed")}
	client := newUnitClient(fake)

	err := client.SendSensorData("sensor", map[string]string{"x": "1"})
	if err == nil || !strings.Contains(err.Error(), "Couldn't send sensor state") {
		t.Fatalf("expected wrapped sensor state error, got: %v", err)
	}
}

func TestSendLastWill(t *testing.T) {
	fake := &fakeMQTTClient{}
	client := newUnitClient(fake)
	client.SensorConfigurationData = map[string]SensorConfig{
		"power.main": NewFloatSensorConfig("power.main", "Main Power", "power", "W", "measurement"),
		"alarm.main": NewAlarmSensorConfig("alarm.main", "Main Alarm"),
	}

	if err := client.SendLastWill(); err != nil {
		t.Fatalf("SendLastWill failed: %v", err)
	}

	seen := map[string]publishCall{}
	for _, call := range fake.publishCalls {
		seen[call.topic] = call
	}

	sensorTopic := "homeassistant/sensor/device-123/availability"
	binaryTopic := "homeassistant/binary_sensor/device-123/availability"
	if call, ok := seen[sensorTopic]; !ok || call.payload != "offline" || !call.retained {
		t.Fatalf("unexpected sensor last will call: %+v", call)
	}
	if call, ok := seen[binaryTopic]; !ok || call.payload != "offline" || !call.retained {
		t.Fatalf("unexpected binary sensor last will call: %+v", call)
	}
}

func TestSubscribeToHomeAssistantStatus(t *testing.T) {
	fake := &fakeMQTTClient{}
	client := newUnitClient(fake)
	client.SensorConfigurationData = map[string]SensorConfig{
		"power.main": NewFloatSensorConfig("power.main", "Main Power", "power", "W", "measurement"),
	}

	if err := client.SubscribeToHomeAssistantStatus(); err != nil {
		t.Fatalf("SubscribeToHomeAssistantStatus failed: %v", err)
	}
	if len(fake.subscribeCalls) != 1 {
		t.Fatalf("expected 1 subscribe call, got %d", len(fake.subscribeCalls))
	}
	if fake.subscribeCalls[0].topic != "homeassistant/status" {
		t.Fatalf("unexpected subscribe topic: %q", fake.subscribeCalls[0].topic)
	}
	if len(fake.subscribeHandlers) != 1 || fake.subscribeHandlers[0] == nil {
		t.Fatal("expected exactly one subscribe handler")
	}

	// Initial availability + last will + discovery
	if len(fake.publishCalls) != 3 {
		t.Fatalf("expected 3 initial publish calls, got %d", len(fake.publishCalls))
	}

	fake.subscribeHandlers[0](fake, fakeMessage{topic: "homeassistant/status", payload: []byte("online")})
	if len(fake.publishCalls) != 5 {
		t.Fatalf("expected 5 publish calls after online callback, got %d", len(fake.publishCalls))
	}

	fake.subscribeHandlers[0](fake, fakeMessage{topic: "homeassistant/status", payload: []byte("offline")})
	if len(fake.publishCalls) != 5 {
		t.Fatalf("offline status should not trigger publish, got %d calls", len(fake.publishCalls))
	}
}

func TestSubscribeToHomeAssistantStatus_SubscribeError(t *testing.T) {
	fake := &fakeMQTTClient{subscribeErr: errors.New("subscribe failed")}
	client := newUnitClient(fake)
	client.SensorConfigurationData = map[string]SensorConfig{
		"power.main": NewFloatSensorConfig("power.main", "Main Power", "power", "W", "measurement"),
	}

	err := client.SubscribeToHomeAssistantStatus()
	if err == nil || !strings.Contains(err.Error(), "Couldn't subscribe to Home Assistant status") {
		t.Fatalf("expected wrapped subscribe error, got: %v", err)
	}
}
