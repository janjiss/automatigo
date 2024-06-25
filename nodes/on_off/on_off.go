package on_off

import (
	"encoding/json"
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
)

type OnOff struct {
	switchTopics []string
	lightTopics  []string
	mqttClient   mqtt.Client
}

func NewOnOff(switchTopcis []string, lightTopics []string, mqttClient mqtt.Client) *OnOff {
	return &OnOff{
		switchTopics: switchTopcis,
		lightTopics:  lightTopics,
		mqttClient:   mqttClient,
	}

}

func (onOff *OnOff) Start() {
	for _, topic := range onOff.switchTopics {
		onOff.mqttClient.Subscribe(topic, 0, onOff.handleMessage)
	}
}

func (onOff *OnOff) handleMessage(client mqtt.Client, msg mqtt.Message) {
	var payload map[string]interface{}
	err := json.Unmarshal(msg.Payload(), &payload)

	if err != nil {
		fmt.Printf("Error unmarshalling payload: %v\n", err)
		return
	}

	action, ok := payload["action"].(string)
	if !ok {
		fmt.Printf("Occupancy value missing or not a boolean: %v\n", payload["action"])
		return
	}
	if action == "off" {
		onOff.turnOff()
	}

	if action == "on" {
		onOff.turnOn()
	}
}

func (onOff *OnOff) turnOff() {
	for _, topic := range onOff.lightTopics {
		onOff.mqttClient.Publish(topic, 0, false, `{"state": "off"}`)
	}
}

func (onOff *OnOff) turnOn() {
	for _, topic := range onOff.lightTopics {
		onOff.mqttClient.Publish(topic, 0, false, `{"state": "on"}`)
	}
}
