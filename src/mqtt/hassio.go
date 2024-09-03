package mqtt

// AvailabilityMessage represents the availability payload for Home Assistant.
type AvailabilityMessage struct {
	Payload string `json:"payload"`
}

// DiscoveryMessage represents the discovery payload to be sent to Home Assistant.
type DiscoveryMessage struct {
	Name                string `json:"name"`
	UniqueID            string `json:"unique_id"`
	StateTopic          string `json:"state_topic"`
	AvailabilityTopic   string `json:"availability_topic"`
	PayloadAvailable    string `json:"payload_available"`
	PayloadNotAvailable string `json:"payload_not_available"`
	Device              Device `json:"device"`
}

// Device represents the device information for Home Assistant.
type Device struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Model        string   `json:"model"`
	Manufacturer string   `json:"manufacturer"`
}
