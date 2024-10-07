package hassio

import (
	"fmt"
)

type FloatSensorConfig struct {
	sensorId          string
	name              string
	deviceClass       string
	unitOfMeasurement string
}

func NewFloatSensorConfig(
	sensorId string,
	name string,
	deviceClass string,
	unitOfMeasurement string,
) *FloatSensorConfig {
	return &FloatSensorConfig{
		sensorId,
		name,
		deviceClass,
		unitOfMeasurement,
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
