package models

import "time"

type Message struct {
	Username string
	Message  string
	PubTime  time.Time
	RoomID   string
	AuthUID  string `json:"auth_uid"`
	PhotoURI string `json:"photo_uri"`
}