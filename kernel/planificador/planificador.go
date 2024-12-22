package planificador

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/sisoputnfrba/tp-golang/kernel/client"
	"github.com/sisoputnfrba/tp-golang/kernel/utils"
	"github.com/sisoputnfrba/tp-golang/utils/generadores"
	"github.com/sisoputnfrba/tp-golang/utils/types"
)

var ColaNew []types.ProcesoNew    //Cola de procesos nuevos (Manejada por FIFO)
var ColaReady map[int][]types.TCB // Aca tengo dudas de como es, no me queda claro si las colas son distintas para PCB y TCB
var ColaBlocked []utils.Bloqueado
var ColaExit []types.TCB //Cola de procesos finalizados
var ColaIO []utils.SolicitudIO
var MapColasMultinivel map[int][]types.TCB

var Semaforo *utils.Semaphore

// Variables para el tema de los quantums
var (
	Mu              sync.Mutex
	ExecuteContador int
)

var NecesitoCompactar bool

func Inicializar_colas() {
	ColaNew = []types.ProcesoNew{}
	ColaReady = make(map[int][]types.TCB)
	ColaBlocked = []utils.Bloqueado{}
	ColaExit = []types.TCB{}
	ColaIO = []utils.SolicitudIO{}
	MapColasMultinivel = make(map[int][]types.TCB)
	Semaforo = utils.NewSemaphore(1)
	utils.Execute = nil
}

// Se le pasa el archivo de pseudocódigo, el tamaño del proceso y la prioridad
func Crear_proceso(pseudo string, tamanio int, prioridad int, logger *slog.Logger) {
	pcb := generadores.Generar_PCB()
	utils.MapaPCB[pcb.PID] = pcb // Guardo el PCB en el mapa de PCBs
	logger.Info(fmt.Sprintf("## (%d:0) Se crea el proceso - Estado: NEW", pcb.PID))
	if len(ColaNew) == 0 {
		// Enviar a memoria el archivo de pseudocódigo y el tamaño del proceso
		success, alt := Inicializar_proceso(pcb, pseudo, tamanio, prioridad, logger)
		if !success {
			// Si no se pudo incializar el proceso y necesita compactacion
			if alt == "COMPACTACION" {

				NecesitoCompactar = true
				for utils.Execute != nil {
					time.Sleep(1000 * time.Millisecond) //no me parece la mejor implementacion a nivel recursos pero no se me ocurre otra sin modificar mucho la estructura actual
				}
				if client.Enviar_QueryPath(0, utils.Configs.IpMemory, utils.Configs.PortMemory, "compactar", "PATCH", logger) {
					logger.Info("Compactacion de Memoria exitosa, reintentando inicializar proceso")
					Inicializar_proceso(pcb, pseudo, tamanio, prioridad, logger)

					NecesitoCompactar = false
					SignalEnviado = true
					Semaforo.Signal()
				}

			}
			// Si no se pudo inicializar el proceso, se encola en ColaNew
			new := types.ProcesoNew{PCB: pcb, Pseudo: pseudo, Tamanio: tamanio, Prioridad: prioridad}
			utils.Encolar(&ColaNew, new)
		}
	} else {
		// Si ya hay otros procesos esperando, simplemente encolar
		new := types.ProcesoNew{PCB: pcb, Pseudo: pseudo, Tamanio: tamanio, Prioridad: prioridad}
		utils.Encolar(&ColaNew, new)
	}
}

// Devuelve un booleano y un string, este indica en caso de que no se pueda inicializar el proceso, si necesita compactacion
func Inicializar_proceso(pcb types.PCB, pseudo string, tamanio int, prioridad int, logger *slog.Logger) (bool, string) {
	// Enviar a memoria el archivo de pseudocódigo y el tamaño del proceso
	parametros := types.PathTamanio{Path: pseudo, Tamanio: tamanio, PID: pcb.PID} //añadi el pid para crear proceso en memoria
	success, alt := client.Enviar_Proceso(parametros, utils.Configs.IpMemory, utils.Configs.PortMemory, "CREAR-PROCESO", logger)

	if success {
		// Si se asigna espacio, se crea el TCB 0 y se pasa a READY
		tcb := generadores.Generar_TCB(&pcb, prioridad)
		utils.MapaPCB[pcb.PID] = pcb // Actualizo el PCB en el mapa de PCBs (nose si está bien asi o abria que agregar unicamente el tcb y no sobreescribir)
		utils.Encolar_ColaReady(ColaReady, tcb)
		logger.Info(fmt.Sprintf("## (%d:%d) Se crea el Hilo - Estado: READY", pcb.PID, tcb.TID))

		// Desbloquear el planificador para procesar el hilo en READY
		SignalEnviado = true
		Semaforo.Signal()
		return true, ""
	}
	if alt == "COMPACTACION" {
		logger.Error("compactar 2")
		return false, "COMPACTACION"
	}
	if alt == "NO HAY MEMORIA" {
		logger.Error("NO HAY MEMORIA SUFICIENTE")
		return false, ""
	}
	// Si no hay espacio en memoria, devolver false
	logger.Error("No se pudo asignar espacio en memoria para el proceso")
	return false, ""
}

