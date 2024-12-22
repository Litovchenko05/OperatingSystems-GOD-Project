package cicloDeInstruccion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/sisoputnfrba/tp-golang/cpu/client"
	"github.com/sisoputnfrba/tp-golang/cpu/cpuInstruction"
	"github.com/sisoputnfrba/tp-golang/cpu/utils"
	"github.com/sisoputnfrba/tp-golang/utils/types"
)

// ?                       VARIABLES GLOBALES                    //

// * Variable global para almacenar PID y TID
var GlobalPIDTID types.PIDTID

var AnteriorPIDTID types.PIDTID

// * Variable global para almacenar la instrucción obtenida
var Instruccion string

// * Función global que representa el estado de los registros de la CPU
var ContextoEjecucion types.RegCPU

// * Variable global para almacenar la información de interrupción
var InterrupcionRecibida *types.InterruptionInfo

var PCpaqueande uint32

/////////////////////////////////////////////////////////////////////

func Comenzar_cpu(logger *slog.Logger) {

	// Log de inicio de la CPU
	logger.Info("Iniciando Ejecucion de CPU")
	logger.Info(fmt.Sprintf("## TID: %d - Solicito Contexto Ejecución", GlobalPIDTID.TID))
	if client.SolicitarContextoEjecucion(GlobalPIDTID, logger) == nil {

		for {
			if !utils.Control {
				break
			}
			// Obtener el valor actual del PC antes de Fetch
			pcActual := client.ReceivedContextoEjecucion.PC
			PCpaqueande = client.ReceivedContextoEjecucion.PC

			// 1. Fetch: obtener la próxima instrucción desde Memoria basada en el PC (Program Counter)
			err := Fetch(GlobalPIDTID.TID, GlobalPIDTID.PID, logger)
			if err != nil {
				logger.Error("Error en Fetch: ", slog.Any("error", err))
				break // Salimos del ciclo si hay error en Fetch
			}

			// Si no hay más instrucciones, salir del ciclo
			if Instruccion == "" {
				logger.Info("No hay más instrucciones. Ciclo de ejecución terminado.")
				break
			}

			// 2. Decode: interpretar la instrucción obtenida
			Decode(Instruccion, logger)

			// 3. Execute: ejecutar la instrucción decodificada (esta dentro de Decode)

			if !utils.Control {
				break
			}

			// 4. Chequear interrupciones
			CheckInterrupt(GlobalPIDTID.TID, GlobalPIDTID.PID, logger)

			// Si el PC no fue modificado por alguna instrucción, lo incrementamos en 1
			if client.ReceivedContextoEjecucion.PC == pcActual {
				client.ReceivedContextoEjecucion.PC++
				logger.Info(fmt.Sprintf("Actualizado PC a: %d", client.ReceivedContextoEjecucion.PC))
			} else {
				logger.Info(fmt.Sprintf("PC modificado por instrucción a: %d", client.ReceivedContextoEjecucion.PC))
			}

		}
		logger.Info("Fin de ciclo de CPU.")
	}
}

//! /////////////////////////////////////////////////////////////////////////////
//////////////////!               FETCH                /////////////////////////
//! //////////////////////////////////////////////////////////////////////////////

