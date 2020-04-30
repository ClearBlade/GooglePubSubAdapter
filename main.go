package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	gcpPubsub "cloud.google.com/go/pubsub"
	adapter_library "github.com/clearblade/adapter-go-library"
	mqttTypes "github.com/clearblade/mqtt_parsing"
	"google.golang.org/api/option"
)

const (
	msgSubscribeQos = 0
	msgPublishQos   = 0
)

var (
	//Adapter command line arguments not handled by adapter_library
	adapterName string //Defaults to gcpPubSubAdapter

	//adapter_library configuration structure
	adapterConfig *adapter_library.AdapterConfig

	//Google Cloud Platform related variables
	pubsubClient     *gcpPubsub.Client
	gcpSubscription  *gcpPubsub.Subscription
	gcpProjectID     string
	gcpCredsPath     string
	gcpPubTopic      string
	gcpSubTopic      string
	gcpPullInterval  = 10 //seconds
	gcpSubPreCreated = false

	//Miscellaneous adapter variables
	topicRoot          = "gcp/pubsub"
	cbSubscribeChannel <-chan *mqttTypes.Publish
	endWorkersChannel  chan string
	interruptChannel   chan os.Signal
)

func init() {
	flag.StringVar(&adapterName, "adapterName", "gcpPubSubAdapter", "name of device the adapter will use for authentication (optional)")
}

func main() {
	fmt.Println("Starting GooglePubSubAdapter...")

	err := adapter_library.ParseArguments(adapterName)
	if err != nil {
		log.Fatalf("[FATAL] Failed to parse arguments: %s\n", err.Error())
	}

	// initialize all things ClearBlade, includes authenticating if needed, and fetching the
	// relevant adapter_config collection entry
	adapterConfig, err = adapter_library.Initialize()
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize: %s\n", err.Error())
	}

	applyAdapterSettings(adapterConfig)

	//Connect to ClearBlade MQTT
	if gcpPubTopic != "" {
		err = adapter_library.ConnectMQTT(adapterConfig.TopicRoot+"/publish", cbMessageHandler)
		if err != nil {
			log.Fatalf("[FATAL] Failed to Connect MQTT: %s\n", err.Error())
		}
	} else {
		err = adapter_library.ConnectMQTT("", nil)
		if err != nil {
			log.Fatalf("[FATAL] Failed to Connect MQTT: %s\n", err.Error())
		}
	}

	//Authenticate with GooglePubSub
	log.Println("[INFO] Authenticating to GCP")
	googleAuthExplicit(gcpProjectID, gcpCredsPath)

	//Subscribe to GCP and initiate the pull worker
	if gcpSubTopic != "" {
		log.Println("[DEBUG] gcpSubTopic provided, Invoking gcpSubscribe")
		gcpSubscribe()

		//Start GCP subscription pull loop
		log.Println("[DEBUG] Invoking gcpPullWorker")
		go gcpPullWorker()
	}

	defer close(endWorkersChannel)
	endWorkersChannel = make(chan string)

	//Handle OS interrupts to shut down gracefully
	interruptChannel := make(chan os.Signal, 1)
	signal.Notify(interruptChannel, syscall.SIGINT, syscall.SIGTERM)
	sig := <-interruptChannel

	log.Printf("[INFO] OS signal %s received, ending go routines.", sig)

	//End the existing goRoutines
	endWorkersChannel <- "Stop Channel"

	if gcpPubTopic != "" {
		endWorkersChannel <- "Stop Channel"
	}
	os.Exit(0)
}

func gcpPullWorker() {
	log.Println("[INFO] gcpPullWorker - Starting GCP pullWorker")
	ticker := time.NewTicker(time.Duration(gcpPullInterval) * time.Second)

	for {
		select {
		case <-ticker.C:
			log.Println("[DEBUG] gcpPullWorker - Invoking gcpPull")
			gcpPull()
		case <-endWorkersChannel:
			log.Println("[DEBUG] gcpPullWorker - stopping ticker")
			ticker.Stop()
			return
		}
	}
}

// Subscribes to a topic on google pubsub
func gcpSubscribe() {
	//Create the new topic
	topic := pubsubClient.TopicInProject(gcpSubTopic, gcpProjectID)
	log.Printf("[DEBUG] gcpSubscribe - GCP topic reference %v created.\n", topic)

	if gcpSubPreCreated {
		//Create a reference to the subscription
		log.Println("[DEBUG] gcpSubscribe - Creating GCP subscription reference")
		gcpSubscription = pubsubClient.SubscriptionInProject(gcpSubTopic, gcpProjectID)
	} else {
		//Create a reference to the subscription
		log.Println("[DEBUG] gcpSubscribe - Creating GCP subscription")
		var err error

		gcpSubscription, err = pubsubClient.CreateSubscription(context.Background(), gcpSubTopic, pubsub.SubscriptionConfig{
			Topic:            topic,
			AckDeadline:      10 * time.Second,
			ExpirationPolicy: time.Duration(0),
		})
		if err != nil {
			log.Printf("[ERROR] gcpSubscribe - Error subscribing to gcpSubTopic: %s\n", err.Error())
			log.Fatal(err)
		}
	}
}

