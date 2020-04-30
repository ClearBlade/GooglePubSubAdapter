# Docker Image Creation

## Prerequisites

- Building the image requires internet access

### Creating the Docker image for the AMQP Adapter

Clone this repository and execute the following commands to create a docker image for the GooglePubSub adapter:  

- ```GOOS=linux GOARCH=amd64 go build```
- ```cd GooglePubSubAdapter/Docker_Build```
- ```docker build -f Dockerfile -t clearblade_gcp_pubsub_adapter ..```


# Using the adapter

## Deploying the adapter image

When the docker image has been created, it will need to be saved and imported into the runtime environment. Execute the following steps to save and deploy the adapter image

- On the machine where the ```docker build``` command was executed, execute ```docker save clearblade_gcp_pubsub_adapter:latest > gcp_pubsub_adapter.tar``` 

- On the server where docker is running, execute ```docker load -i gcp_pubsub_adapter.tar```

## Executing the adapter

Once you create the docker image, start the GooglePubSub adapter using the following command:


```docker run -d --name GooglePubSubAdapter --network cb-net -v <host_creds_path>:<container_creds_path> --restart always -e CB_SERVICE_ACCOUNT=<YOUR_DEVICE_NAME> -e CB_SERVICE_ACCOUNT_TOKEN=<DEVICE_ACTIVE_KEY> -e CB_SYSTEM_KEY=<YOUR_SYSTEMKEY> -e CB_SYSTEM_SECRET=<YOUR_SYSTEMSECRET> clearblade_gcp_pubsub_adapter --platformURL <YOUR_PLATFORMURL> --messagingURL <YOUR_MESSAGINGURL>```

### Environment Variables

__CB_SERVICE_ACCOUNT__ - The name of a device created on the ClearBlade Platform
__CB_SERVICE_ACCOUNT_TOKEN__ - The active key of a device created on the ClearBlade Platform
__CB_SYSTEM_KEY__ - The System Key of your System on the ClearBlade Platform
__CB_SYSTEM_SECRET__ - The System Secret of your System on the ClearBlade Platform

### Command Line Arguments

```
--platformURL The address of the ClearBlade Platform (ex. https://platform.clearblade.com)
--messagingURL The MQTT broker address (ex. platform.clearblade.com:1883)
```

Ex.
```docker run -d --name GooglePubSubAdapter --network cb-net -v ~/mycreds/gcp_creds.json:/usr/local/etc/pubsub/gcp_creds.json:ro --restart always -e CB_SERVICE_ACCOUNT=gcpPubSubAdapter -e CB_SERVICE_ACCOUNT_TOKEN=01234567890 -e CB_SYSTEM_KEY=cc9d8bba0bfeeed78595c4dfbb0b -e CB_SYSTEM_SECRET=CC9D8BBA0BB4F1C5AD8994E6D41B clearblade_gcp_pubsub_adapter --platformURL https://platform.clearblade.com --messagingURL platform.clearblade.com:1883```
