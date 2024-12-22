package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/sisoputnfrba/tp-golang/cpu/cicloDeInstruccion"
	"github.com/sisoputnfrba/tp-golang/kernel/client"
	"github.com/sisoputnfrba/tp-golang/kernel/planificador"
	"github.com/sisoputnfrba/tp-golang/kernel/utils"
	"github.com/sisoputnfrba/tp-golang/utils/conexiones"
	"github.com/sisoputnfrba/tp-golang/utils/types"
)

func Iniciar_kernel(logger *slog.Logger) {
	mux := http.NewServeMux()

	// Endpoints
	mux.HandleFunc("POST /PROCESS_CREATE", PROCESS_CREATE(logger))
	mux.HandleFunc("POST /PROCESS_EXIT", PROCESS_EXIT(logger))
	mux.HandleFunc("POST /THREAD_CREATE", THREAD_CREATE(logger))
	mux.HandleFunc("POST /THREAD_JOIN", THREAD_JOIN(logger))
	mux.HandleFunc("POST /THREAD_CANCEL", THREAD_CANCEL(logger))
	mux.HandleFunc("POST /THREAD_EXIT", THREAD_EXIT(logger))
	mux.HandleFunc("POST /DUMP_MEMORY", DUMP_MEMORY(logger))
	mux.HandleFunc("POST /dump_response", Respuesta_dump(logger))
	mux.HandleFunc("POST /MUTEX_CREATE", MUTEX_CREATE(logger))
	mux.HandleFunc("POST /MUTEX_LOCK", MUTEX_LOCK(logger))
	mux.HandleFunc("POST /MUTEX_UNLOCK", MUTEX_UNLOCK(logger))
	mux.HandleFunc("POST /IO", IO(logger))

	mux.HandleFunc("POST /recibir-desalojo", Recibir_desalojo(logger))

	conexiones.LevantarServidor(strconv.Itoa(utils.Configs.Port), mux, logger)

}

// Syscalls referidas a procesos

func PROCESS_CREATE(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Info(fmt.Sprintf("## (%d:%d) - Solicitó syscall: PROCESS_CREATE", utils.Execute.PID, utils.Execute.TID))
		decoder := json.NewDecoder(r.Body)
		var magic types.ProcessCreateParams
		err := decoder.Decode(&magic)
		if err != nil {
			logger.Error(fmt.Sprintf("Error al decodificar mensaje: %s", err.Error()))
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Error al decodificar mensaje"))
			return
		}
		planificador.Crear_proceso(magic.Path, magic.Tamanio, magic.Prioridad, logger)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

func Colas_vacias(colas map[int][]types.TCB) bool {
	for _, cola := range colas {
		if len(cola) != 0 {
			return false
		}
	}
	return true
}

func PROCESS_EXIT(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Info(fmt.Sprintf("## (%d:%d) - Solicitó syscall: PROCESS_EXIT", utils.Execute.PID, utils.Execute.TID))
		finaliza := utils.Execute.PID

		utils.Execute = nil
		planificador.Finalizar_proceso(finaliza, logger)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))

		if !Colas_vacias(planificador.ColaReady) {
			planificador.SignalEnviado = true
			planificador.Semaforo.Signal()
		}
	}
}

func DUMP_MEMORY(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Info(fmt.Sprintf("## (%d:%d) - Solicitó syscall: DUMP_MEMORY", utils.Execute.PID, utils.Execute.TID))
		parametros := types.PIDTID{TID: utils.Execute.TID, PID: utils.Execute.PID} // Saco el pid y el tid del hilo que esta ejecutando

		bloqueado := utils.Bloqueado{PID: parametros.PID, TID: parametros.TID, Motivo: utils.DUMP}
		logger.Info(fmt.Sprintf("## (%d:%d) - Bloqueado por: DUMP MEMORY", utils.Execute.PID, utils.Execute.TID))
		utils.Execute = nil
		utils.Encolar(&planificador.ColaBlocked, bloqueado)

		planificador.SignalEnviado = true
		planificador.Semaforo.Signal()

		client.Enviar_Body(parametros, utils.Configs.IpMemory, utils.Configs.PortMemory, "MEMORY-DUMP", logger)

		w.WriteHeader(http.StatusOK)
	}
}

