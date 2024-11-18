package hassio

import (
	"fmt"
)

var _ SensorConfig = (*FloatSensorConfig)(nil)

type FloatSensorConfig struct {
	sensorId          string
	name              string
	deviceClass       string
	unitOfMeasurement string
	stateClass        string
}

func NewFloatSensorConfig(
	sensorId string,
	name string,
	deviceClass string,
	unitOfMeasurement string,
	stateClass string,
) *FloatSensorConfig {
	return &FloatSensorConfig{
		sensorId,
		name,
		deviceClass,
		unitOfMeasurement,
		stateClass,
	}
}

func (f *FloatSensorConfig) DeviceClass() string {
	return f.deviceClass
}

func (f *FloatSensorConfig) Name() string {
	return f.name
}

func (f *FloatSensorConfig) UnitOfMeasurement() string {
	return f.unitOfMeasurement
}

func (f *FloatSensorConfig) SensorType() string {
	return "sensor"
}

func (f *FloatSensorConfig) ConvertValue(value float64) string {
	return fmt.Sprintf("%f", value)
}

func (f *FloatSensorConfig) ValueTemplate() string {
	return fmt.Sprintf("{{ value_json['%s'] | float }}", f.sensorId)
}

func (f *FloatSensorConfig) StateClass() string {
	return f.stateClass
}

func (f *FloatSensorConfig) SensorId() string { return f.sensorId }
