package hassio

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type FloatSensorConfig struct {
	NameField              string
	DeviceClassField       string
	UnitOfMeasurementField string
	DecimalsField          int
}

func (f *FloatSensorConfig) DeviceClass() string {
	return f.DeviceClassField
}

func (f *FloatSensorConfig) Name() string {
	return f.NameField
}

func (f *FloatSensorConfig) UnitOfMeasurement() string {
	return f.UnitOfMeasurementField
}

func (f *FloatSensorConfig) Decimals() int {
	return f.DecimalsField
}

func (f *FloatSensorConfig) MqttName() string {
	// Example implementation: return a formatted MQTT name
	return strings.ReplaceAll(".", "_", f.Name())
}

func (f *FloatSensorConfig) ConvertValue(value float64) string {
	return strconv.FormatFloat(value, 'f', f.DecimalsField, 64)
}

func (f *FloatSensorConfig) ValueTemplate() string {
	// Example implementation: return a value template
	return fmt.Sprintf("{{ value_json['%s'] | float / %d }}", f.NameField, (int)(math.Pow10(f.DecimalsField)))
}
