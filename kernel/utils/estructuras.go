package utils

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/sisoputnfrba/tp-golang/utils/types"
)

// Struct para manejar las peticiones de IO
type SolicitudIO struct {
	PID       uint32    `json:"pid"`       // ID del proceso que realizó la solicitud
	TID       uint32    `json:"tid"`       // ID del hilo que realizó la solicitud
	Duracion  int       `json:"duracion"`  // Duración de la solicitud (en milisegundos)
	Timestamp time.Time `json:"timestamp"` // Indica el momento en el que se realizó la solicitud
}

// Hilo ejecutando actualmente
type ExecuteActual struct {
	PID       uint32 `json:"pid"`
	TID       uint32 `json:"tid"`
	IDexecute int    `json:"idexecute"`
}

var Execute *ExecuteActual

// Mapa para almacenar los PCB con su PID como clave
var MapaPCB map[uint32]types.PCB

// Inicializa el mapa de PCBs
func InicializarPCBMapGlobal() {
	MapaPCB = make(map[uint32]types.PCB)
}

// Función para obtener el PCB a partir de un PID
func Obtener_PCB_por_PID(pid uint32) *types.PCB {
	pcb, existe := MapaPCB[pid]
	if !existe {
		return nil
	}
	return &pcb
}

// Funcion para modificar la variable Execute
func Modificar_Execute(pid uint32, tid uint32) {
	if Execute == nil {
		Execute = &ExecuteActual{}
		Execute.PID = pid
		Execute.TID = tid
	} else {
		Execute.PID = pid
		Execute.TID = tid
	}
}

// Elimina los TCBs del PCB de las multiples colas de Ready (no importa cual sea el algoritmo de planificación)
func Eliminar_TCBs_de_cola_Ready(pcb *types.PCB, colas map[int][]types.TCB, logger *slog.Logger) {
	// Itera sobre cada cola de prioridad
	for prioridad, cola := range colas {
		var nuevaCola []types.TCB
		// Itera la cola buscando los TCBs que pertenecen al PCB actual
		for _, tcb := range cola {
			if tcb.PID != pcb.PID {
				nuevaCola = append(nuevaCola, tcb) // Mantiene los TCBs que no pertenecen al PCB actual
			} else {
				logger.Info(fmt.Sprintf("TCB con TID %d y PID %d eliminado de la cola de prioridad %d", tcb.TID, tcb.PID, prioridad))
			}
		}
		// Actualiza la cola en el mapa de colas
		colas[prioridad] = nuevaCola
	}
}

// Elimina los TCBs del PCB de la cola de BLOCKED
func Eliminar_TCBs_de_cola_Block(pcb *types.PCB, cola *[]Bloqueado, logger *slog.Logger) {
	var nuevaCola []Bloqueado
	// Itera la cola buscando los TCBs que pertenecen al PCB actual
	for _, tcb := range *cola {
		if tcb.PID != pcb.PID {
			nuevaCola = append(nuevaCola, tcb) // Mantiene los TCBs que no pertenecen al PCB actual
		} else {
			logger.Info(fmt.Sprintf("TCB con TID %d y PID %d eliminado de la cola de BLOCK", tcb.TID, tcb.PID))
		}
	}
	*cola = nuevaCola
}

func Eliminar_TCBs_de_cola_Block_Finalizar_Hilo(tcb Bloqueado, cola *[]Bloqueado, logger *slog.Logger) {
	var nuevaCola []Bloqueado

	for _, tcbCola := range *cola {
		if tcb.PID != tcbCola.PID {
			nuevaCola = append(nuevaCola, tcbCola) // Mantiene los TCBs que no pertenecen al PCB actual
		} else if tcb.PID == tcbCola.PID && tcb.TID != tcbCola.TID {
			nuevaCola = append(nuevaCola, tcbCola)
		}
		logger.Info(fmt.Sprintf("TCB con TID %d y PID %d eliminado de la cola de BLOCK", tcb.TID, tcb.PID))
	}
	*cola = nuevaCola
}

