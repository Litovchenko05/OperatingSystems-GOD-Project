package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/sisoputnfrba/tp-golang/kernel/planificador"
	"github.com/sisoputnfrba/tp-golang/kernel/server"
	"github.com/sisoputnfrba/tp-golang/kernel/utils"
	"github.com/sisoputnfrba/tp-golang/utils/logging"
)

func main() {

	// Inicializamos la configuracion y el logger
	utils.Configs = utils.Iniciar_Configuracion("config-pd.json")
	logger := logging.Iniciar_Logger("kernel.log", utils.Configs.LogLevel)

	// Inicializamos las colas de procesos
	planificador.Inicializar_colas()

	// Inicializamos el planificador
	planificador.Iniciar_planificador(utils.Configs, logger)

	// Inicializamos el mapa de PCBs
	utils.InicializarPCBMapGlobal()

	// Obtener los parametros del primer proceso a ejecutar
	archivoPseudocodigo := os.Args[1]
	tamanioProceso, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("Error: El tamaño del proceso debe ser un número entero.")
		panic(err)
	}

	// Creación del proceso inicial
	planificador.Crear_proceso(archivoPseudocodigo, tamanioProceso, 0, logger)

	// Inicializamos la cola de IO
	go planificador.Procesar_cola_IO(&planificador.ColaIO, logger)

	// Iniciamos Kernel como server
	server.Iniciar_kernel(logger)
}
