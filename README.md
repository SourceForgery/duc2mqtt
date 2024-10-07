[![ci](https://github.com/SourceForgery/duc2mqtt/actions/workflows/ci.yml/badge.svg)](https://github.com/SourceForgery/duc2mqtt/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/SourceForgery/duc2mqtt/bastec.svg)](https://pkg.go.dev/github.com/SourceForgery/duc2mqtt/bastec)
[![Go Reference](https://pkg.go.dev/badge/github.com/SourceForgery/duc2mqtt/bastec.svg)](https://pkg.go.dev/github.com/SourceForgery/duc2mqtt/hassio)

# DUC2MQTT

duc2mqtt is a project designed to integrate with Home Assistant via MQTT, allowing you to publish sensors from Bastec Bas2 DUC to MQTT.

As the license states, this software is NOT endorsed by Bastec, nor is it supported by them in any way. 

## Installation

### Using systemd

1. Copy the `duc2mqtt.service` file to the systemd directory.
    ```sh
    sudo cp duc2mqtt.service /etc/systemd/system/duc2mqtt.service
    ```

2. Reload the systemd daemon to acknowledge the new service file.
    ```sh
    sudo systemctl daemon-reload
    ```

3. Enable the service to start at boot.
    ```sh
    sudo systemctl enable duc2mqtt.service
    ```

4. Start the service.
    ```sh
    sudo systemctl start duc2mqtt.service
    ```

5. Check the status to ensure it's running properly.
    ```sh
    sudo systemctl status duc2mqtt.service
    ```

### Using Docker Compose

1. Ensure you have Docker Compose installed on your system.
    ```sh
    sudo apt-get install docker-compose
    ```

2. Navigate to the directory containing the `docker-compose.yml` file.

3. Start the services defined in the Docker Compose file.
    ```sh
    docker-compose up -d
    ```

4. Verify the containers are running.
    ```sh
    docker-compose ps
    ```
   
### As a Home Assistant addon

Unfortunately, I've been unable to make a repository, so the following is required.
The instructions are for HassOS, but should apply to any docker-based home assistant installation

1. Install an ssh-server on home assistant (I use the official addon Terminal & SSH)
2. Use sftp to upload the files in the repository's addon directory into e.g. duc2mqtt.
   If you're not used to sftp, I recommend using filezilla. Caveat: SFTP is what you want, _NOT_ FTPS
3. The structure should look like
   /addon/duc2mqtt/Dockerfile
   /addon/duc2mqtt/config.json
4. Restart the hassio
5. Go into the plugin and press install. It will take a while as it will download
   and build the duc2mqtt software.
6. Configure it

## Building the Project

### Prerequisites

- Go SDK
- Docker (for containerized build)

### Building with Go

1. Clone the repository if you haven't already.
    ```sh
    git clone https://github.com/SourceForgery/duc2mqtt.git
    cd duc2mqtt
    ```

2. Build the Go project.
    ```sh
    go build -o duc2mqtt ./src/main.go
    ```

3. You can now run the executable.
    ```sh
    ./duc2mqtt
    ```

### Building with Docker

1. Ensure Docker is installed on your system.
    ```sh
    sudo apt-get install docker
    ```

2. Build the Docker image.
    ```sh
    docker build -t duc2mqtt:latest .
    ```

3. Run the Docker container.
    ```sh
    docker run -d --name duc2mqtt duc2mqtt:latest
    ```

## Configuration

### Yaml
```yaml
mqtt:
  url: tcp://user:password@test.mosquitto.org:1883
  uniqueId: duccer
  topicPrefix: homeassistant
duc:
  url: http://foo:bar@192.168.0.0
  disallowedPrefixes:
  - 1.dm.
  - 1.al.
  - 1.am.
```

### Json
```json
{
  "mqtt": {
    "url": "tcp://user:password@test.mosquitto.org:1883",
    "uniqueId": "duccer",
    "topicPrefix": "homeassistant"
  },
  "duc": {
    "url": "http://foo:bar@192.168.0.0",
    "disallowedPrefixes": [
      "1.dm.",
      "1.al.",
      "1.am."
    ]
  }
}
```

### Explanation

* mqtt
  * **url** the url to the mqtt server (with auth). I use rabbitmq which is an amqp server which has a special caveat,
    namely vhosts. vhosts are set by adding a path at the end, e.g.
   `tcp://user:password@test.mosquitto.org:1883/my_vhost`. If you
     have `/` as vhost, you need to set it in the url with double slashes, e.g. `....org:1883//`
  * **topicPrefix** optional. If not set, will default to "homeassistant". it's the first part
    of the mqtt topic published to, e.g. "homeassistant/sensor/id/status"
  * **uniqueId** what the device will present itself as in the mqtt. Just use something that isn't used by something else.
* duc
  * **url** http url (with auth) used to connect to the Bas2 duc. 
  * **disallowedPrefixes** There are a lot of test properties in a freshly installed duc that are
    useless. Some others are not interesting for other reasons. This allows for blacklisting
    sensor pids.


## Contributing

This is mostly a scratch-my-itch project. That means issues and pull requests for bugs will be happily investigated
or accepted, but maybe not in a timely fashion. 

Feature issues and pull requests on the other hand are less likely to be accepted due to it likely introducing some
bugs my environment doesn't check for. There's absolutely nothing stopping you from forking (least of all me),
so don't let my lack of time/interest stop you from making great features. Don't be suprised if I grab a feature
from your fork, should I see a use for it in my environment.

## License

Apache 2.0 License