// Busca los TCBs del PCB en las colas de Ready y Blocked y los mueve a la cola de Exit
func Enviar_proceso_a_exit(pid uint32, colaReady map[int][]types.TCB, colaBlocked *[]Bloqueado, colaExit *[]types.TCB, logger *slog.Logger) bool {

	pcb := Obtener_PCB_por_PID(pid)
	if pcb == nil {
		logger.Error(fmt.Sprintf("No existe el proceso con PID: %d", pid))
		return false
	}

	// Elimina TCBs de la cola de ready y blocked si es que hubiera
	Eliminar_TCBs_de_cola_Ready(pcb, colaReady, logger)
	Eliminar_TCBs_de_cola_Block(pcb, colaBlocked, logger)

	// Mueve todos los TCBs del PCB a la cola de exit
	for _, tcb := range pcb.TCBs {
		*colaExit = append(*colaExit, tcb)
		logger.Info(fmt.Sprintf("TCB con TID %d movido a la cola de Exit", tcb.TID))
	}

	// Limpiar los TCBs del PCB
	pcb.TCBs = nil
	delete(MapaPCB, pid)
	logger.Info(fmt.Sprintf("Todos los TCBs del PCB con PID %d han sido liberados", pcb.PID))
	return true
}

// ! Si anda mal probar ponerle los punteors a las colas y el map -- Revisar los punteros de las funciones -- Revisar la asignacion de valores
// Se lo saque porque en go los map, slices y punteros ya son referencias, por lo cual
// no es necesario pasarlos como punteros
func Librerar_Bloqueados_De_Hilo(colaBloqueados *[]Bloqueado, colaReady map[int][]types.TCB, tcb types.TCB, logger *slog.Logger) {

	for _, bloqueado := range *colaBloqueados {

		if bloqueado.PID == tcb.PID && bloqueado.Motivo == THREAD_JOIN {
			num, err := strconv.ParseUint(bloqueado.QuienFue, 10, 32)
			if err != nil {
				logger.Error("El error esta en el primer if de Librerar_Bloqueados_De_Hilo")
			}
			num32 := uint32(num)
			if num32 == tcb.TID {
				Eliminar_TCBs_de_cola_Block_Finalizar_Hilo(bloqueado, colaBloqueados, logger)
				Encolar_ColaReady(colaReady, MapaPCB[bloqueado.PID].TCBs[bloqueado.TID])
				logger.Info(fmt.Sprintf("TCB con TID %d y PID %d, Bloqueado por THREAD_JOIN movido a la cola de Ready", bloqueado.TID, bloqueado.PID))
			}
		} else if bloqueado.PID == tcb.PID && bloqueado.Motivo == Mutex {

			if MapaPCB[tcb.PID].Mutexs[bloqueado.QuienFue] == strconv.Itoa(int(tcb.TID)) {
				MapaPCB[bloqueado.PID].Mutexs[bloqueado.QuienFue] = strconv.Itoa(int(bloqueado.TID))
				Eliminar_TCBs_de_cola_Block_Finalizar_Hilo(bloqueado, colaBloqueados, logger)
				Encolar_ColaReady(colaReady, MapaPCB[bloqueado.PID].TCBs[bloqueado.TID])
				logger.Info(fmt.Sprintf("TCB con TID %d y PID %d, Bloqueado por Mutex movido a la cola de Ready", bloqueado.TID, bloqueado.PID))
			}
		}
	}

}

// Estructuras para manejar los bloqueados
type Motivo int

const ( // Esto funciona mas o menos como el enum de c
	THREAD_JOIN Motivo = iota // Vale 0
	Mutex                     // Vale 1
	IO                        // Vale 2
	DUMP                      // Vale 3
)

// Como no se puede hacer un slice con un struc generico, hago que el QuienFue sea un string
// Y cuando necesite que sea un uint32 lo parseo
// ACLARACIONES: EL QUIENFUE SE PASA SIEMPRE COMO STRING
type Bloqueado struct {
	PID      uint32 `json:"pid"`
	TID      uint32 `json:"tid"`
	Motivo   Motivo `json:"motivo"`
	QuienFue string `json:"quien_fue"` // si es THREAD_JOIN es un uint32, si es Mutex es un string
}
