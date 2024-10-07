package pellet_stove

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// MQTTClientInterface defines the methods used from the MQTT client.
type MQTTClientInterface interface {
	Publish(topic string, qos byte, retained bool, payload interface{}) MQTT.Token
	Subscribe(topic string, qos byte, callback MQTT.MessageHandler) MQTT.Token
	Disconnect(quiesce uint)
}

// PelletStoveController manages the pellet stove based on temperature readings.
type PelletStoveController struct {
	Client             MQTTClientInterface
	CurrentTemperature float64
	TemperatureKnown   bool
	StoveOn            bool
	StoveStateKnown    bool
	Setpoint           float64
	TemperatureMargin  float64
	TemperatureTopic   string
	ControlTopic       string
	StatusTopic        string // New field for stove status topic
	MasterSwitchOn     bool
	HTTPClient         *http.Client
}

// NewPelletStoveController initializes a new controller instance.
func NewPelletStoveController(broker, clientID, temperatureTopic, controlTopic, statusTopic string, setpoint, margin float64) *PelletStoveController {
	opts := MQTT.NewClientOptions().AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetCleanSession(true) // Set to true unless you have a reason not to
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(10 * time.Second)

	controller := &PelletStoveController{
		CurrentTemperature: 0.0,
		TemperatureKnown:   false,
		StoveOn:            false,
		StoveStateKnown:    false,
		Setpoint:           setpoint,
		TemperatureMargin:  margin,
		TemperatureTopic:   temperatureTopic,
		ControlTopic:       controlTopic,
		MasterSwitchOn:     false,
		HTTPClient:         &http.Client{Timeout: 10 * time.Second},
	}

	controller.fetchDeviceInfo()

	runPeriodically(15*time.Second, func() {
		controller.fetchDeviceInfo()
		controller.ControlPelletStove()
	}, nil)

	opts.OnConnect = func(c MQTT.Client) {
		// Subscribe to temperature topic
		if token := c.Subscribe(temperatureTopic, 0, controller.temperatureHandler); token.Wait() && token.Error() != nil {
			fmt.Printf("Error subscribing to topic %s: %v\n", temperatureTopic, token.Error())
			os.Exit(1)
		}
		fmt.Printf("Subscribed to temperature topic %s\n", temperatureTopic)

		// Subscribe to status topic if provided

		if controlTopic != "" {
			if token := c.Subscribe(controlTopic, 0, controller.statusHandler); token.Wait() && token.Error() != nil {
				fmt.Printf("Error subscribing to topic %s: %v\n", controlTopic, token.Error())
				os.Exit(1)
			}

			c.Publish(controlTopic+"/get", 0, false, `{"power_on_behavior": ""}`)
			fmt.Printf("Subscribed to status topic %s\n", controlTopic)
		}
	}

	opts.OnConnectionLost = func(client MQTT.Client, err error) {
		fmt.Printf("Connection lost: %v\n", err)
	}

	mqttClient := MQTT.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		fmt.Printf("Error connecting to broker: %v\n", token.Error())
		os.Exit(1)
	}
	fmt.Println("Connected to MQTT broker")

	// Assign the MQTT client to the interface field
	controller.Client = mqttClient

	return controller
}

func (p *PelletStoveController) statusHandler(client MQTT.Client, msg MQTT.Message) {
	var payload map[string]interface{}
	err := json.Unmarshal(msg.Payload(), &payload)

	signal := payload["state"].(string)

	if err != nil {
		fmt.Printf("Error unmarshalling payload: %v\n", err)
	}

	switch signal {
	case "ON":
		p.StoveOn = true
		p.StoveStateKnown = true
	case "OFF":
		p.StoveOn = false
		p.StoveStateKnown = true
	default:
		fmt.Printf("Received unknown stove status: %s\n", signal)
	}
	fmt.Printf("Stove status updated: %s\n", signal)
	p.ControlPelletStove()
}

