package schema

// IntegrationSchema is the json schema for a protocol version 2 integration
var IntegrationSchema = `{
  "$id": "http://example.com/example.json",
  "type": "object",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "properties": {
    "name": {
      "$id": "/properties/name",
      "type": "string",
      "examples": [
        "com.newrelic.kubernetes"
      ]
    },
    "protocol_version": {
      "$id": "/properties/protocol_version",
      "type": "string",
      "enum": ["2"]
    },
    "integration_version": {
      "$id": "/properties/integration_version",
      "type": "string",
      "examples": [
        "1.0.0-beta2.2"
      ]
    },
    "data": {
      "$id": "/properties/data",
      "type": "array",
      "minItems": 1,
      "items": {
        "$id": "/properties/data/items",
        "type": "object",
        "properties": {
          "entity": {
            "$id": "/properties/data/items/properties/entity",
            "type": "object",
            "properties": {
              "name": {
                "$id": "/properties/data/items/properties/entity/properties/name",
                "type": "string",
                "examples": [
                  "kube-addon-manager-minikube"
                ]
              },
              "type": {
                "$id": "/properties/data/items/properties/entity/properties/type",
                "type": "string",
                "examples": [
                  "k8s:sergio-minikube:kube-system:pod"
                ]
              }
            }
          },
          "metrics": {
            "$id": "/properties/data/items/properties/metrics",
            "type": "array"
            }
          },
          "inventory": {
            "$id": "/properties/data/items/properties/inventory",
            "type": "object"
          },
          "events": {
            "$id": "/properties/data/items/properties/events",
            "type": "array"
          }
        }
      }
    },
    "required": [
      "name",
      "protocol_version",
      "integration_version"
    ]
  }
}`
