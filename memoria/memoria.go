package main

import (
	"github.com/sisoputnfrba/tp-golang/memoria/memUsuario"
	"github.com/sisoputnfrba/tp-golang/memoria/server"
	"github.com/sisoputnfrba/tp-golang/memoria/utils"
	"github.com/sisoputnfrba/tp-golang/utils/logging"
)

func main() {
	// Inicializamos la configuracion y el logger
	utils.Configs = utils.Iniciar_configuracion("config-pd.json")
	logger := logging.Iniciar_Logger("memoria.log", utils.Configs.LogLevel)

	// Inicializacion de memoria de usuario
	esquema := utils.Configs.Scheme
	if esquema == "FIJAS" {
		memUsuario.Inicializar_Memoria_De_Usuario(logger)
	} else if esquema == "DINAMICAS" {
		memUsuario.Inicializar_Memoria_Dinamica(logger)
	} else {
		logger.Info("mal definido el esquema de particiones")
	}

	// Inicializamos la memoria (Lo levantamos como servidor)
	server.Iniciar_memoria(logger)
}