// Run starts the controller and waits for termination signals.
func (p *PelletStoveController) Run() {
	// Wait for interrupt signal to gracefully shutdown the controller
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	<-sigc
	fmt.Println("Shutting down pellet stove controller")
}

func (p *PelletStoveController) temperatureHandler(client MQTT.Client, msg MQTT.Message) {
	var payload map[string]interface{}
	err := json.Unmarshal(msg.Payload(), &payload)
	if err != nil {
		fmt.Printf("Error unmarshalling payload: %v\n", err)
		return
	}
	temp := payload["temperature"].(float64)

	p.CurrentTemperature = temp
	p.TemperatureKnown = true
	fmt.Printf("Received temperature: %.2f°C\n", p.CurrentTemperature)
	p.ControlPelletStove()
}

func (p *PelletStoveController) fetchDeviceInfo() {
	var request struct {
		Temperature float64 `json:"temperature"`
		StoveOn     bool    `json:"stove_on"`
	}

	request.Temperature = p.CurrentTemperature
	request.StoveOn = p.StoveOn

	body, err := json.Marshal(request)
	if err != nil {
		fmt.Println(err)
		return
	}

	req, err := http.NewRequest("POST", "https://app.janis.wtf/api/device", bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}

	// Add basic auth credentials to the request
	req.SetBasicAuth("janis", "sapkaja21")
	req.Header.Set("Content-Type", "application/json")
	req.Body.Close()

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		fmt.Printf("Error fetching temperature: %v\n", err)
		return
	}

	var result struct {
		DesiredTemperature float64 `json:"desired_temperature"`
		On                 bool    `json:"on"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("Error decoding temperature response: %v\n", err)
		return
	}

	p.Setpoint = result.DesiredTemperature
	p.MasterSwitchOn = result.On

	fmt.Printf("Received temperature from server: %.2f°C\n", result.DesiredTemperature)
}

func (p *PelletStoveController) ControlPelletStove() {
	if !p.TemperatureKnown || !p.StoveStateKnown {
		fmt.Println("Waiting for temperature and stove state information...")
		return
	}

	if !p.MasterSwitchOn {
		if p.StoveOn {
			p.turnStoveOff()
		}
		fmt.Println("Master switch is off, not controlling stove")
		return
	}

	if p.CurrentTemperature < (p.Setpoint-p.TemperatureMargin) && !p.StoveOn {
		p.turnStoveOn()
	} else if p.CurrentTemperature > (p.Setpoint+p.TemperatureMargin) && p.StoveOn {
		p.turnStoveOff()
	} else {
		fmt.Println("Stove state is already correct")
	}
}

func (p *PelletStoveController) turnStoveOn() {
	fmt.Println("Turning pellet stove ON")
	p.publishControlCommand(`{"state": "on"}`)
	p.StoveOn = true
	p.StoveStateKnown = true
}

func (p *PelletStoveController) turnStoveOff() {
	fmt.Println("Turning pellet stove OFF")
	p.publishControlCommand(`{"state": "off"}`)
	p.StoveOn = false
	p.StoveStateKnown = true
}

func (p *PelletStoveController) publishControlCommand(command string) {
	token := p.Client.Publish(p.ControlTopic+"/set", 0, false, command)
	token.Wait()
	if token.Error() != nil {
		fmt.Printf("Error publishing control command: %v\n", token.Error())
	}
}

func StartPelletStove(mqttHost string) {
	clientID := "PelletStoveController"
	temperatureTopic := "zigbee2mqtt/living-room-temp"
	controlTopic := "zigbee2mqtt/pellet-stove"
	statusTopic := "zigbee2mqtt/pellet-stove/get"
	setpoint := 21.0 // Desired temperature in Celsius
	margin := 0.5    // Temperature margin for hysteresis

	controller := NewPelletStoveController(mqttHost, clientID, temperatureTopic, controlTopic, statusTopic, setpoint, margin)
	controller.Run()
}

func runPeriodically(interval time.Duration, f func(), stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				f()
			case <-stop:
				return
			}
		}
	}()
}