func Reintentar_procesos(logger *slog.Logger) {
	if len(ColaNew) > 0 {
		// Intentar inicializar el primer proceso en ColaNew
		new := ColaNew[0]
		success, alt := Inicializar_proceso(new.PCB, new.Pseudo, new.Tamanio, new.Prioridad, logger)
		if success {
			// Si se inicializa correctamente, quitarlo de ColaNew
			utils.Desencolar(&ColaNew)
		}
		if alt == "COMPACTACION" {

			NecesitoCompactar = true
			for utils.Execute != nil {
				time.Sleep(1000 * time.Millisecond)
			}
			if client.Enviar_Body(types.EstructuraEmpty{}, utils.Configs.IpMemory, utils.Configs.PortMemory, "compactar", logger) {
				logger.Info("Compactacion de Memoria exitosa, reintentando inicializar proceso")

				Inicializar_proceso(new.PCB, new.Pseudo, new.Tamanio, new.Prioridad, logger)
				utils.Desencolar(&ColaNew)

				NecesitoCompactar = false
				SignalEnviado = true
				Semaforo.Signal()
			}
		}
	}
}

// Se le pasa el pid del proceso a finalizar
func Finalizar_proceso(pid uint32, logger *slog.Logger) {

	success := client.Enviar_QueryPath(pid, utils.Configs.IpMemory, utils.Configs.PortMemory, "FINALIZAR-PROCESO", "PATCH", logger)

	if success {
		OK := utils.Enviar_proceso_a_exit(pid, ColaReady, &ColaBlocked, &ColaExit, logger)
		if OK {
			logger.Info(fmt.Sprintf("## Finaliza el proceso %d", pid))
			Reintentar_procesos(logger) // Intentar inicializar procesos en ColaNew
		} else {
			logger.Error("Algo salió mal en Memoria al querer finalizar el proceso")
		}
	} else {
		logger.Error("Algo salió mal en Memoria al querer finalizar el proceso")
	}
}

// Recibo de la cpu el archivo de instrucciones y la prioridad
func Crear_hilo(path string, prioridad int, logger *slog.Logger) {

	// Crear TCB
	pcb := utils.Obtener_PCB_por_PID(utils.Execute.PID)
	if pcb == nil {
		logger.Error("No se encontro el PCB")
		return
	}
	tcb := generadores.Generar_TCB(pcb, prioridad)

	//	Informar memoria
	infoMemoria := types.EnviarHiloAMemoria{
		TID:  tcb.TID,
		PID:  pcb.PID,
		Path: path,
	}
	if !client.Enviar_Body(infoMemoria, utils.Configs.IpMemory, utils.Configs.PortMemory, "CREAR_HILO", logger) {
		logger.Error("Error al crear hilo")
	}

	// Ingresar a la cola de READY
	utils.Encolar_ColaReady(ColaReady, tcb)

	logger.Info(fmt.Sprintf("## (%d:%d) Se crea el Hilo - Estado: READY", pcb.PID, tcb.TID))
}

// Finalizar hilo
func Finalizar_hilo(TID uint32, PID uint32, logger *slog.Logger) {

	// Informar memoria
	infoMemoria := types.PIDTID{
		TID: TID,
		PID: PID,
	}
	if !client.Enviar_Body(infoMemoria, utils.Configs.IpMemory, utils.Configs.PortMemory, "FINALIZAR_HILO", logger) {
		logger.Error("Error al finalizar hilo") //! SACAR
	}
	logger.Info("Se comunico a memoria la finalizacion del hilo")

	logger.Info(fmt.Sprintf("## (%d:%d) Finaliza el hilo", PID, TID))

	// Mover al estado de ready lo que estaban bloqueados por ese TID (THREAD_JOIN y MUTEX)
	utils.Librerar_Bloqueados_De_Hilo(&ColaBlocked, ColaReady, utils.MapaPCB[PID].TCBs[TID], logger)

	// Mandar a la cola de exit
	utils.Encolar(&ColaExit, utils.MapaPCB[PID].TCBs[TID])

	// Quitar de la lista de los TCBs del PCB
	utils.Sacar_TCB_Del_Map(&utils.MapaPCB, PID, TID, logger)

	Reintentar_procesos(logger) // Intentar inicializar procesos en ColaNew
}

