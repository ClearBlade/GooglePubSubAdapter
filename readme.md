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

### Environment Variables
The Google PubSub Adapter is dependant upon certain environment variables prior to attempting to execute the adapter. The environment variables that need to be created are as follows:

  __CB_SYSTEM_KEY__
  * Optional, if no environment variable is created, use the _systemKey_ command line flag
  
  __CB_SYSTEM_SECRET__
  * Optional, if no environment variable is created, use the _systemSecret_ command line flag

  __CB_SERVICE_ACCOUNT__
  * REQUIRED
  * The name of the device defined within the ClearBlade Platform or ClearBlade Edge representing the adapter

  __CB_SERVICE_ACCOUNT_TOKEN__
  * REQUIRED
  * The authentication token of the device defined within the ClearBlade Platform or ClearBlade Edge representing the adapter

Environment variables can be created on Linux based systems by executing the following command from a terminal prompt:
`export [variable_name]=[variable_value]`

_example_
`export CB_SERVICE_ACCOUNT=My_Device_Name`
`export CB_SERVICE_ACCOUNT_TOKEN=MyDeviceToken`


### Executing the adapter

`GooglePubSubAdapter -systemKey=<SYSTEM_KEY> -systemSecret=<SYSTEM_SECRET> -platformURL=<PLATFORM_URL> -messagingURL=<MESSAGING_URL> -adapterConfigCollection=<COLLECTION_NAME> -logLevel=<LOG_LEVEL>`

   __*Where*__ 

   __systemKey__
  * OPTIONAL
  * Can be set up as an environment variable with the name CB_SYSTEM_KEY
  * The system key of the ClearBLade Platform __System__ the adapter will connect to

   __systemSecret__
  * OPTIONAL
  * Can be set up as an environment variable with the name CB_SYSTEM_SECRET
  * The system secret of the ClearBLade Platform __System__ the adapter will connect to
         
   __platformURL__
  * The url of the ClearBlade Platform instance the adapter will connect to
  * OPTIONAL
  * Defaults to __http://localhost:9000__

   __messagingURL__
  * The MQTT url of the ClearBlade Platform instance the adapter will connect to
  * OPTIONAL
  * Defaults to __localhost:1883__

   __adapterConfigCollection__
  * The name of the data collection used to house adapter configuration data
  * OPTIONAL 
  * Defaults to __adapter_config__

   __logLevel__
  * The level of runtime logging the adapter should provide.
  * Available log levels:
    * FATAL
    * ERROR
    * INFO
    * DEBUG
  * OPTIONAL
  * Defaults to __info__


## Setup
---
The __GooglePubSubAdapter__ adapter is dependent upon the ClearBlade Go SDK and its dependent libraries being installed as well as the GCloud pubsub SDK. The __GooglePubSubAdapter__ adapter was written in Go and therefore requires Go to be installed (https://golang.org/doc/install).


### Adapter compilation
In order to compile the adapter for execution, the following steps need to be performed:

 1. Retrieve the adapter source code  
    * ```git clone git@github.com:ClearBlade/GooglePubSubAdapter.git```
 2. Navigate to the __GooglePubSubAdapter__ directory  
    * ```cd GooglePubSubAdapter```
 3. ```go get -u github.com/ClearBlade/Go-SDK.git```
    * This command should be executed from within your Go workspace
 4. ```go get -u cloud.google.com/go/pubsub```
    * This command should be executed from within your Go workspace
 5. Compile the adapter
    * ```go build```



