package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	gcpPubsub "cloud.google.com/go/pubsub"
	cb "github.com/clearblade/Go-SDK"
	mqttTypes "github.com/clearblade/mqtt_parsing"
	mqtt "github.com/clearblade/paho.mqtt.golang"
	"github.com/hashicorp/logutils"
	"google.golang.org/api/option"
)

const (
	platURL                        = "http://localhost:9000"
	messURL                        = "localhost:1883"
	msgSubscribeQos                = 0
	msgPublishQos                  = 0
	adapterConfigCollectionDefault = "adapter_config"
)

var (
	//Adapter command line arguments
	platformURL             string //Defaults to http://localhost:9000
	messagingURL            string //Defaults to localhost:1883
	sysKey                  string
	sysSec                  string
	deviceName              string //Defaults to gcpPubSubAdapter
	activeKey               string
	logLevel                string //Defaults to info
	adapterConfigCollection string

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
	topicRoot = "gcp/pubsub"

	cbBroker           cbPlatformBroker
	cbSubscribeChannel <-chan *mqttTypes.Publish
	endWorkersChannel  chan string
	interruptChannel   chan os.Signal
)

type cbPlatformBroker struct {
	name         string
	clientID     string
	client       *cb.DeviceClient
	platformURL  *string
	messagingURL *string
	systemKey    *string
	systemSecret *string
	deviceName   *string
	password     *string
	topic        string
	qos          int
}

func init() {
	flag.StringVar(&sysKey, "systemKey", "", "system key (required)")
	flag.StringVar(&sysSec, "systemSecret", "", "system secret (required)")
	flag.StringVar(&deviceName, "deviceName", "gcpPubSubAdapter", "name of device (optional)")
	flag.StringVar(&activeKey, "password", "", "password (or active key) for device authentication (required)")
	flag.StringVar(&platformURL, "platformURL", platURL, "platform url (optional)")
	flag.StringVar(&messagingURL, "messagingURL", messURL, "messaging URL (optional)")
	flag.StringVar(&logLevel, "logLevel", "info", "The level of logging to use. Available levels are 'debug, 'info', 'warn', 'error', 'fatal' (optional)")
	flag.StringVar(&adapterConfigCollection, "adapterConfigCollection", adapterConfigCollectionDefault, "The name of the data collection used to house adapter configuration (optional)")
}

func usage() {
	log.Printf("Usage: GooglePubSubAdapter [options]\n\n")
	flag.PrintDefaults()
}

func validateFlags() {
	flag.Parse()

	if sysKey == "" || sysSec == "" || activeKey == "" {

		log.Printf("ERROR - Missing required flags\n\n")
		flag.Usage()
		os.Exit(1)
	}
}