// Función Fetch para obtener la próxima instrucción
func Fetch(tid uint32, pid uint32, logger *slog.Logger) error {
	if client.ReceivedContextoEjecucion == nil {
		logger.Error("No se ha recibido el contexto de ejecución. Imposible realizar Fetch.")
		return fmt.Errorf("contexto de ejecución no disponible")
	}

	// Obtener el valor del PC (Program Counter) de la variable global
	pc := client.ReceivedContextoEjecucion.PC

	// Crear la estructura de solicitud
	requestData := struct {
		PC  uint32 `json:"pc"`
		TID uint32 `json:"tid"`
		PID uint32 `json:"pid"`
	}{PC: pc, TID: tid, PID: pid}

	// Serializar los datos en JSON
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		logger.Error("Error al codificar PC y TID a JSON: ", slog.Any("error", err))
		return err
	}

	// Crear la URL del módulo de Memoria
	url := fmt.Sprintf("http://%s:%d/instruccion", utils.Configs.IpMemory, utils.Configs.PortMemory)

	// Crear la solicitud POST
	req, err := http.NewRequest("GET", url, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("Error al crear la solicitud: ", slog.Any("error", err))
		return err
	}

	// Establecer el encabezado de la solicitud
	req.Header.Set("Content-Type", "application/json")

	// Enviar la solicitud
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Error al enviar la solicitud de Fetch: ", slog.Any("error", err))
		return err
	}
	defer resp.Body.Close()

	// Verificar si la respuesta fue exitosa
	if resp.StatusCode != http.StatusOK {
		logger.Error(fmt.Sprintf("Error en la respuesta de Fetch: Código de estado %d", resp.StatusCode))
		return fmt.Errorf("error en la respuesta de Fetch: Código de estado %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// fmt.Println("Error al leer el cuerpo de la respuesta:", err)
		return err
	}

	// Convertir a string
	bodyString := string(bodyBytes)

	// Guardar la instrucción en la variable global
	Instruccion = bodyString

	// Log de Fetch exitoso
	logger.Info(fmt.Sprintf("## TID: %d - FETCH - Program Counter: %d", tid, pc))

	return nil
}

//! ///////////////////////////////////////////////////////////////////////////////
//! /////////////////               DECODE                /////////////////////////
//! ///////////////////////////////////////////////////////////////////////////////

func Decode(instruccion string, logger *slog.Logger) {
	logger.Info(fmt.Sprintf("Decodificando la instrucción: %s", instruccion))

	// Separar la instrucción en partes, suponiendo que esté en formato "INSTRUCCION ARGUMENTOS" ej: SET AX 5
	partes := strings.Fields(instruccion)
	if len(partes) == 0 {
		logger.Error("Instrucción vacía")
		return
	}

	operacion := partes[0] // Tipo de operación (SET, READ_MEM, etc.)
	args := partes[1:]     // Argumentos de la operación

	// Llamar a Execute para ejecutar la instrucción decodificada
	Execute(operacion, args, logger)
}

type estructuraEmpty struct {
}
type EstructuraTid struct {
	TID uint32
}
type EstructuraTiempo struct {
	MS int
}
type EstructuraRecurso struct {
	Recurso string
}

