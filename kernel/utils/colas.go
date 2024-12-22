package utils

import (
	"fmt"
	"log/slog"

	"github.com/sisoputnfrba/tp-golang/utils/types"
)

// Agrega un elemento a la cola (IMPORTANTE: la cola debe coincidar con el tipo de elemento)
func Encolar[T any](cola *[]T, elemento T) {
	*cola = append(*cola, elemento)
}

// Esta se usa para encolar en la cola de ready, ya que se necesita saber la prioridad del proceso
func Encolar_ColaReady(colaReady map[int][]types.TCB, tcb types.TCB) {
	if Configs.SchedulerAlgorithm == "CMN" {
		colaReady[tcb.Prioridad] = append(colaReady[tcb.Prioridad], tcb)
	} else {
		colaReady[0] = append(colaReady[0], tcb)
	}
}

// Sirve para desencolar un TCB de la cola de ready indicando su prioridad (para FIFO y PRIORIDADES usamos 0); Retorna el elemento y un booleano que indica si fue exitoso
func Desencolar_TCB(colasReady map[int][]types.TCB, prioridad int) (*types.TCB, bool) {

	// Verifica si existe una cola para la prioridad dada
	cola, existe := colasReady[prioridad]
	if !existe || len(cola) == 0 {
		return nil, false // No hay TCBs en la cola para esta prioridad
	}

	tcb := cola[0]

	// Actualiza la cola sacando el primer elemento
	colasReady[prioridad] = cola[1:]

	return &tcb, true
}

// Desencola un elemento de la cola y retorna ese elemento
func Desencolar[T any](cola *[]T) T {
	if len(*cola) == 0 {
		var vacio T
		return vacio // O manejar el caso de cola vacía
	}
	elemento := (*cola)[0]
	*cola = (*cola)[1:] // Elimina el primer elemento
	return elemento
}

// Desencola un elemento de la cola de BLOQUEADOS por motivo y retorna ese elemento (IMPORTANTE: tener en cuenta que Motivo es un tipo de dato Const)
func Desencolar_Por_Motivo(cola *[]Bloqueado, motivo Motivo) *Bloqueado {
	for i, elem := range *cola {
		if elem.Motivo == motivo {
			elemento := elem
			*cola = append((*cola)[:i], (*cola)[i+1:]...)
			return &elemento
		}
	}
	return nil // No se encontró un elemento con el motivo especificado
}

func Desencolar_cola_block(bloqueado Bloqueado, cola *[]Bloqueado) bool {
	for i, elem := range *cola {
		if elem.PID == bloqueado.PID && elem.TID == bloqueado.TID {
			*cola = append((*cola)[:i], (*cola)[i+1:]...)
			return true
		}
	}
	return false

}

func Sacar_TCB_Del_Map(mapaPCBS *map[uint32]types.PCB, pid uint32, tid uint32, logger *slog.Logger) {
	pcb, existe := (*mapaPCBS)[pid]
	if !existe {
		logger.Error(fmt.Sprintf("El PCB con PID %d no existe", pid))
		return
	}

	// Verificamos si el TCB existe dentro del PCB
	_, existeTCB := pcb.TCBs[tid]
	if !existeTCB {
		logger.Error(fmt.Sprintf("El TCB con TID %d no existe en el PCB con PID %d", tid, pid))
		return
	}

	// Eliminamos el TCB del mapa de TCBs
	delete(pcb.TCBs, tid)
	logger.Info(fmt.Sprintf("TCB con TID %d eliminado del PCB con PID %d", tid, pid)) // Esto no hace falta que vaya

	// Actualizamos el PCB en el mapa de PCBs
	(*mapaPCBS)[pid] = pcb
}

// Devuelve dos valores, el primero es la solicitud y el segundo es un booleano que indica si la cola está vacía
func Proxima_solicitud(cola *[]SolicitudIO) (SolicitudIO, bool) {
	if len(*cola) == 0 {
		return SolicitudIO{}, false
	}
	solicitud := (*cola)[0]
	*cola = (*cola)[1:] // Remueve la primera solicitud del slice
	return solicitud, true
}