func main() {
	fmt.Println("Starting GooglePubSubAdapter...")

	//Validate the command line flags
	flag.Usage = usage
	validateFlags()

	rand.Seed(time.Now().UnixNano())

	//Initialize the logging mechanism
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"},
		MinLevel: logutils.LogLevel(strings.ToUpper(logLevel)),
		Writer:   os.Stdout,
	}

	log.SetOutput(filter)

	//Add mqtt logging
	// logger := log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
	// mqtt.ERROR = logger
	// mqtt.CRITICAL = logger
	// mqtt.WARN = logger
	// mqtt.DEBUG = logger

	cbBroker = cbPlatformBroker{
		name:         "ClearBlade",
		clientID:     deviceName + "_client",
		client:       nil,
		platformURL:  &platformURL,
		messagingURL: &messagingURL,
		systemKey:    &sysKey,
		systemSecret: &sysSec,
		deviceName:   &deviceName,
		password:     &activeKey,
		topic:        "",
		qos:          msgSubscribeQos,
	}

	// Initialize ClearBlade Client
	if err := initCbClient(cbBroker); err != nil {
		log.Println(err.Error())
		log.Println("Unable to initialize CB broker client. Exiting.")
		return
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

// ClearBlade Client init helper
func initCbClient(platformBroker cbPlatformBroker) error {
	log.Println("[INFO] initCbClient - Initializing the ClearBlade client")

	log.Printf("[DEBUG] initCbClient - Platform URL: %s\n", *(platformBroker.platformURL))
	log.Printf("[DEBUG] initCbClient - Platform Messaging URL: %s\n", *(platformBroker.messagingURL))
	log.Printf("[DEBUG] initCbClient - System Key: %s\n", *(platformBroker.systemKey))
	log.Printf("[DEBUG] initCbClient - System Secret: %s\n", *(platformBroker.systemSecret))
	log.Printf("[DEBUG] initCbClient - Device Name: %s\n", *(platformBroker.deviceName))
	log.Printf("[DEBUG] initCbClient - Password: %s\n", *(platformBroker.password))

	cbBroker.client = cb.NewDeviceClientWithAddrs(*(platformBroker.platformURL), *(platformBroker.messagingURL), *(platformBroker.systemKey), *(platformBroker.systemSecret), *(platformBroker.deviceName), *(platformBroker.password))

	for err := cbBroker.client.Authenticate(); err != nil; {
		log.Printf("[ERROR] initCbClient - Error authenticating %s: %s\n", platformBroker.name, err.Error())
		log.Println("[ERROR] initCbClient - Will retry in 1 minute...")

		// sleep 1 minute
		time.Sleep(time.Duration(time.Minute * 1))
		err = cbBroker.client.Authenticate()
	}

	//Retrieve adapter configuration data
	log.Println("[INFO] initCbClient - Retrieving adapter configuration...")
	getAdapterConfig()

	log.Println("[INFO] initCbClient - Initializing MQTT")
	callbacks := cb.Callbacks{OnConnectionLostCallback: onConnectLost, OnConnectCallback: onConnect}
	if err := cbBroker.client.InitializeMQTTWithCallback(platformBroker.clientID+"-"+strconv.Itoa(rand.Intn(10000)), "", 30, nil, nil, &callbacks); err != nil {
		log.Fatalf("[FATAL] initCbClient - Unable to initialize MQTT connection with %s: %s", platformBroker.name, err.Error())
		return err
	}

	return nil
}

//If the connection to the broker is lost, we need to reconnect and
//re-establish all of the subscriptions
func onConnectLost(client mqtt.Client, connerr error) {
	log.Printf("[INFO] OnConnectLost - Connection to broker was lost: %s\n", connerr.Error())

	//End the existing goRoutines
	endWorkersChannel <- "Stop Channel"

	if gcpPubTopic != "" {
		endWorkersChannel <- "Stop Channel"
	}

	//We don't need to worry about manally re-initializing the mqtt client. The auto reconnect logic will
	//automatically try and reconnect. The reconnect interval could be as much as 20 minutes.

	//Auto reconnect does not appear to be working in all cases. Let's just end and let the OS restart the adapter
	// log.Printf("[INFO] OnConnectLost - Sending SIGINT: \n")
	// interruptChannel <- syscall.SIGINT
}

//When the connection to the broker is complete, set up any subscriptions
//and authenticate the google pubsub client
func onConnect(client mqtt.Client) {
	log.Println("[INFO] OnConnect - Connected to ClearBlade Platform MQTT broker")

	log.Println("[INFO] OnConnect - Authenticating to GCP")
	googleAuthExplicit(gcpProjectID, gcpCredsPath)

	//We only need to subscribe to a platform MQTT topic if we will be publishing data to GCP.
	if gcpPubTopic != "" {
		//CleanSession, by default, is set to true. This results in non-durable subscriptions.
		//We therefore need to re-subscribe
		log.Println("[INFO] OnConnect - Begin configuring platform subscription")

		var err error
		for cbSubscribeChannel, err = cbSubscribe(topicRoot + "/publish"); err != nil; {
			//Wait 30 seconds and retry
			log.Printf("[ERROR] OnConnect - Error subscribing to MQTT: %s\n", err.Error())
			log.Println("[ERROR] OnConnect - Will retry in 30 seconds...")
			time.Sleep(time.Duration(30 * time.Second))
			cbSubscribeChannel, err = cbSubscribe(topicRoot + "/publish")
		}
		//Start subscribe worker
		go cbSubscribeWorker()
	} else {
		log.Println("[INFO] OnConnect - gcpPubTopic no gcpPubTopic specified, no need to subscribe to platform topic.")
	}

	if gcpSubTopic != "" {
		log.Println("[DEBUG] OnConnect - gcpSubTopic provided, Invoking gcpSubscribe")
		gcpSubscribe()

		//Start GCP subscription pull loop
		log.Println("[DEBUG] OnConnect - Invoking gcpPullWorker")
		go gcpPullWorker()
	}

}

func cbSubscribeWorker() {
	log.Println("[INFO] subscribeWorker - Starting MQTT subscribeWorker")

	//Wait for subscriptions to be received
	for {
		select {
		case message, ok := <-cbSubscribeChannel:
			if ok {
				//Ensure a publish request was received
				if strings.HasSuffix(message.Topic.Whole, topicRoot+"/publish") {
					log.Println("[INFO] subscribeWorker - Handling GCP publish request...")
				} else {
					log.Printf("[ERROR] subscribeWorker - Unknown request received: topic = %s, payload = %#v\n", message.Topic.Whole, message.Payload)
				}
			}
		case _ = <-endWorkersChannel:
			//End the current go routine when the stop signal is received
			log.Println("[INFO] subscribeWorker - Stopping subscribeWorker")
			return
		}
	}
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

// Subscribes to a topic
func cbSubscribe(topic string) (<-chan *mqttTypes.Publish, error) {
	log.Printf("[INFO] subscribe - Subscribing to MQTT topic %s\n", topic)
	subscription, error := cbBroker.client.Subscribe(topic, cbBroker.qos)
	if error != nil {
		log.Printf("[ERROR] subscribe - Unable to subscribe to MQTT topic: %s due to error: %s\n", topic, error.Error())
		return nil, error
	}

	log.Printf("[DEBUG] subscribe - Successfully subscribed to MQTT topic %s\n", topic)
	return subscription, nil
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
		log.Printf("[DEBUG] gcpPull - Pubsub message received: %v\n", m)
		cbPublish(topicRoot, string(m.Data))
		m.Ack()
	})
	if err != context.Canceled {
		log.Printf("[ERROR] gcpPull - Error receiving pubsub messages: %s\n", err.Error())
		cbPublish(topicRoot+"/error", err.Error())
	}
}