func Respuesta_dump(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		respuestaDelDump := types.RespuestaDump{}
		err := json.NewDecoder(r.Body).Decode(&respuestaDelDump)
		if err != nil {
			logger.Error(fmt.Sprintf("Error al decodificar mensaje: %s", err.Error()))
			w.WriteHeader(http.StatusBadRequest)
		}

		if respuestaDelDump.Respuesta == "OK" {
			desbloqueado := utils.Desencolar_Por_Motivo(&planificador.ColaBlocked, utils.DUMP)
			pcb := utils.Obtener_PCB_por_PID(desbloqueado.PID)
			tcb := pcb.TCBs[desbloqueado.TID]
			utils.Encolar_ColaReady(planificador.ColaReady, tcb)

			planificador.SignalEnviado = true
			planificador.Semaforo.Signal()

		} else {
			desbloqueado := utils.Desencolar_Por_Motivo(&planificador.ColaBlocked, utils.DUMP)
			pcb := utils.Obtener_PCB_por_PID(desbloqueado.PID)
			tcb := pcb.TCBs[desbloqueado.TID]
			planificador.Finalizar_proceso(tcb.PID, logger)
		}
	}
}

// Syscalls referidas a hilos

// BODY - VERBO POST
func THREAD_CREATE(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Info(fmt.Sprintf("## (%d:%d) - Solicitó syscall: THREAD_CREATE", utils.Execute.PID, utils.Execute.TID))

		// Agarramos los parametros del body
		var params types.ThreadCreateParams
		err := json.NewDecoder(r.Body).Decode(&params)
		if err != nil {
			logger.Error(fmt.Sprintf("Error al decodificar mensaje: %s\n", err.Error()))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Creamos el hilo
		planificador.Crear_hilo(params.Path, params.Prioridad, logger)

		// Respondemos con un OK
		respuesta, err := json.Marshal("OK")
		if err != nil {
			http.Error(w, "Error al codificar mensaje como JSON", http.StatusInternalServerError)
		}

		if utils.Configs.SchedulerAlgorithm != "FIFO" {
			planificador.SignalEnviado = true
			planificador.Semaforo.Signal()
		}

		w.WriteHeader(http.StatusOK)
		w.Write(respuesta)
	}
}

func THREAD_EXIT(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logger.Info(fmt.Sprintf("## (%d:%d) - Solicitó syscall: THREAD_EXIT", utils.Execute.PID, utils.Execute.TID))

		// Finalizamos el hilo
		planificador.Finalizar_hilo(utils.Execute.TID, utils.Execute.PID, logger)

		// Respondemos con un OK
		respuesta, err := json.Marshal("OK")
		if err != nil {
			http.Error(w, "Error al codificar mensaje como JSON", http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusOK)
		w.Write(respuesta)

		utils.Execute = nil
		planificador.SignalEnviado = true
		planificador.Semaforo.Signal()
	}
}

