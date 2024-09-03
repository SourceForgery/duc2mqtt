// main.go
package main

import (
	"duc2mqtt/src/bastec"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"gopkg.in/yaml.v3"
	"net/url"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
)

// Config represents the YAML configuration structure.
type Config struct {
	Mqtt struct {
		Url string `yaml:"url"`
	} `yaml:"mqtt"`
	Duc struct {
		Url string `yaml:"url"`
	}
}

// Options represents the command-line options.
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
		logrus.Fatal("Failed to parse command-line options: ", err)
	}

	config := parseConfig(opts)
	initializeLogging(opts)

	ducUrl, err := url.Parse(config.Duc.Url)
	if err != nil {
		logrus.Fatal("Failed to parse DUC URL: ", err)
	}
	client, err := bastec.Connect(*ducUrl)
	if err != nil {
		logrus.Fatal("Failed to connect to DUC: ", err)
	}

	mqttClient := initializeMqtt(opts, config)

	browse, err := client.Browse()
	if err != nil {
		logrus.Errorf("Failed to browse %s: %v", opts2, err)
	}
	for _, point := range browse.Result.Points {

	}
}

func initializeMqtt(opts Options, config Config) *bastec.BrowseResponse {
	opts2 := MQTT.NewClientOptions().AddBroker("")

}

func parseConfig(opts Options) Config {
	// Load configuration from YAML file.
	configData, err := os.ReadFile(opts.ConfigFile)
	if err != nil {
		logrus.Fatal("Failed to read configuration file: ", err)
	}

	var config Config
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		logrus.Fatal("Failed to parse configuration file: ", err)
	}
	return config
}

func initializeLogging(opts Options) {
	// Set up logrus logging.
	switch len(opts.Verbose) - len(opts.Quiet) {
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
