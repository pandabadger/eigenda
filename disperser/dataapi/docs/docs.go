// Package docs Code generated by swaggo/swag. DO NOT EDIT
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/feed/blobs": {
            "get": {
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Feed"
                ],
                "summary": "Fetch blobs metadata list",
                "parameters": [
                    {
                        "type": "integer",
                        "description": "Limit [default: 10]",
                        "name": "limit",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/dataapi.BlobsResponse"
                        }
                    },
                    "400": {
                        "description": "error: Bad request",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    },
                    "404": {
                        "description": "error: Not found",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "error: Server error",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/feed/blobs/{blob_key}": {
            "get": {
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Feed"
                ],
                "summary": "Fetch blob metadata by blob key",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Blob Key",
                        "name": "blob_key",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/dataapi.BlobMetadataResponse"
                        }
                    },
                    "400": {
                        "description": "error: Bad request",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    },
                    "404": {
                        "description": "error: Not found",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "error: Server error",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/metrics": {
            "get": {
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Metrics"
                ],
                "summary": "Fetch metrics",
                "parameters": [
                    {
                        "type": "integer",
                        "description": "Start unix timestamp [default: 1 hour ago]",
                        "name": "start",
                        "in": "query"
                    },
                    {
                        "type": "integer",
                        "description": "End unix timestamp [default: unix time now]",
                        "name": "end",
                        "in": "query"
                    },
                    {
                        "type": "integer",
                        "description": "Limit [default: 10]",
                        "name": "limit",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/dataapi.Metric"
                        }
                    },
                    "400": {
                        "description": "error: Bad request",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    },
                    "404": {
                        "description": "error: Not found",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "error: Server error",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/metrics/non_signers": {
            "get": {
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Metrics"
                ],
                "summary": "Fetch non signers",
                "parameters": [
                    {
                        "type": "integer",
                        "description": "Interval to query for non signers in seconds [default: 3600]",
                        "name": "interval",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "array",
                            "items": {
                                "$ref": "#/definitions/dataapi.NonSigner"
                            }
                        }
                    },
                    "400": {
                        "description": "error: Bad request",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    },
                    "404": {
                        "description": "error: Not found",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "error: Server error",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/metrics/throughput": {
            "get": {
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Metrics"
                ],
                "summary": "Fetch throughput time series",
                "parameters": [
                    {
                        "type": "integer",
                        "description": "Start unix timestamp [default: 1 hour ago]",
                        "name": "start",
                        "in": "query"
                    },
                    {
                        "type": "integer",
                        "description": "End unix timestamp [default: unix time now]",
                        "name": "end",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "array",
                            "items": {
                                "$ref": "#/definitions/dataapi.Throughput"
                            }
                        }
                    },
                    "400": {
                        "description": "error: Bad request",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    },
                    "404": {
                        "description": "error: Not found",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "error: Server error",
                        "schema": {
                            "$ref": "#/definitions/dataapi.ErrorResponse"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "core.BlobCommitments": {
            "type": "object",
            "properties": {
                "commitment": {
                    "$ref": "#/definitions/core.Commitment"
                },
                "length": {
                    "type": "integer"
                },
                "length_proof": {
                    "$ref": "#/definitions/core.Commitment"
                }
            }
        },
        "core.Commitment": {
            "type": "object",
            "properties": {
                "x": {
                    "type": "array",
                    "items": {
                        "type": "integer"
                    }
                }
            }
        },
        "core.SecurityParam": {
            "type": "object",
            "properties": {
                "adversary_threshold": {
                    "description": "AdversaryThreshold is the maximum amount of stake that can be controlled by an adversary in the quorum as a percentage of the total stake in the quorum",
                    "type": "integer"
                },
                "quorum_id": {
                    "type": "integer"
                },
                "quorum_rate": {
                    "description": "Rate Limit. This is a temporary measure until the node can derive rates on its own using rollup authentication. This is used\nfor restricting the rate at which retrievers are able to download data from the DA node to a multiple of the rate at which the\ndata was posted to the DA node.",
                    "type": "integer"
                },
                "quorum_threshold": {
                    "description": "QuorumThreshold is the amount of stake that must sign a message for it to be considered valid as a percentage of the total stake in the quorum",
                    "type": "integer"
                }
            }
        },
        "dataapi.BlobMetadataResponse": {
            "type": "object",
            "properties": {
                "batch_header_hash": {
                    "type": "string"
                },
                "batch_id": {
                    "type": "integer"
                },
                "batch_root": {
                    "type": "string"
                },
                "blob_commitment": {
                    "$ref": "#/definitions/core.BlobCommitments"
                },
                "blob_inclusion_proof": {
                    "type": "string"
                },
                "blob_index": {
                    "type": "integer"
                },
                "blob_key": {
                    "type": "string"
                },
                "blob_status": {
                    "$ref": "#/definitions/github_com_Layr-Labs_eigenda_disperser.BlobStatus"
                },
                "confirmation_block_number": {
                    "type": "integer"
                },
                "fee": {
                    "type": "string"
                },
                "reference_block_number": {
                    "type": "integer"
                },
                "requested_at": {
                    "type": "integer"
                },
                "security_params": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/core.SecurityParam"
                    }
                },
                "signatory_record_hash": {
                    "type": "string"
                }
            }
        },
        "dataapi.BlobsResponse": {
            "type": "object",
            "properties": {
                "data": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/dataapi.BlobMetadataResponse"
                    }
                },
                "meta": {
                    "$ref": "#/definitions/dataapi.Meta"
                }
            }
        },
        "dataapi.ErrorResponse": {
            "type": "object",
            "properties": {
                "error": {
                    "type": "string"
                }
            }
        },
        "dataapi.Meta": {
            "type": "object",
            "properties": {
                "size": {
                    "type": "integer"
                }
            }
        },
        "dataapi.Metric": {
            "type": "object",
            "properties": {
                "cost_in_wei": {
                    "type": "integer"
                },
                "throughput": {
                    "type": "number"
                },
                "total_stake": {
                    "type": "integer"
                }
            }
        },
        "dataapi.NonSigner": {
            "type": "object",
            "properties": {
                "count": {
                    "type": "integer"
                },
                "operatorId": {
                    "type": "string"
                }
            }
        },
        "dataapi.Throughput": {
            "type": "object",
            "properties": {
                "throughput": {
                    "type": "number"
                },
                "timestamp": {
                    "type": "integer"
                }
            }
        },
        "github_com_Layr-Labs_eigenda_disperser.BlobStatus": {
            "type": "integer",
            "enum": [
                0,
                1,
                2,
                3,
                4
            ],
            "x-enum-varnames": [
                "Processing",
                "Confirmed",
                "Failed",
                "Finalized",
                "InsufficientSignatures"
            ]
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "1",
	Host:             "",
	BasePath:         "",
	Schemes:          []string{"https", "http"},
	Title:            "EigenDA Data Access API",
	Description:      "This is the EigenDA Data Access API server.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
