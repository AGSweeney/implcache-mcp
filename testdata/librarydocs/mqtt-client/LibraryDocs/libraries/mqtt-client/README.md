---
title: mqtt-client
component: mqtt-client
level: library
status: verified
evidence: E1
topics: [mqtt, publish]
source_paths: [src/mqtt_client.go]
retrieval:
  questions:
    - How do I connect an MQTT client?
    - How do I publish a message?
---

# mqtt-client

Use `Connect` then `Publish` for the shared MQTT helper.

## Connect

Call `Connect(broker)` before publishing.

## Publish

`Publish(topic, payload)` sends bytes to the broker.