// Función Execute para ejecutar la instrucción decodificada
func Execute(operacion string, args []string, logger *slog.Logger) {
	var proceso types.Proceso
	proceso.ContextoEjecucion = *client.ReceivedContextoEjecucion
	proceso.Pid = GlobalPIDTID.PID
	proceso.Tid = GlobalPIDTID.TID

	switch operacion {
	case "SET":
		if len(args) != 2 {
			logger.Error("Error en argumentos de SET: se esperaban 2 argumentos")
			return
		}
		registro := args[0]
		valor, err := strconv.ParseUint(args[1], 10, 32)
		if err != nil {
			logger.Error("Error al convertir el valor para SET")
			return
		}
		// Asignar el valor al registro
		cpuInstruction.AsignarValorRegistro(registro, uint32(valor), GlobalPIDTID.TID, logger)

	case "READ_MEM":
		if len(args) != 2 {
			logger.Error("Error en argumentos de READ_MEM: se esperaban 2 argumentos")
			return
		}
		registroDatos := args[0]
		registroDireccion := args[1]
		cpuInstruction.LeerMemoria(registroDatos, registroDireccion, GlobalPIDTID, logger)

	case "WRITE_MEM":
		if len(args) != 2 {
			logger.Error("Error en argumentos de WRITE_MEM: se esperaban 2 argumentos")
			return
		}
		registroDireccion := args[0]
		registroDatos := args[1]
		cpuInstruction.EscribirMemoria(registroDireccion, registroDatos, GlobalPIDTID, logger)

	case "SUM":
		if len(args) != 2 {
			logger.Error("Error en argumentos de SUM: se esperaban 2 argumentos")
			return
		}
		registroDestino := args[0]
		registroOrigen := args[1]
		cpuInstruction.SumarRegistros(registroDestino, registroOrigen, GlobalPIDTID.TID, logger)

	case "SUB":
		if len(args) != 2 {
			logger.Error("Error en argumentos de SUB: se esperaban 2 argumentos")
			return
		}
		registroDestino := args[0]
		registroOrigen := args[1]
		cpuInstruction.RestarRegistros(registroDestino, registroOrigen, GlobalPIDTID.TID, logger)

	case "JNZ":
		if len(args) != 2 {
			logger.Error("Error en argumentos de JNZ: se esperaban 2 argumentos")
			return
		}
		registro := args[0]
		instruccion := args[1]
		cpuInstruction.SaltarSiNoCero(registro, instruccion, GlobalPIDTID.TID, logger)

	case "LOG":
		if len(args) != 1 {
			logger.Error("Error en argumentos de LOG: se esperaba 1 argumento")
			return
		}
		registro := args[0]
		cpuInstruction.LogRegistro(registro, GlobalPIDTID, logger)

	case "DUMP_MEMORY":

		//	Informar memoria
		dumpMemory := estructuraEmpty{}
		proceso.ContextoEjecucion.PC++
		client.EnviarContextoDeEjecucion(proceso, "actualizar_contexto", logger)
		logger.Info(fmt.Sprintf("## TID: %d - Actualizo Contexto Ejecución", GlobalPIDTID.TID))
		//AnteriorPIDTID = GlobalPIDTID

		utils.Control = false //! OJO
		client.CederControlAKernell(dumpMemory, "DUMP_MEMORY", logger)

	case "IO":

		// Parseo los MS
		ms := parcearArgs(args[0], logger)

		//	Informar memoria
		io := EstructuraTiempo{
			MS: ms,
		}
		proceso.ContextoEjecucion.PC++

		utils.Control = false //! OJO (creo que va asi porque cuando manda a io no sigue ejecutando el io)
		client.EnviarContextoDeEjecucion(proceso, "actualizar_contexto", logger)
		logger.Info(fmt.Sprintf("## TID: %d - Actualizo Contexto Ejecución", GlobalPIDTID.TID))
		//AnteriorPIDTID = GlobalPIDTID
		CederControlAKernell2(io, "IO", logger)

	case "PROCESS_CREATE":

		// Parsear a entero
		arg1 := parcearArgs(args[1], logger)
		arg2 := parcearArgs(args[2], logger)

		//	Informar memoria
		processCreate := types.ProcessCreateParams{
			Path:      args[0],
			Tamanio:   arg1,
			Prioridad: arg2,
		}
		proceso.ContextoEjecucion.PC++
		client.EnviarContextoDeEjecucion(proceso, "actualizar_contexto", logger)
		logger.Info(fmt.Sprintf("## TID: %d - Actualizo Contexto Ejecución", GlobalPIDTID.TID))
		//AnteriorPIDTID = GlobalPIDTID
		client.CederControlAKernell(processCreate, "PROCESS_CREATE", logger)

	case "THREAD_CREATE":
		// Parsear la prioridad a entero
		prio := parcearArgs(args[1], logger)

		//	Informar memoria
		threadCreate := types.ThreadCreateParams{
			Path:      args[0],
			Prioridad: prio,
		}
		proceso.ContextoEjecucion.PC++
		client.EnviarContextoDeEjecucion(proceso, "actualizar_contexto", logger)
		logger.Info(fmt.Sprintf("## TID: %d - Actualizo Contexto Ejecución", GlobalPIDTID.TID))
		//AnteriorPIDTID = GlobalPIDTID
		client.CederControlAKernell(threadCreate, "THREAD_CREATE", logger)

	case "THREAD_JOIN":

		//Parseo el TID
		tid := parcearArgs(args[0], logger)

		threadJoin := EstructuraTid{
			TID: uint32(tid),
		}

		//	Informar memoria
		// utils.Control = false
		proceso.ContextoEjecucion.PC++
		client.EnviarContextoDeEjecucion(proceso, "actualizar_contexto", logger)
		logger.Info(fmt.Sprintf("## TID: %d - Actualizo Contexto Ejecución", GlobalPIDTID.TID))
		//AnteriorPIDTID = GlobalPIDTID
		CederControlAKernell2(threadJoin, "THREAD_JOIN", logger)

	case "THREAD_CANCEL":

		// Parseo el TID
		tid := parcearArgs(args[0], logger)

		//	Informar memoria
		threadCancel := EstructuraTid{
			TID: uint32(tid),
		}
		proceso.ContextoEjecucion.PC++
		client.EnviarContextoDeEjecucion(proceso, "actualizar_contexto", logger)
		logger.Info(fmt.Sprintf("## TID: %d - Actualizo Contexto Ejecución", GlobalPIDTID.TID))
		//AnteriorPIDTID = GlobalPIDTID
		client.CederControlAKernell(threadCancel, "THREAD_CANCEL", logger)

	case "MUTEX_CREATE":
		//	Informar memoria
		mutexCreate := EstructuraRecurso{
			Recurso: args[0],
		}
		proceso.ContextoEjecucion.PC++
		client.EnviarContextoDeEjecucion(proceso, "actualizar_contexto", logger)
		logger.Info(fmt.Sprintf("## TID: %d - Actualizo Contexto Ejecución", GlobalPIDTID.TID))
		//AnteriorPIDTID = GlobalPIDTID
		client.CederControlAKernell(mutexCreate, "MUTEX_CREATE", logger)

	case "MUTEX_LOCK":
		//	Informar memoria
		mutexLock := EstructuraRecurso{
			Recurso: args[0],
		}
		proceso.ContextoEjecucion.PC++
		client.EnviarContextoDeEjecucion(proceso, "actualizar_contexto", logger)
		logger.Info(fmt.Sprintf("## TID: %d - Actualizo Contexto Ejecución", GlobalPIDTID.TID))
		//AnteriorPIDTID = GlobalPIDTID
		CederControlAKernell2(mutexLock, "MUTEX_LOCK", logger)

	case "MUTEX_UNLOCK":

		//	Informar memoria
		mutexUnlock := EstructuraRecurso{
			Recurso: args[0],
		}

		proceso.ContextoEjecucion.PC++
		client.EnviarContextoDeEjecucion(proceso, "actualizar_contexto", logger)
		logger.Info(fmt.Sprintf("## TID: %d - Actualizo Contexto Ejecución", GlobalPIDTID.TID))
		//AnteriorPIDTID = GlobalPIDTID
		CederControlAKernell2(mutexUnlock, "MUTEX_UNLOCK", logger)

	case "THREAD_EXIT":
		//	Informar memoria
		threadExit := estructuraEmpty{}
		proceso.ContextoEjecucion.PC++
		client.EnviarContextoDeEjecucion(proceso, "actualizar_contexto", logger)
		logger.Info(fmt.Sprintf("## TID: %d - Actualizo Contexto Ejecución", GlobalPIDTID.TID))
		//AnteriorPIDTID = GlobalPIDTID
		CederControlAKernell2(threadExit, "THREAD_EXIT", logger)

	case "PROCESS_EXIT":
		//	Informar memoria
		processExit := estructuraEmpty{}
		proceso.ContextoEjecucion.PC++
		client.EnviarContextoDeEjecucion(proceso, "actualizar_contexto", logger)
		logger.Info(fmt.Sprintf("## TID: %d - Actualizo Contexto Ejecución", GlobalPIDTID.TID))
		//AnteriorPIDTID = GlobalPIDTID

		// ROMPO EL CICLO YA QUE SIEMPRE VA A FINALIZAR EL PROCESO
		utils.Control = false
		client.CederControlAKernell(processExit, "PROCESS_EXIT", logger)

	default:
		logger.Error(fmt.Sprintf("Operación desconocida: %s", operacion))

	}
}

