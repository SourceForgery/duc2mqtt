// main.go
package main

import (
	"encoding/json"
	"fmt"
	"github.com/SourceForgery/duc2mqtt/bastec"
	hassio2 "github.com/SourceForgery/duc2mqtt/hassio"
	"github.com/jessevdk/go-flags"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"net/url"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

var (
	version = "unknown"
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

type Options struct {
	ConfigFile    string `short:"c" long:"config" description:"Path to configuration file" default:"config.yaml"`
	LoggingFormat string `short:"l" long:"logging" choice:"coloured" choice:"plain" choice:"json" default:"coloured" description:"Log output format"`
	Verbose       []bool `short:"v" long:"verbose" description:"Enable verbose logging (repeat for more verbosity)"`
	Quiet         []bool `short:"q" long:"quiet" description:"Reduce verbosity (repeat for less verbosity)"`
	Version       bool   `short:"V" long:"version" description:"Print version information and exit"`
}

func main() {
	var opts Options

	// Parse command-line options.
	_, err := flags.Parse(&opts)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse command-line options")
	}

	config := parseConfig(opts)

	initializeLogging(opts)

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		log.Fatal().Msg("Failed to read build info")
	}

	var vcsVersion = "unknown"
	for _, setting := range buildInfo.Settings {
		if setting.Key == "vcs.revision" {
			vcsVersion = setting.Value
		}
	}

	if opts.Version {
		log.Info().
			Str("vcsVersion", vcsVersion).
			Str("goVersion", buildInfo.GoVersion).
			Str("version", version).
			Msgf("duc2mqtt version %s compiled with %s, commitId: %s", version, buildInfo.GoVersion, vcsVersion)
		os.Exit(1)
	}

	ducUrl, err := url.Parse(config.Duc.Url)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse DUC URL")
	}
	ducClient, err := bastec.Connect(*ducUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to DUC")
	}

	ducClient.DisallowedPrefixes = config.Duc.DisallowedPrefixes

	mqttUrl, err := url.Parse(config.Mqtt.Url)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse mqtt url")
	}

	amqpVhost := strings.TrimPrefix(mqttUrl.Path, "/")
	hassioClient, err := hassio2.ConnectMqtt(*mqttUrl, amqpVhost, config.Mqtt.UniqueId, config.Mqtt.TopicPrefix)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to mqtt")
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
		log.Fatal().Err(err).Msg("Failed to subscribe to Home Assistant status")
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
			log.Fatal().Err(err).Msg("Failed to get values")
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
				log.Error().Err(err).Msg("Failed to send sensor data")
			} else {
				log.Info().Msg("Successfully sent sensor data")
			}
		}
	}
}

func fetchMqttDeviceConfig(ducClient *bastec.BastecClient) map[string]hassio2.SensorConfig {
	browse, err := ducClient.Browse()
	if err != nil {
		log.Error().Err(err).Msg("Failed to browse")
	}

	sensorConfigs := map[string]hassio2.SensorConfig{}
device:
	for _, point := range browse.Result.Points {

		for _, prefix := range ducClient.DisallowedPrefixes {
			if strings.HasPrefix(point.Pid, prefix) {
				log.Debug().Msgf("Skipping sensor %s: %s", point.Pid, point.Desc)
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
			case "W":
				deviceClass = "power"
			case "kWh":
				deviceClass = "energy"
				stateClass = "total"
			default:
				log.Warn().Msgf("Unknown device class for sensor %s: %s", point.Pid, point.Attr)
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
			log.Warn().Msgf("Unknown device class for sensor %s: %s", point.Pid, point.Desc)
			continue
		}
		log.Info().Msgf("Found sensor %s(converted to %s): %s", point.Pid, hassio2.MqttName(point.Pid), point.Desc)
		sensorConfigs[point.Pid] = sensorConfig
	}
	return sensorConfigs
}

func parseConfig(opts Options) Config {
	// Load configuration from YAML file.
	configData, err := os.ReadFile(opts.ConfigFile)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read configuration file")
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
		log.Fatal().Err(err).Msg("Failed to parse configuration file")
	}

	if config.IntervalSeconds == 0 {
		config.IntervalSeconds = 10
	}
	return config
}

func initializeLogging(opts Options) {
	var lg zerolog.Logger
	switch loggingFormat := opts.LoggingFormat; loggingFormat {
	case "json":
		lg = zerolog.New(os.Stdout)
		break
	case "coloured":
		lg = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
		break
	case "plain":
		lg = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339, NoColor: true})
		break
	default:
		log.Panic().Msgf("What the f is %s", loggingFormat)
	}
	log.Logger = lg.With().Timestamp().Logger()
	setLogLevel(len(opts.Verbose))
}

func setLogLevel(verbosity int) {
	switch verbosity {
	case 0:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case 1:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}
}
