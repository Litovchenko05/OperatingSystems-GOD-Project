package utils

import (
	"encoding/json"
	"log"
	"os"
)

// * Variable global para controlar el ciclo de ejecución
var Control = true

type Config struct {
	IpMemory   string `json:"ip_memory"`
	PortMemory int    `json:"port_memory"`
	IpKernel   string `json:"ip_kernel"`
	PortKernel int    `json:"port_kernel"`
	Port       int    `json:"port"`
	LogLevel   string `json:"log_level"`
}

var Configs Config // Variable global dentro del package

func Iniciar_configuracion(filePath string) Config {

	configFile, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&Configs)

	return Configs
}
