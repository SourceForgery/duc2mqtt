package hassio

import (
	"fmt"
	"math"
	"strconv"
)

type FloatSensorConfig struct {
	sensorId          string
	name              string
	deviceClass       string
	unitOfMeasurement string
	decimals          int
}

func NewFloatSensorConfig(
	sensorId string,
	name string,
	deviceClass string,
	unitOfMeasurement string,
	decimals int,
) *FloatSensorConfig {
	return &FloatSensorConfig{
		sensorId,
		name,
		deviceClass,
		unitOfMeasurement,
		decimals,
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

func (f *FloatSensorConfig) Decimals() int {
	return f.decimals
}

func (f *FloatSensorConfig) SensorType() string {
	return "sensor"
}

func (f *FloatSensorConfig) ConvertValue(value float64) string {
	return strconv.FormatFloat(value, 'f', f.decimals, 64)
}

func (f *FloatSensorConfig) ValueTemplate() string {
	return fmt.Sprintf("{{ value_json['%s'] | float / %d }}", f.sensorId, (int)(math.Pow10(f.decimals)))
}