// Publishes data to a topic
func cbPublish(topic string, data string) error {
	log.Printf("[INFO] cbPublish - Publishing to topic %s\n", topic)
	error := cbBroker.client.Publish(topic, []byte(data), cbBroker.qos)
	if error != nil {
		log.Printf("[ERROR] cbPublish - Unable to publish to topic: %s due to error: %s\n", topic, error.Error())
		return error
	}

	log.Printf("[DEBUG] publish - Successfully published message to topic %s\n", topic)
	return nil
}

func gcpPublish(topic string, data string) error {
	//TODO
	return nil
}

func getAdapterConfig() {
	log.Println("[INFO] getAdapterConfig - Retrieving adapter config")
	var settingsJSON map[string]interface{}

	//Retrieve the adapter configuration row
	query := cb.NewQuery()
	query.EqualTo("adapter_device_name", deviceName)

	//A nil query results in all rows being returned
	log.Println("[DEBUG] getAdapterConfig - Executing query against table " + adapterConfigCollection)
	results, err := cbBroker.client.GetDataByName(adapterConfigCollection, query)
	if err != nil {
		log.Println("[DEBUG] getAdapterConfig - Adapter configuration could not be retrieved. Using defaults")
		log.Printf("[ERROR] getAdapterConfig - Error retrieving adapter configuration: %s\n", err.Error())
	} else {
		if len(results["DATA"].([]interface{})) > 0 {
			log.Printf("[DEBUG] getAdapterConfig - Adapter config retrieved: %#v\n", results)
			log.Println("[INFO] getAdapterConfig - Adapter config retrieved")

			//MQTT topic root
			if results["DATA"].([]interface{})[0].(map[string]interface{})["topic_root"] != nil {
				log.Printf("[DEBUG] getAdapterConfig - Setting topicRoot to %s\n", results["DATA"].([]interface{})[0].(map[string]interface{})["topic_root"].(string))
				topicRoot = results["DATA"].([]interface{})[0].(map[string]interface{})["topic_root"].(string)
			} else {
				log.Printf("[INFO] getAdapterConfig - Topic root is nil. Using default value %s\n", topicRoot)
			}

			//adapter_settings
			log.Println("[DEBUG] getAdapterConfig - Retrieving adapter settings...")
			if results["DATA"].([]interface{})[0].(map[string]interface{})["adapter_settings"] != nil {
				if err := json.Unmarshal([]byte(results["DATA"].([]interface{})[0].(map[string]interface{})["adapter_settings"].(string)), &settingsJSON); err != nil {
					log.Printf("[ERROR] getAdapterConfig - Error while unmarshalling json: %s. Defaulting all adapter settings.\n", err.Error())
				}
			} else {
				log.Println("[INFO] applyAdapterConfig - Settings are nil. Defaulting all adapter settings.")
			}
		} else {
			log.Println("[INFO] getAdapterConfig - No rows returned. Using defaults")
		}
	}

	if settingsJSON == nil {
		settingsJSON = make(map[string]interface{})
	}

	applyAdapterSettings(settingsJSON)
}

