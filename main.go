// main.go
package main

import (
	"encoding/json"
	"fmt"
	"github.com/SourceForgery/duc2mqtt/bastec"
	hassio2 "github.com/SourceForgery/duc2mqtt/hassio"
	"github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"net/url"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

// Config represents the YAML configuration structure.
type Config struct {
	Mqtt struct {
		Url         string `yaml:"url" json:"url"`
		UniqueId    string `yaml:"uniqueId" json:"uniqueId"`
		TopicPrefix string `yaml:"topicPrefix" json:"topicPrefix"`
		Name        string `yaml:"name" json:"name"`
	} `yaml:"mqtt" json:"mqtt"`
	Duc struct {
		Url                string   `yaml:"url" json:"url"`
		DisallowedPrefixes []string `yaml:"disallowedPrefixes" json:"disallowedPrefixes"`
	} `yaml:"duc" json:"duc"`
	IntervalSeconds int64 `yaml:"intervalSeconds" json:"intervalSeconds"`
}

func logger() *logrus.Entry {
	return logrus.WithField("logger", "main")
}

type Options struct {
	ConfigFile string `short:"c" long:"config" description:"Path to configuration file" default:"config.yaml"`
	Verbose    []bool `short:"v" long:"verbose" description:"Enable verbose logging (repeat for more verbosity)"`
	Quiet      []bool `short:"q" long:"quiet" description:"Reduce verbosity (repeat for less verbosity)"`
	Version    bool   `short:"V" long:"version" description:"Print version information and exit"`
}

func main() {
	var opts Options

	// Parse command-line options.
	_, err := flags.Parse(&opts)
	if err != nil {
		logger().
			WithError(err).
			Fatal("Failed to parse command-line options")
	}

	config := parseConfig(opts)

	initializeLogging(opts)

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		logger().Fatal("Failed to read build info")
	}

	var vcsVersion = "unknown"
	for _, setting := range buildInfo.Settings {
		if setting.Key == "vcs.revision" {
			vcsVersion = setting.Value
		}
	}

	if opts.Version {
		logger().
			WithField("vcsVersion", vcsVersion).
			WithField("goVersion", buildInfo.GoVersion).
			WithField("version", buildInfo.Main.Version).
			Infof("duc2mqtt version %s compiled with %s, commitId: %s", buildInfo.Main.Version, buildInfo.GoVersion, vcsVersion)
	}

	ducUrl, err := url.Parse(config.Duc.Url)
	if err != nil {
		logger().WithError(err).Fatal("Failed to parse DUC URL: ", err)
	}
	ducClient, err := bastec.Connect(*ducUrl)
	if err != nil {
		logger().WithError(err).Fatal("Failed to connect to DUC: ", err)
	}

	ducClient.DisallowedPrefixes = config.Duc.DisallowedPrefixes

	mqttUrl, err := url.Parse(config.Mqtt.Url)
	if err != nil {
		logger().WithError(err).Fatal("Failed to parse mqtt url", err)
	}

	amqpVhost := strings.TrimPrefix(mqttUrl.Path, "/")
	hassioClient, err := hassio2.ConnectMqtt(*mqttUrl, amqpVhost, config.Mqtt.UniqueId, config.Mqtt.TopicPrefix)
	if err != nil {
		logger().WithError(err).Fatal("Failed to connect to mqtt", err)
	}

	hassioClient.Device = &hassio2.Device{
		Identifiers:      []string{config.Mqtt.UniqueId},
		Name:             config.Mqtt.Name,
		SWVersion:        buildInfo.Main.Version,
		HWVersion:        "N/A",
		SerialNumber:     "N/A",
		Model:            "Duc2Mqtt",
		ModelID:          "Duc2Mqtt",
		Manufacturer:     "SourceForgery",
		ConfigurationURL: fmt.Sprintf("http://%s/config", ducUrl.Host),
	}

	hassioClient.SensorConfigurationData = fetchMqttDeviceConfig(ducClient)
	err = hassioClient.SubscribeToHomeAssistantStatus()
	if err != nil {
		logger().WithError(err).Fatal("Failed to subscribe to Home Assistant status: ", err)
	}

	config.publishValuesLoop(hassioClient, ducClient)
}

