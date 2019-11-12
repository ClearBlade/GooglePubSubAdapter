# GooglePubSubAdapter Adapter

The __GooglePubSubAdapter__ adapter provides the ability for the ClearBlade platform to communicate with a the PubSub functionality within the Google Cloud Platform. 

# MQTT Topic Structure
The __GooglePubSubAdapter__ adapter utilizes MQTT messaging to communicate with the ClearBlade Platform. The __GooglePubSubAdapter__ adapter will subscribe to a specific topic in order to handle requests from the ClearBlade Platform/Edge to publish data to Cloud Pub/Sub. Additionally, the __GooglePubSubAdapter__ adapter will publish messages to MQTT topics in order to provide the ClearBlade Platform/Edge with data received from a Cloud Pub/Sub topic subscription. The topic structures utilized by the __GooglePubSubAdapter__ adapter are as follows:

  * Publish data to Cloud Pub/Sub: {__TOPIC ROOT__}/publish
  * Send Cloud Pub/Sub subscription data to Clearblade: {__TOPIC ROOT__}/receive/response
  * Adapter error conditions encountered: {__TOPIC ROOT__}/error


## ClearBlade Platform Dependencies
The __GooglePubSubAdapter__ adapter was constructed to provide the ability to communicate with a _System_ defined in a ClearBlade Platform instance. Therefore, the adapter requires a _System_ to have been created within a ClearBlade Platform instance.

Once a System has been created, artifacts must be defined within the ClearBlade Platform system to allow the adapters to function properly. At a minimum: 

  * A device needs to be created in the Auth --> Devices collection. The device will represent the adapter account. The _name_ and _active key_ values specified in the Auth --> Devices collection will be used by the adapter to authenticate to the ClearBlade Platform or ClearBlade Edge. 
  * An adapter configuration data collection needs to be created in the ClearBlade Platform _system_ and populated with the data appropriate to the __GooglePubSubAdapter__ adapter. The schema of the data collection should be as follows:


| Column Name      | Column Datatype |
| ---------------- | --------------- |
| adapter_name     | string          |
| topic_root       | string          |
| adapter_settings | string (json)   |

### adapter_settings
The adapter_settings column will need to contain a JSON object containing the following attributes:

##### gcpProjectID
* The ID of the GCP Project to connect to

##### gcpCredsPath
* The path to the json credential file used to authenticate with the Google Cloud Platform

##### gcpPubTopic
* The GCP Cloud PubSub topic the adapter should publish data to
* Optional, omit from settings object if you will not be publishing data to GCP Cloud Pub Sub

##### gcpSubTopic
* The GCP Cloud PubSub topic the adapter should subscribe to
* Optional, omit from settings object if you will not be subscribing to any GCP Cloud Pub Sub topic

##### gcpPullInterval
* The number of seconds to wait between each attempt to pull data from GCP Cloud Pub Sub

##### gcpSubPreCreated
* Boolean value that indicates whether or not the GCP Pub Sub subscription was pre-created in GCP
* If false, the adapter will attempt to create a subscription on GCP. This requires pubsub.editor permissions on GCP

#### adapter_settings_example
{
  "gcpProjectID": "MyGcpProjectID",
  "gcpCredsPath": "/var/my_service_account_creds.json",
  "gcpPubTopic": "",
  "gcpSubTopic": "myTopic",
  "gcpPullInterval": 60,
  "gcpSubPreCreated": true
}

## Usage

### Executing the adapter

`GooglePubSubAdapter -systemKey=<SYSTEM_KEY> -systemSecret=<SYSTEM_SECRET> -platformURL=<PLATFORM_URL> -messagingURL=<MESSAGING_URL> -deviceName=<DEVICE_NAME> -password=<DEVICE_ACTIVE_KEY> -adapterConfigCollectionID=<COLLECTION_ID> -logLevel=<LOG_LEVEL>`

   __*Where*__ 

   __systemKey__
  * REQUIRED
  * The system key of the ClearBLade Platform __System__ the adapter will connect to

   __systemSecret__
  * REQUIRED
  * The system secret of the ClearBLade Platform __System__ the adapter will connect to
   
   __deviceName__
  * The device name the adapter will use to authenticate to the ClearBlade Platform
  * Requires the device to have been defined in the _Auth - Devices_ collection within the ClearBlade Platform __System__
  * OPTIONAL
  * Defaults to __gcpPubSubAdapter__
   
   __password__
  * REQUIRED
  * The active key the adapter will use to authenticate to the platform
  * Requires the device to have been defined in the _Auth - Devices_ collection within the ClearBlade Platform __System__
   
   __platformUrl__
  * The url of the ClearBlade Platform instance the adapter will connect to
  * OPTIONAL
  * Defaults to __http://localhost:9000__

   __messagingUrl__
  * The MQTT url of the ClearBlade Platform instance the adapter will connect to
  * OPTIONAL
  * Defaults to __localhost:1883__

   __adapterConfigCollectionID__
  * REQUIRED 
  * The collection ID of the data collection used to house adapter configuration data

   __logLevel__
  * The level of runtime logging the adapter should provide.
  * Available log levels:
    * fatal
    * error
    * warn
    * info
    * debug
  * OPTIONAL
  * Defaults to __info__


## Setup
---
The __GooglePubSubAdapter__ adapter is dependent upon the ClearBlade Go SDK and its dependent libraries being installed as well as the GCloud pubsub SDK. The __GooglePubSubAdapter__ adapter was written in Go and therefore requires Go to be installed (https://golang.org/doc/install).


### Adapter compilation
In order to compile the adapter for execution, the following steps need to be performed:

 1. Retrieve the adapter source code  
    * ```git clone git@github.com:ClearBlade/GooglePubSubAdapter.git```
 2. Navigate to the xdotadapter directory  
    * ```cd GooglePubSubAdapter```
 3. ```git clone https://github.com/ClearBlade/Go-SDK.git```
    * This command should be executed from within your Go workspace
 3. ```go get -u cloud.google.com/go/pubsub```
 4. Compile the adapter
    * ```go build```



