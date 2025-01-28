#!/usr/bin/env bash

# Pull the official RabbitMQ image (if not already present).
docker pull rabbitmq:3-management

# Run RabbitMQ with the management plugin enabled.
#  - Name the container "my-rabbitmq".
#  - Bind the standard AMQP port (5672) and the management UI port (15672).
docker run -d \
  --name my-rabbitmq \
  -p 5672:5672 \
  -p 15672:15672 \
  rabbitmq:3-management
