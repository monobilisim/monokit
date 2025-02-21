// This file defines the types used in the pgsqlHealth package
//
// It provides the following types:
// - Response: Represents the JSON response from the Patroni API
// - Member: Represents a member of the Patroni cluster
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