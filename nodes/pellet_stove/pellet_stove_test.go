package pellet_stove_test

import (
	"testing"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	ps "janjiss.com/automatigo/nodes/pellet_stove"
)

// MockMQTTClient is a mock implementation of MQTTClientInterface.
type MockMQTTClient struct {
	publishedMessages []MockMessage
}

type MockMessage struct {
	Topic   string
	Payload string
	Qos     byte
	Retain  bool
}

func (m *MockMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) MQTT.Token {
	m.publishedMessages = append(m.publishedMessages, MockMessage{
		Topic:   topic,
		Payload: payload.(string),
		Qos:     qos,
		Retain:  retained,
	})
	return &DummyToken{}
}

func (m *MockMQTTClient) Subscribe(topic string, qos byte, callback MQTT.MessageHandler) MQTT.Token {
	// For testing, we can simulate receiving messages by calling the callback directly if needed.
	return &DummyToken{}
}

func (m *MockMQTTClient) Disconnect(quiesce uint) {
	// No-op for mock
}

// DummyToken is a mock implementation of the MQTT Token interface.
type DummyToken struct{}

func (t *DummyToken) Wait() bool                       { return true }
func (t *DummyToken) WaitTimeout(d time.Duration) bool { return true }
func (t *DummyToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (t *DummyToken) Error() error { return nil }

// TestControlPelletStove tests the control logic of the pellet stove controller.
func TestControlPelletStove_UnknownInitialState(t *testing.T) {
	mockClient := &MockMQTTClient{}
	controller := &ps.PelletStoveController{
		Client:             mockClient,
		CurrentTemperature: 0.0,
		TemperatureKnown:   false,
		StoveOn:            false,
		StoveStateKnown:    false,
		Setpoint:           21.0,
		TemperatureMargin:  0.5,
		ControlTopic:       "pelletstove/control",
	}

	// Simulate receiving temperature
	controller.CurrentTemperature = 20.0
	controller.TemperatureKnown = true
	controller.ControlPelletStove()

	// Should not take any action because stove state is unknown
	if len(mockClient.publishedMessages) != 0 {
		t.Errorf("Expected no commands sent when stove state is unknown, got %v", mockClient.publishedMessages)
	}

	// Simulate receiving stove status
	controller.StoveOn = false
	controller.StoveStateKnown = true
	controller.ControlPelletStove()

	// Now it should turn the stove on
	if !controller.StoveOn {
		t.Errorf("Expected stove to be ON")
	}
	if len(mockClient.publishedMessages) != 1 || mockClient.publishedMessages[0].Payload != "ON" {
		t.Errorf("Expected command to be ON, got %v", mockClient.publishedMessages)
	}
}
