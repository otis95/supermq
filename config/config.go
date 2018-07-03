package config

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/ini.v1"
)

//mqtt_server = tcp://0.0.0.0:1883
//tlsmqtt_server = tcp://0.0.0.0:1884
//wsmqtt_server = 0.0.0.0:8083
//wssmqtt_server = 0.0.0.0:8084
//web_server = 0.0.0.0:11883
//cert_path = certs/michael.crt
//key_path = certs/michael.key

type Config struct {
	//MQTT
	MQTTServer    string `ini:"mqtt_server"`
	TlsMQTTServer string `ini:"tlsmqtt_server"`
	WSMQTTServer  string `ini:"wsmqtt_server"`
	WSSMQTTServer string `ini:"wssmqtt_server"`
	WebServer     string `ini:"web_server"`
	CertPath      string `ini:"cert_path"`
	KeyPath       string `ini:"key_path"`

	LogLevel    string `ini:"log_level"`
	LogDirWin   string `ini:"log_dir_win"`
	LogDirLinux string `ini:"log_dir_linux"`
	LogPrefix   string `ini:"log_prefix"`
	LogToFile   bool   `ini:"log_to_file"`

	RouteModel bool     `ini:"route_model"`
	RouteId    string   `ini:"route_id"`
	Host       string   `ini:"host"`
	Port       int      `ini:"port"`
	Ip         string   `ini:"ip"`
	Routes     []string `ini:"routes"`
}

func (c Config) String() string {
	mqtt1 := fmt.Sprintf("MQTT:[%v]/[%v]", c.MQTTServer, c.TlsMQTTServer)
	mqtt2 := fmt.Sprintf("MQTT:[%v]/[%v]", c.WSMQTTServer, c.WSSMQTTServer)
	mqtt3 := fmt.Sprintf("MQTT:[%v]/[%v]/[%v]", c.WebServer, c.CertPath, c.KeyPath)

	log := fmt.Sprintf("LOG:[win:%v]/[linux:%v]:[prefix:%v]:[ToFile:%v]:[LogLevel:%v]", c.LogDirWin, c.LogDirLinux, c.LogPrefix, c.LogToFile, c.LogLevel)

	return mqtt1 + ", " + mqtt2 + ", " + mqtt3 + ", " + log
}

//Read Server's Config Value from "path"
func ReadConfig(path string) (Config, error) {
	var config Config
	conf, err := ini.Load(path)
	if err != nil {
		log.Println("load config file fail!")
		return config, err
	}
	conf.BlockMode = false
	err = conf.MapTo(&config)
	if err != nil {
		log.Println("mapto config file fail!")
		return config, err
	}
	return config, nil
}
