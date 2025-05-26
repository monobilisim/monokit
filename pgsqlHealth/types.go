// This file defines the types used in the pgsqlHealth package
//
// It provides the following types:
// - Response: Represents the JSON response from the Patroni API
// - Member: Represents a member of the Patroni cluster
// - Service: Represents a service in the Consul catalog
package pgsqlHealth

import "database/sql"

var Connection *sql.DB
var nodeName string

type Response struct {
	Members []Member `json:"members"`
	Pause   bool     `json:"pause"`
	Scope   string   `json:"scope"`
}

type Member struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	State    string `json:"state"`
	APIURL   string `json:"api_url"`
	Host     string `json:"host"`
	Port     int64  `json:"port"`
	Timeline int64  `json:"timeline"`
}

type Service struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	State  string `json:"state"`
	Host   string `json:"host"`
	Port   int64  `json:"port"`
	APIURL string `json:"api_url"`
}

type ConsulCatalog struct {
	ServiceName string    `json:"service_name"`
	Services    []Service `json:"services"`
}

type ConsulMember struct {
	Node      string `json:"node"`
	Address   string `json:"address"`
	Status    string `json:"status"`
	ServiceID string `json:"service_id"`
}
