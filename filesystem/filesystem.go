package main

import (
	"github.com/sisoputnfrba/tp-golang/filesystem/utils"

	"github.com/sisoputnfrba/tp-golang/utils/logging"
)

func main() {

	// Inicio configs
	utils.Configs = utils.Iniciar_configuracion("config-pd.json")

	// Inicio log
	logger := logging.Iniciar_Logger("filesystem.log", utils.Configs.LogLevel)

	// Inicializar estructura filesystem
	utils.Inicializar_Estructura_Filesystem(logger)

	// Iniciar filesystem como server
	utils.Iniciar_fileSystem(logger)

}
