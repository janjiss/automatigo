package timer_light

import (
	"encoding/json"
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
	"github.com/nathan-osman/go-sunrise"
	"time"
)

type TimerLight struct {
	sensorTopics []string
	lightTopics  []string
	timer        *time.Timer
	mqttClient   mqtt.Client
}

func NewTimerLight(sensorTopics []string, lightTopics []string, mqttClient mqtt.Client) *TimerLight {
	return &TimerLight{
		sensorTopics: sensorTopics,
		lightTopics:  lightTopics,
		timer:        nil,
		mqttClient:   mqttClient,
	}

}

func (timerLight *TimerLight) Start() {
	for _, topic := range timerLight.sensorTopics {
		timerLight.mqttClient.Subscribe(topic, 0, timerLight.handleMessage)
	}
}

func (timerLight *TimerLight) handleMessage(client mqtt.Client, msg mqtt.Message) {
	var payload map[string]interface{}

	err := json.Unmarshal(msg.Payload(), &payload)

	if err != nil {
		fmt.Printf("Error unmarshalling payload: %v\n", err)
		return
	}

	occupancy, ok := payload["occupancy"].(bool)
	if !ok {
		fmt.Printf("Occupancy value missing or not a boolean: %v\n", payload["occupancy"])
		return
	}

	if occupancy {
		timerLight.handleOccupancy()
	} else {
		timerLight.handleNoOccupancy()
	}
}

func (timerLight *TimerLight) handleOccupancy() {
	if timerLight.timer != nil {
		timerLight.timer.Stop()
	}

	if !isSunUp() {
		timerLight.turnOnLight()
		timerLight.timer = time.AfterFunc(5*time.Minute, timerLight.turnOffLight)
	}
}

func (timerLight *TimerLight) handleNoOccupancy() {
	if timerLight.timer == nil || isSunUp() {
		timerLight.turnOffLight()
	}
}

func (timerLight *TimerLight) turnOffLight() {
	for _, topic := range timerLight.lightTopics {
		timerLight.mqttClient.Publish(topic, 0, false, `{"state": "off"}`)
	}
}

func (timerLight *TimerLight) turnOnLight() {
	for _, topic := range timerLight.lightTopics {
		timerLight.mqttClient.Publish(topic, 0, false, `{"state": "on"}`)
	}
}

func isSunUp() bool {
	currentTime := time.Now().UTC()

	rise, set := sunrise.SunriseSunset(
		56.97399, 21.95721,
		currentTime.Year(), currentTime.Month(), currentTime.Day(),
	)

	return currentTime.After(rise) && currentTime.Before(set)
}