func CheckInterrupt(tidActual uint32, pidActual uint32, logger *slog.Logger) {

	var proceso types.Proceso
	proceso.ContextoEjecucion = *client.ReceivedContextoEjecucion
	proceso.Pid = GlobalPIDTID.PID
	proceso.Tid = GlobalPIDTID.TID

	// Verificar si hay una interrupción pendiente
	if InterrupcionRecibida != nil {
		if InterrupcionRecibida.TID == tidActual && InterrupcionRecibida.PID == pidActual {
			// Log de la interrupción recibida
			logger.Info(fmt.Sprintf("Atendiendo Interrupcion: %s ", InterrupcionRecibida.NombreInterrupcion))
			// proceso.ContextoEjecucion.PC++ //! ACA ESTA EL ERROR (SI FUNCIONA BORRAR TODA LA LINEA)

			if client.ReceivedContextoEjecucion.PC == PCpaqueande {
				proceso.ContextoEjecucion.PC++
			}

			client.EnviarContextoDeEjecucion(proceso, "actualizar_contexto", logger)
			logger.Info(fmt.Sprintf("## TID: %d - Actualizo Contexto Ejecución", GlobalPIDTID.TID))
			client.EnviarDesalojo(proceso.Pid, proceso.Tid, InterrupcionRecibida.NombreInterrupcion, logger)

			// Eliminar la interrupción después de procesarla
			InterrupcionRecibida = nil
		} else {
			// Si el TID no coincide, descartar la interrupción
			logger.Info("Interrupción descartada debido a TID no coincidente \n" +
				fmt.Sprintf("Interrupción PID: %d \n", InterrupcionRecibida.PID) +
				fmt.Sprintf("Interrupción TID: %d \n", InterrupcionRecibida.TID) +
				fmt.Sprintf("Actual PID: %d \n", pidActual) +
				fmt.Sprintf("Actual TID: %d", tidActual))
			// Descartar la interrupción al no coincidir el TID
			InterrupcionRecibida = nil
		}
	}
}