// Función que procesa las solicitudes de I/O de la cola
func Procesar_cola_IO(colaIO *[]utils.SolicitudIO, logger *slog.Logger) {
	for {
		solicitud, haySolicitudes := utils.Proxima_solicitud(colaIO)
		if haySolicitudes {
			// Simular la duración de la E/S
			logger.Info(fmt.Sprintf("Procesando E/S para TID %d durante %d ms", solicitud.TID, solicitud.Duracion))
			time.Sleep(time.Duration(solicitud.Duracion) * time.Millisecond)

			// Una vez terminada la E/S, desbloquear el hilo
			desbloqueado := utils.Desencolar(&ColaBlocked)
			pcb := utils.Obtener_PCB_por_PID(desbloqueado.PID)
			tcb := pcb.TCBs[desbloqueado.TID]
			logger.Info(fmt.Sprintf("## (%d:%d) finalizó IO y pasa a READY", solicitud.PID, solicitud.TID))
			utils.Encolar_ColaReady(ColaReady, tcb)

			SignalEnviado = true
			Semaforo.Signal()
		} else {
			// No hay solicitudes en la cola, esperar un tiempo antes de volver a chequear
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// -------------------------------------- PLANIFICADORES CORTO PLAZO --------------------------------------

func Iniciar_planificador(config utils.Config, logger *slog.Logger) {
	switch config.SchedulerAlgorithm {
	case "FIFO":
		logger.Info("Iniciando planificador FIFO")
		go FIFO(logger)
	case "PRIORIDADES":
		logger.Info("Iniciando planificador por Prioridades")
		go PRIORIDADES(logger)
	case "CMN":
		logger.Info("Iniciando planificador CMN")
		go COLAS_MULTINIVEL(logger)
	default:
		logger.Info("Tipo de planificador no reconocido. Usando FIFO por defecto.")
		go FIFO(logger) // Por defecto, usa FIFO si no se reconoce el tipo
	}
}

func FIFO(logger *slog.Logger) {
	for {
		Semaforo.Wait()

		if len(ColaReady[0]) == 0 {
			logger.Info("No hay procesos en la cola de Ready")
			time.Sleep(100 * time.Millisecond) // Espera antes de volver a intentar
			continue
		}
		proximo, _ := utils.Desencolar_TCB(ColaReady, 0)

		// Lo ponemos a "ejecutar"
		utils.Execute = &utils.ExecuteActual{
			PID: proximo.PID,
			TID: proximo.TID,
		}

		client.Enviar_Body(types.PIDTID{TID: utils.Execute.TID, PID: utils.Execute.PID}, utils.Configs.IpCPU, utils.Configs.PortCPU, "EJECUTAR_KERNEL", logger)
	}
}

func PRIORIDADES(logger *slog.Logger) {
	for {
		Semaforo.Wait()

		if len(ColaReady[0]) > 0 {
			siguienteHilo := ColaReady[0][0]
			// Vamos buscando el hilo de menor prioridad (esto a su vez cumple que si hay otro de igual prioridad, desempata por el primero que llegó)
			for _, tcb := range ColaReady[0] {
				if tcb.Prioridad < siguienteHilo.Prioridad {
					siguienteHilo = tcb
				}
			}
			// Vemos si no hay nadie ejecutando o si la prioridad del siguiente hilo es mayor
			if utils.Execute == nil || siguienteHilo.Prioridad < utils.MapaPCB[utils.Execute.PID].TCBs[utils.Execute.TID].Prioridad {

				if utils.Execute != nil {
					// Enviamos la interrupción de desalojo por Prioridades
					client.Enviar_Body(types.InterruptionInfo{NombreInterrupcion: "PRIORIDAD", TID: utils.Execute.TID, PID: utils.Execute.PID}, utils.Configs.IpCPU, utils.Configs.PortCPU, "PRIORIDAD", logger)
				} else {
					logger.Info(fmt.Sprintf("Ejecutando hilo %d (PID: %d) con prioridad %d", siguienteHilo.TID, siguienteHilo.PID, siguienteHilo.Prioridad))
					utils.Execute = &utils.ExecuteActual{
						PID: siguienteHilo.PID,
						TID: siguienteHilo.TID,
					}
					// Remueve el hilo seleccionado de la cola de READY
					for i, tcb := range ColaReady[0] {
						if tcb.TID == siguienteHilo.TID {
							ColaReady[0] = append(ColaReady[0][:i], ColaReady[0][i+1:]...)
							break
						}
					}
					client.Enviar_Body_Async(types.PIDTID{TID: utils.Execute.TID, PID: utils.Execute.PID}, utils.Configs.IpCPU, utils.Configs.PortCPU, "EJECUTAR_KERNEL", logger)
				}
			} else {
			}
		} else {
			time.Sleep(100 * time.Millisecond) // Espera antes de volver a intentar
		}
	}
}

var SignalEnviado = false

func COLAS_MULTINIVEL(logger *slog.Logger) {

	for {

		Semaforo.Wait()
		SignalEnviado = false
		if NecesitoCompactar {
			utils.Execute = nil
			continue
		}

		// Agarro el proximo y lo elimino de la cola de ready
		proximo, hayAlguien := seleccionarSiguienteHilo()

		// Si no hay nadie en la cola de ready
		if !hayAlguien {
			logger.Info("No hay procesos en la cola de Ready")
			time.Sleep(100 * time.Millisecond) // Espera antes de volver a intentar

			continue
		}

		// Si hay alguien en la cola de ready
		if utils.Execute == nil {

			Mu.Lock()

			execID := ExecuteContador + 1
			utils.Execute = &utils.ExecuteActual{
				PID:       proximo.PID,
				TID:       proximo.TID,
				IDexecute: execID,
			}

			ExecuteContador = execID

			exec := utils.Execute

			logger.Info(fmt.Sprintf("Ejecutando hilo %d (PID: %d) con prioridad %d", proximo.TID, proximo.PID, proximo.Prioridad))

			utils.Desencolar_TCB(ColaReady, proximo.Prioridad)
			client.Enviar_Body_Async(types.PIDTID{TID: utils.Execute.TID, PID: utils.Execute.PID}, utils.Configs.IpCPU, utils.Configs.PortCPU, "EJECUTAR_KERNEL", logger)
			go Quantum(exec, logger) // Comenzamos un hilo para que maneje el quantum

			Mu.Unlock()

		} else if proximo.Prioridad < utils.MapaPCB[utils.Execute.PID].TCBs[utils.Execute.TID].Prioridad {

			client.Enviar_Body(types.InterruptionInfo{NombreInterrupcion: "PRIORIDAD", TID: utils.Execute.TID, PID: utils.Execute.PID}, utils.Configs.IpCPU, utils.Configs.PortCPU, "INTERRUPCION_FIN_QUANTUM", logger)

		} else {
		}

	}

}

func Quantum(exec *utils.ExecuteActual, logger *slog.Logger) {
	quantum := time.Duration(utils.Configs.Quantum) * time.Millisecond
	timer := time.NewTimer(quantum)

	<-timer.C

	Mu.Lock()
	defer Mu.Unlock()

	if utils.Execute != nil && utils.Execute.IDexecute == exec.IDexecute {
		client.Enviar_Body(types.InterruptionInfo{NombreInterrupcion: "FIN_QUANTUM", TID: utils.Execute.TID, PID: utils.Execute.PID}, utils.Configs.IpCPU, utils.Configs.PortCPU, "INTERRUPCION_FIN_QUANTUM", logger)
	}
}

func seleccionarSiguienteHilo() (types.TCB, bool) {

	// Encontrar el índice máximo de la cola de ready
	maxIndex := -1
	for index := range ColaReady {
		if index > maxIndex {
			maxIndex = index
		}
	}

	// Recorremos las colas desde la de mayor prioridad hasta la menor
	for prioridad := 0; prioridad <= maxIndex; prioridad++ {
		if len(ColaReady[prioridad]) > 0 {

			// Tomar el primer hilo de la cola
			siguienteHilo := ColaReady[prioridad][0]
			return siguienteHilo, true
		}
	}
	return types.TCB{}, false // No hay hilos disponibles
}

// No le veo sentido a esta funcion ya que Encolar_ColaReady ya hace lo mismo
func Meter_A_Planificar_Colas_Multinivel(tcb types.TCB, logger *slog.Logger) {

	// Agrego el tcb a la cola correspondiente, si no existe la cola se crea automáticamente
	ColaReady[tcb.Prioridad] = append(ColaReady[tcb.Prioridad], tcb)
}

func InsertarEnPosicion(slice []types.TCB, elemento types.TCB, posicion int) []types.TCB {

	nuevoSlice := append(slice[:posicion], append([]types.TCB{elemento}, slice[posicion:]...)...)
	return nuevoSlice
}