func THREAD_CANCEL(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logger.Info(fmt.Sprintf("## (%d:%d) - Solicitó syscall: THREAD_CANCEL", utils.Execute.PID, utils.Execute.TID))

		// Totamos el valor del body
		var tid cicloDeInstruccion.EstructuraTid
		err := json.NewDecoder(r.Body).Decode(&tid)
		if err != nil {
			logger.Error(fmt.Sprintf("Error al decodificar mensaje: %s\n", err.Error()))
		}

		// Finalizamos el hilo
		_, existe := utils.MapaPCB[utils.Execute.PID].TCBs[uint32(tid.TID)]
		if existe {
			planificador.Finalizar_hilo(uint32(tid.TID), utils.Execute.PID, logger)
		}

		// Respondemos con un OK
		respuesta, err := json.Marshal("OK")
		if err != nil {
			http.Error(w, "Error al codificar mensaje como JSON", http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
		w.Write(respuesta)
		client.Enviar_Body(types.PIDTID{TID: utils.Execute.TID, PID: utils.Execute.PID}, utils.Configs.IpCPU, utils.Configs.PortCPU, "EJECUTAR_KERNEL", logger)
	}
}

func THREAD_JOIN(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Info(fmt.Sprintf("## (%d:%d) - Solicitó syscall: THREAD_JOIN", utils.Execute.PID, utils.Execute.TID))

		// Tomamos el valor del body
		var tid cicloDeInstruccion.EstructuraTid
		err := json.NewDecoder(r.Body).Decode(&tid)
		if err != nil {
			logger.Error(fmt.Sprintf("Error al decodificar mensaje: %s\n", err.Error()))
		}

		_, existe := utils.MapaPCB[utils.Execute.PID].TCBs[uint32(tid.TID)]

		if !existe {
			respuesta, err := json.Marshal("CONTINUAR_EJECUCION")
			if err != nil {
				http.Error(w, "Error al codificar mensaje como JSON", http.StatusInternalServerError)
			}
			w.WriteHeader(http.StatusAccepted)
			w.Write(respuesta)
			return
		}

		// Mandamos el hilo a block
		bloqueado := utils.Bloqueado{PID: utils.Execute.PID, TID: utils.Execute.TID, Motivo: utils.THREAD_JOIN, QuienFue: strconv.Itoa(int(tid.TID))}

		utils.Encolar(&planificador.ColaBlocked, bloqueado)

		if utils.Configs.SchedulerAlgorithm == "FIFO" || utils.Configs.SchedulerAlgorithm == "PRIORIDADES" {
		} else {

		}

		logger.Info(fmt.Sprintf("## (%d:%d) - Bloqueado por: THREAD_JOIN", utils.Execute.PID, utils.Execute.TID))
		utils.Execute = nil

		respuesta, err := json.Marshal("OK")
		if err != nil {
			http.Error(w, "Error al codificar mensaje como JSON", http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
		w.Write(respuesta)

		planificador.SignalEnviado = true
		planificador.Semaforo.Signal()

	}
}

func MUTEX_CREATE(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logger.Info(fmt.Sprintf("## (%d:%d) - Solicitó syscall: MUTEX_CREATE", utils.Execute.PID, utils.Execute.TID))

		// Tomamos el valor del tid de la variable del body
		var mutexName cicloDeInstruccion.EstructuraRecurso
		err := json.NewDecoder(r.Body).Decode(&mutexName)
		if err != nil {
			logger.Error(fmt.Sprintf("Error al decodificar mensaje: %s\n", err.Error()))
		}

		// Creamos el mutex
		_, existe := utils.MapaPCB[utils.Execute.PID].Mutexs[mutexName.Recurso]
		if existe {
			respuesta, err := json.Marshal("MUTEX_YA_EXISTE")
			if err != nil {
				http.Error(w, "Error al codificar mensaje como JSON", http.StatusInternalServerError)
			}
			w.WriteHeader(http.StatusOK)
			w.Write(respuesta)
		}

		// Creamos el mutex y lo agregamos al mapa de mutexs del PCB
		utils.MapaPCB[utils.Execute.PID].Mutexs[mutexName.Recurso] = "LIBRE"
		respuesta, err := json.Marshal("OK")
		if err != nil {
			http.Error(w, "Error al codificar mensaje como JSON", http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
		w.Write(respuesta)
	}
}

// 3 CASOS:
// 1. Si el mutex no existe, finaliza el hilo y responde con "HILO_FINALIZADO"
// 2. Si el mutex esta libre, lo toma y responde con "MUTEX_TOMADO"
// 3. Si el mutex esta ocupado, bloquea el hilo y responde con "HILO_BLOQUEADO"
func MUTEX_LOCK(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logger.Info(fmt.Sprintf("## (%d:%d) - Solicitó syscall: MUTEX_LOCK", utils.Execute.PID, utils.Execute.TID))

		var mutexName cicloDeInstruccion.EstructuraRecurso
		err := json.NewDecoder(r.Body).Decode(&mutexName)
		if err != nil {
			logger.Error(fmt.Sprintf("Error al decodificar mensaje: %s\n", err.Error()))
		}

		// Verificamos que el mutex exista - si NO existe mandamos el hilo a Exit
		_, existe := utils.MapaPCB[utils.Execute.PID].Mutexs[mutexName.Recurso]
		if !existe {
			planificador.Finalizar_hilo(utils.Execute.TID, utils.Execute.PID, logger)
			respuesta, err := json.Marshal("HILO_FINALIZADO")
			if err != nil {
				http.Error(w, "Error al codificar mensaje como JSON", http.StatusInternalServerError)
			}
			w.WriteHeader(http.StatusOK)
			w.Write(respuesta)

			utils.Execute = nil
			planificador.SignalEnviado = true
			planificador.Semaforo.Signal()
			return
		}

		// Tomamos el mutex si esta libre
		if utils.MapaPCB[utils.Execute.PID].Mutexs[mutexName.Recurso] == "LIBRE" {
			utils.MapaPCB[utils.Execute.PID].Mutexs[mutexName.Recurso] = strconv.Itoa(int(utils.Execute.TID))
			respuesta, err := json.Marshal("MUTEX_TOMADO")
			if err != nil {
				w.Write([]byte("Error al codificar mensaje como JSON"))
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			w.WriteHeader(http.StatusAccepted)
			w.Write(respuesta)

			return
		}

		// Si no esta libre, bloqueamos el hilo
		if utils.MapaPCB[utils.Execute.PID].Mutexs[mutexName.Recurso] != "LIBRE" {
			bloqueado := utils.Bloqueado{PID: utils.Execute.PID, TID: utils.Execute.TID, Motivo: utils.Mutex, QuienFue: mutexName.Recurso}

			// Encolamos en ColaBlock y desencolamos de ColaReady
			utils.Encolar(&planificador.ColaBlocked, bloqueado)

			logger.Info(fmt.Sprintf("## (%d:%d) - Bloqueado por: MUTEX", utils.Execute.PID, utils.Execute.TID))

			// Respondemos con un HILO_BLOQUEADO
			respuesta, err := json.Marshal("HILO_BLOQUEADO")
			if err != nil {
				http.Error(w, "Error al codificar mensaje como JSON", http.StatusInternalServerError)
			}
			w.WriteHeader(http.StatusOK)
			w.Write(respuesta)

			utils.Execute = nil
			planificador.SignalEnviado = true
			planificador.Semaforo.Signal()
			return
		}

	}
}

// Si el mutex no existe responde con "HILO_FINALIZADO" y finaliza el hilo
// Si el mutex se le asigna a un hilo responde "MUTEX_ASIGNADO"
// Si el mutex queda libre responde "MUTEX_LIBRE"
// Si el hilo no posee el mutex responde "HILO_NO_POSEE_MUTEX"
func MUTEX_UNLOCK(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logger.Info(fmt.Sprintf("## (%d:%d) - Solicitó syscall: MUTEX_UNLOCK", utils.Execute.PID, utils.Execute.TID))

		// Tomamos el valor del tid de la variable del body
		var mutexName cicloDeInstruccion.EstructuraRecurso
		err := json.NewDecoder(r.Body).Decode(&mutexName)
		if err != nil {
			logger.Error(fmt.Sprintf("Error al decodificar mensaje: %s\n", err.Error()))
		}

		// Verificamos que el mutex exista caso contrario mandamos el hilo a exit
		_, existe := utils.MapaPCB[utils.Execute.PID].Mutexs[mutexName.Recurso]
		if !existe {
			planificador.Finalizar_hilo(utils.Execute.TID, utils.Execute.PID, logger)
			respuesta, err := json.Marshal("HILO_FINALIZADO")
			if err != nil {
				http.Error(w, "Error al codificar mensaje como JSON", http.StatusInternalServerError)
			}
			w.WriteHeader(http.StatusOK)
			w.Write(respuesta)

			utils.Execute = nil
			planificador.SignalEnviado = true
			planificador.Semaforo.Signal()
			return
		}

		// Si el mutex existe, lo asignamos o liberamos segun corresponda
		if utils.MapaPCB[utils.Execute.PID].Mutexs[mutexName.Recurso] == strconv.Itoa(int(utils.Execute.TID)) {
			planificador.Mu.Lock()
			defer planificador.Mu.Unlock()

			nadieNecesitaMutex := true

			for _, bloqueado := range planificador.ColaBlocked {
				// Si alguien quiere el mutex
				if bloqueado.PID == utils.Execute.PID && bloqueado.Motivo == utils.Mutex && bloqueado.QuienFue == mutexName.Recurso {

					nadieNecesitaMutex = false
					utils.MapaPCB[utils.Execute.PID].Mutexs[mutexName.Recurso] = strconv.Itoa(int(bloqueado.TID))

					// Desencolamos de la cola de bloqueados y encolamos en la cola de ready

					utils.Desencolar_cola_block(bloqueado, &planificador.ColaBlocked)
					utils.Encolar_ColaReady(planificador.ColaReady, utils.MapaPCB[bloqueado.PID].TCBs[bloqueado.TID])

					logger.Info(fmt.Sprintf("## (%d:%d) - Desbloqueado por: MUTEX y asignado a el", bloqueado.PID, bloqueado.TID))

					respuesta, err := json.Marshal("MUTEX_ASIGNADO")
					if err != nil {
						http.Error(w, "Error al codificar mensaje como JSON", http.StatusInternalServerError)
					}
					w.WriteHeader(http.StatusAccepted)
					w.Write(respuesta)

					return
				}
				// Si el mutex no lo necesita nadie
			}

			if nadieNecesitaMutex {
				utils.MapaPCB[utils.Execute.PID].Mutexs[mutexName.Recurso] = "LIBRE"
				respuesta, err := json.Marshal("MUTEX_LIBRE")
				if err != nil {
					http.Error(w, "Error al codificar mensaje como JSON", http.StatusInternalServerError)
				}

				logger.Info(fmt.Sprintf("## %s quedo LIBRE", mutexName.Recurso))

				w.WriteHeader(http.StatusAccepted)
				w.Write(respuesta)
				return
			}
		}
		// Si el mutex existe y no esta tomado por el hilo q invoca la syscall
		respuesta, err := json.Marshal("HILO_NO_POSEE_MUTEX")
		if err != nil {
			http.Error(w, "Error al codificar mensaje como JSON", http.StatusInternalServerError)
		}

		logger.Info("EL hilo no posee el mutex")

		w.WriteHeader(http.StatusAccepted)
		w.Write(respuesta)
	}
}

func IO(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logger.Info(fmt.Sprintf("## (%d:%d) - Solicitó syscall: IO", utils.Execute.PID, utils.Execute.TID))

		var ms cicloDeInstruccion.EstructuraTiempo
		err := json.NewDecoder(r.Body).Decode(&ms)
		if err != nil {
			logger.Error(fmt.Sprintf("Error al decodificar mensaje: %s\n", err.Error()))
		}

		solicitud := utils.SolicitudIO{
			PID:       utils.Execute.PID,
			TID:       utils.Execute.TID,
			Duracion:  ms.MS,
			Timestamp: time.Now(),
		}
		utils.Encolar(&planificador.ColaBlocked, utils.Bloqueado{PID: utils.Execute.PID, TID: utils.Execute.TID, Motivo: utils.IO})
		utils.Encolar(&planificador.ColaIO, solicitud)
		logger.Info(fmt.Sprintf("## (%d:%d) - Bloqueado por: IO", utils.Execute.PID, utils.Execute.TID))

		utils.Execute = nil
		planificador.SignalEnviado = true
		planificador.Semaforo.Signal()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

func Recibir_desalojo(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var magic types.HiloDesalojado
		err := decoder.Decode(&magic)
		if err != nil {
			logger.Error(fmt.Sprintf("Error al decodificar hilo desalojado: %s", err.Error()))
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Error al decodificar mensaje"))
			return
		}

		switch magic.Motivo {
		case "FIN_QUANTUM":

			planificador.Mu.Lock()

			if utils.Execute == nil {
				break
			}

			tcb, existe := utils.MapaPCB[magic.PID].TCBs[magic.TID]
			if existe {
				if utils.Execute.PID == magic.PID && utils.Execute.TID == magic.TID {
					utils.Encolar_ColaReady(planificador.ColaReady, tcb)
					logger.Info(fmt.Sprintf("## (%d:%d) - Desalojado por fin de Quantum", magic.PID, magic.TID))
				}
			}

			utils.Execute = nil

			planificador.Mu.Unlock()

			planificador.SignalEnviado = true
			planificador.Semaforo.Signal()

		case "SEGMENTATION_FAULT":
			planificador.Finalizar_proceso(magic.PID, logger)
			utils.Execute = nil
			planificador.SignalEnviado = true
			planificador.Semaforo.Signal()

		case "PRIORIDAD":
			utils.Execute = nil
			logger.Info(fmt.Sprintf("## (%d:%d) - Desalojado por PRIORIDAD", magic.PID, magic.TID))
			utils.Encolar_ColaReady(planificador.ColaReady, utils.Obtener_PCB_por_PID(magic.PID).TCBs[magic.TID])
			planificador.SignalEnviado = true
			planificador.Semaforo.Signal()
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}