func parcearArgs(arg string, logger *slog.Logger) int {
	argParseado, err := strconv.Atoi(arg)
	if err != nil {
		logger.Error("Error al convertir la prioridad para THREAD_CREATE")
		return -1 // Return a default value or handle the error appropriately
	}
	return argParseado
}

// PONGO ACA POR UN TEMA DE INCLUCIONES CIRCULARES

func CederControlAKernell2[T any](dato T, endpoint string, logger *slog.Logger) {

	body, err := json.Marshal(dato)
	if err != nil {
		logger.Error("Se produjo un error codificando el mensaje")
		return
	}

	url := fmt.Sprintf("http://%s:%d/%s", utils.Configs.IpKernel, utils.Configs.PortKernel, endpoint)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		logger.Error(fmt.Sprintf("Se produjo un error enviando mensaje a ip:%s puerto:%d", utils.Configs.IpKernel, utils.Configs.PortKernel))
		return
	}
	// Aseguramos que el body sea cerrado
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted { //! USO ESTE CUANDO NO NECESITO QUE ROMPA EL BUCLE
		return
	}
	if resp.StatusCode == http.StatusOK { //! USO ESTE CUANDO NECESITO QUE ROMPA EL BUCLE
		utils.Control = false
		GlobalPIDTID = types.PIDTID{TID: 10000, PID: 0}
		return
	}
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		logger.Error("La respuesta del servidor no fue OK")
		return // Indica que la respuesta no fue exitosa
	}
}