func gcpPull() {
	log.Println("[INFO] gcpPullWorker - Pulling recent messages")
	err := gcpSubscription.Receive(context.Background(), func(ctx context.Context, m *pubsub.Message) {
		log.Printf("[DEBUG] gcpPull - Google PubSub message received: %v\n", m)

		adapter_library.Publish(topicRoot, m.Data)
		m.Ack()
	})
	if err != context.Canceled {
		log.Printf("[ERROR] gcpPull - Error receiving Google PubSub messages: %s\n", err.Error())
		adapter_library.Publish(topicRoot+"/error", []byte(err.Error()))
	}
}

func gcpPublish(topic string, data string) error {
	ctx := context.Background()
	gcpTopic, err := pubsubClient.CreateTopic(ctx, topic)
	defer gcpTopic.Stop()

	if err != nil {
		fmt.Printf("[ERROR] gcpPublish - Error creating GCP topic: %s\n", err.Error())
	}

	res := gcpTopic.Publish(ctx, &pubsub.Message{Data: []byte(data)})
	id, err := res.Get(ctx)
	if err != nil {
		fmt.Printf("[ERROR] gcpPublish - Error publishing message: %s\n", err.Error())
	}
	fmt.Printf("[INFO] gcpPublish - Published a message with a message ID: %s\n", id)
	return nil
}

func applyAdapterSettings(adapterSettings *adapter_library.AdapterConfig) {
	var settingsJSON map[string]interface{}
	if err := json.Unmarshal([]byte(adapterSettings.AdapterSettings), &settingsJSON); err != nil {
		log.Printf("[ERROR] applyAdapterSettings - Error while unmarshalling json: %s. Defaulting all adapter settings.\n", err.Error())
	}

	//GCP Project ID
	if settingsJSON["gcpProjectID"] != nil && settingsJSON["gcpProjectID"] != "" {
		gcpProjectID = settingsJSON["gcpProjectID"].(string)
		log.Printf("[DEBUG] applyAdapterSettings - Setting gcpProjectID to %s\n", gcpProjectID)
	} else {
		panic("gcpProjectID not provided. Authentication to GCP is not possible.")
	}

	//GCP Credentials file path
	if settingsJSON["gcpCredsPath"] != nil && settingsJSON["gcpCredsPath"] != "" {
		gcpCredsPath = settingsJSON["gcpCredsPath"].(string)
		log.Printf("[DEBUG] applyAdapterSettings - Setting gcpCredsPath to %s\n", gcpCredsPath)
	} else {
		panic("gcpCredsPath not provided. Authentication to GCP is not possible.")
	}

	//gcpPubTopic
	if settingsJSON["gcpPubTopic"] != nil {
		gcpPubTopic = settingsJSON["gcpPubTopic"].(string)
		log.Printf("[DEBUG] applyAdapterSettings - Setting gcpPubTopic to %s\n", gcpPubTopic)
	} else {
		log.Printf("[INFO] applyAdapterSettings - A gcpPubTopic value was not found.\n")
	}

	//gcpSubTopic
	if settingsJSON["gcpSubTopic"] != nil {
		gcpSubTopic = settingsJSON["gcpSubTopic"].(string)
		log.Printf("[DEBUG] applyAdapterSettings - Setting gcpSubTopic to %s\n", gcpSubTopic)
	} else {
		log.Printf("[INFO] applyAdapterSettings - A gcpPubTopic value was not found.\n")
	}

	//gcpPullInterval
	if settingsJSON["gcpPullInterval"] != nil {
		log.Printf("[DEBUG] applyAdapterSettings - Setting gcpPullInterval to %d\n", gcpPullInterval)
		gcpPullInterval = int(settingsJSON["gcpPullInterval"].(float64))
	} else {
		log.Printf("[INFO] applyAdapterSettings - A gcpPullInterval value was not found.\n")
	}

	//gcpSubPreCreated
	if settingsJSON["gcpSubPreCreated"] != nil {
		log.Printf("[DEBUG] applyAdapterSettings - Setting gcpSubPreCreated to %t\n", gcpSubPreCreated)
		gcpSubPreCreated = settingsJSON["gcpSubPreCreated"].(bool)
	} else {
		log.Printf("[INFO] applyAdapterSettings - A gcpSubPreCreated value was not found.\n")
	}
}

// authExplicit reads Google cloud credentials from the specified path.
func googleAuthExplicit(projectID string, jsonPath string) {
	ctx := context.Background()
	var err error

	pubsubClient, err = gcpPubsub.NewClient(ctx, projectID, option.WithCredentialsFile(jsonPath))
	if err != nil {
		log.Printf("[ERROR] googleAuthExplicit - Error authenticating to GCP: %s\n", err.Error())
		log.Fatal(err)
	}
	log.Println("[INFO] googleAuthExplicit - Authentication to GCP successful")
}

func cbMessageHandler(message *mqttTypes.Publish) {
	// process incoming MQTT messages as needed here
	//Ensure a publish request was received
	if strings.HasSuffix(message.Topic.Whole, topicRoot+"/publish") {
		log.Println("[INFO] cbMessageHandler - Handling GCP publish request...")
		gcpPublish(gcpPubTopic, string(message.Payload))
	} else {
		log.Printf("[ERROR] cbMessageHandler - Unknown request received: topic = %s, payload = %#v\n", message.Topic.Whole, message.Payload)
	}
}
