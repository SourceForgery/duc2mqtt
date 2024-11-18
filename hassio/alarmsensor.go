package hassio

import (
	"fmt"
)

var _ SensorConfig = (*AlarmSensorConfig)(nil)

type AlarmSensorConfig struct {
	sensorId string
	name     string
}

func NewAlarmSensorConfig(sensorId string, name string) *AlarmSensorConfig {
	return &AlarmSensorConfig{
		sensorId: sensorId,
		name:     name,
	}
}

func (a AlarmSensorConfig) DeviceClass() string {
	return "problem"
}

func (a AlarmSensorConfig) Name() string {
	return a.name
}

func (a AlarmSensorConfig) UnitOfMeasurement() string {
	return ""
}

func (a AlarmSensorConfig) Decimals() int {
	return 0
}

func (a AlarmSensorConfig) SensorType() string {
	return "binary_sensor"
}

func (a AlarmSensorConfig) SensorId() string {
	return a.sensorId
}

func (a AlarmSensorConfig) ConvertValue(value float64) string {
	if value > 0 {
		return "ON"
	}
	return "OFF"
}

func (a AlarmSensorConfig) ValueTemplate() string {
	return fmt.Sprintf("{{ value_json['%s'] }}", a.sensorId)
}

func (a AlarmSensorConfig) StateClass() string {
	return "measurement"
}
