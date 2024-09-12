package hassio

import "fmt"

type AlarmSensorConfig struct {
	NameField string
}

func (a AlarmSensorConfig) DeviceClass() string {
	return "problem"
}

func (a AlarmSensorConfig) Name() string {
	return a.NameField
}

func (a AlarmSensorConfig) UnitOfMeasurement() string {
	return ""
}

func (a AlarmSensorConfig) Decimals() int {
	return 0
}

func (a AlarmSensorConfig) MqttName() string {
	return fmt.Sprintf("home/alarm/%s", a.NameField)
}

func (a AlarmSensorConfig) ConvertValue(value float64) string {
	if value > 0 {
		return "on"
	}
	return "off"
}

func (a AlarmSensorConfig) ValueTemplate() string {
	return "{{ value }}"
}
