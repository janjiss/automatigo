package main

import (
	"github.com/Netflix/go-env"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"janjiss.com/automatigo/nodes/on_off"
	"janjiss.com/automatigo/nodes/pellet_stove"
	"janjiss.com/automatigo/nodes/timer_light"
)

type Environment struct {
	MqttHost              string  `env:"MQTT_HOST"`
	ClientId              string  `env:"CLIENT_ID"`
	TemperatureTopic      string  `env:"TEMPERATURE_TOPIC"`
	PelletStoveRelayTopic string  `env:"PELLET_STOVE_RELAY_TOPIC"`
	TargetTemperature     float64 `env:"TARGET_TEMPERATURE"`
	Hysteresis            float64 `env:"HYSTERESIS"`
	MinRuntime            int     `env:"MIN_RUNTIME_MINUTES"`
}

func startPatioLight(client mqtt.Client) {
	sensorTopics := []string{"zigbee2mqtt/patio-sensor"}
	lightTopics := []string{"zigbee2mqtt/patio-outlet/set"}

	timerLight := timer_light.NewTimerLight(
		sensorTopics,
		lightTopics,
		client,
	)

	timerLight.Start()
}

func startFrontDoorLight(client mqtt.Client) {
	sensorTopics := []string{"zigbee2mqtt/front-door-sensor-1", "zigbee2mqtt/front-door-sensor-2"}
	lightTopics := []string{"zigbee2mqtt/front-door-lamp-1/set", "zigbee2mqtt/front-door-lamp-2/set"}

	timerLight := timer_light.NewTimerLight(
		sensorTopics,
		lightTopics,
		client,
	)

	timerLight.Start()
}

func startDirtyRoomSocket(client mqtt.Client) {
	switchTopics := []string{"zigbee2mqtt/dirty-room-switch"}
	lightTopics := []string{"zigbee2mqtt/dirty-room-outlet/set"}

	onOff := on_off.NewOnOff(
		switchTopics,
		lightTopics,
		client,
	)

	onOff.Start()
}

func startLivingRoomSocket(client mqtt.Client) {
	switchTopics := []string{"zigbee2mqtt/living-room-switch"}
	lightTopics := []string{"zigbee2mqtt/living-room-outlet/set"}

	onOff := on_off.NewOnOff(
		switchTopics,
		lightTopics,
		client,
	)

	onOff.Start()
}

func main() {
	var environment Environment
	_, err := env.UnmarshalFromEnviron(&environment)
	if err != nil {
		panic(err)
	}

	opts := mqtt.NewClientOptions().AddBroker(environment.MqttHost)

	opts.SetClientID(environment.ClientId)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	go startPatioLight(client)
	go startFrontDoorLight(client)
	go startDirtyRoomSocket(client)
	go startLivingRoomSocket(client)

	pellet_stove.StartPelletStove(environment.MqttHost)
}