func applyAdapterSettings(adapterSettings map[string]interface{}) {
	//GCP Project ID
	if adapterSettings["gcpProjectID"] != nil && adapterSettings["gcpProjectID"] != "" {
		gcpProjectID = adapterSettings["gcpProjectID"].(string)
		log.Printf("[DEBUG] applyAdapterConfig - Setting gcpProjectID to %s\n", gcpProjectID)
	} else {
		panic("gcpProjectID not provided. Authentication to GCP is not possible.")
	}

	//GCP Credentials file path
	if adapterSettings["gcpCredsPath"] != nil && adapterSettings["gcpCredsPath"] != "" {
		gcpCredsPath = adapterSettings["gcpCredsPath"].(string)
		log.Printf("[DEBUG] applyAdapterConfig - Setting gcpCredsPath to %s\n", gcpCredsPath)
	} else {
		panic("gcpCredsPath not provided. Authentication to GCP is not possible.")
	}

	//gcpPubTopic
	if adapterSettings["gcpPubTopic"] != nil {
		gcpPubTopic = adapterSettings["gcpPubTopic"].(string)
		log.Printf("[DEBUG] applyAdapterConfig - Setting gcpPubTopic to %s\n", gcpPubTopic)
	} else {
		log.Printf("[INFO] applyAdapterSettings - A gcpPubTopic value was not found.\n")
	}

	//gcpSubTopic
	if adapterSettings["gcpSubTopic"] != nil {
		gcpSubTopic = adapterSettings["gcpSubTopic"].(string)
		log.Printf("[DEBUG] applyAdapterConfig - Setting gcpSubTopic to %s\n", gcpSubTopic)
	} else {
		log.Printf("[INFO] applyAdapterSettings - A gcpPubTopic value was not found.\n")
	}

	//gcpPullInterval
	if adapterSettings["gcpPullInterval"] != nil {
		log.Printf("[DEBUG] applyAdapterConfig - Setting gcpPullInterval to %d\n", gcpPullInterval)
		gcpPullInterval = int(adapterSettings["gcpPullInterval"].(float64))
	} else {
		log.Printf("[INFO] applyAdapterSettings - A gcpPullInterval value was not found.\n")
	}

	//gcpSubPreCreated
	if adapterSettings["gcpSubPreCreated"] != nil {
		log.Printf("[DEBUG] applyAdapterConfig - Setting gcpSubPreCreated to %t\n", gcpSubPreCreated)
		gcpSubPreCreated = adapterSettings["gcpSubPreCreated"].(bool)
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
		log.Println("[ERROR] googleAuthExplicit - Error authenticating to GCP")
		log.Fatal(err)
	}
	log.Println("[INFO] googleAuthExplicit - Authentication to GCP successful")
}