func (config *Config) publishValuesLoop(hassioClient *hassio2.Client, ducClient *bastec.BastecClient) {
	first := true
	for {
		if !first {
			time.Sleep(time.Duration(config.IntervalSeconds) * time.Second)
		}
		first = false
		var foo []string
		for value := range hassioClient.SensorConfigurationData {
			foo = append(foo, value)
		}
		values, err := ducClient.GetValues(foo)
		if err != nil {
			logger().WithError(err).Fatal("Failed to get values: ", err)
			continue
		}
		valuesToSend := make(map[string]map[string]string)

		for _, point := range values.Result.Points {
			sensorConfig := hassioClient.SensorConfigurationData[point.Pid]
			if valuesToSend[sensorConfig.SensorType()] == nil {
				valuesToSend[sensorConfig.SensorType()] = make(map[string]string)
			}
			valuesToSend[sensorConfig.SensorType()][point.Pid] = sensorConfig.ConvertValue(point.Value)
		}
		for sensorType, sensorValuesToSend := range valuesToSend {
			err = hassioClient.SendSensorData(sensorType, sensorValuesToSend)
			if err != nil {
				logger().WithError(err).Error("Failed to send sensor data: ", err)
			} else {
				logger().Infof("Successfully sent sensor data")
			}
		}
	}
}

func fetchMqttDeviceConfig(ducClient *bastec.BastecClient) map[string]hassio2.SensorConfig {
	browse, err := ducClient.Browse()
	if err != nil {
		logger().WithError(err).Errorf("Failed to browse: %v", err)
	}

	sensorConfigs := map[string]hassio2.SensorConfig{}
device:
	for _, point := range browse.Result.Points {

		for _, prefix := range ducClient.DisallowedPrefixes {
			if strings.HasPrefix(point.Pid, prefix) {
				logger().Debugf("Skipping sensor %s: %s", point.Pid, point.Desc)
				continue device
			}
		}

		var sensorConfig hassio2.SensorConfig
		switch point.Type {
		case "enum":
			sensorConfig = hassio2.NewAlarmSensorConfig(point.Pid, point.Desc)
		case "number":
			deviceClass := ""
			stateClass := "measurement"
			switch point.Attr {
			case "A":
				deviceClass = "current"
			case "V":
				deviceClass = "voltage"
			case "kWh":
				deviceClass = "energy"
				stateClass = "total"
			default:
				logger().Warnf("Unknown device class for sensor %s: %s", point.Pid, point.Attr)
				continue device
			}
			sensorConfig = hassio2.NewFloatSensorConfig(
				point.Pid,
				point.Desc,
				deviceClass,
				point.Attr,
				stateClass,
			)
		default:
			logger().Warnf("Unknown device class for sensor %s: %s", point.Pid, point.Desc)
			continue
		}
		logger().Infof("Found sensor %s(converted to %s): %s", point.Pid, hassio2.MqttName(point.Pid), point.Desc)
		sensorConfigs[point.Pid] = sensorConfig
	}
	return sensorConfigs
}

func parseConfig(opts Options) Config {
	// Load configuration from YAML file.
	configData, err := os.ReadFile(opts.ConfigFile)
	if err != nil {
		logger().WithError(err).Fatal("Failed to read configuration file: ", err)
	}

	var config Config
	if strings.HasSuffix(opts.ConfigFile, "yaml") {
		err = yaml.Unmarshal(configData, &config)
	} else if strings.HasSuffix(opts.ConfigFile, "json") {
		err = json.Unmarshal(configData, &config)
	} else {
		err = fmt.Errorf("unknown file extension: %s", opts.ConfigFile)
	}
	if err != nil {
		logger().WithError(err).Fatal("Failed to parse configuration file: ", err)
	}

	if config.IntervalSeconds == 0 {
		config.IntervalSeconds = 10
	}
	return config
}

func initializeLogging(opts Options) {
	// Set up logrus logging.
	switch len(opts.Verbose) - len(opts.Quiet) {
	case 2:
		logrus.SetLevel(logrus.TraceLevel)
	case 1:
		logrus.SetLevel(logrus.DebugLevel)
	case 0:
		logrus.SetLevel(logrus.InfoLevel)
	case -1:
		logrus.SetLevel(logrus.WarnLevel)
	case -2:
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		logrus.Fatal("Invalid log level specified")
	}

	logrus.SetOutput(os.Stdout)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}
