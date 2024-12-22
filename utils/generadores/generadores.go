package generadores

import (
	"github.com/sisoputnfrba/tp-golang/kernel/utils"
	"github.com/sisoputnfrba/tp-golang/utils/types"
)

var MapaParaTCBS map[uint32]uint32 = make(map[uint32]uint32)

var PidCounter uint32 = 0

// Genera un PID único (el tipo de dato uint32 es para que no tome valore negativos).
func Generar_PID() uint32 {
	PidCounter++
	return PidCounter
}

// Genera un PCB con un PID único y con las listas de TCBs y Mutexs vacías.
func Generar_PCB() types.PCB {
	mutex := make(map[string]string)
	tcbs := make(map[uint32]types.TCB)

	pcb := types.PCB{
		PID:    Generar_PID(),
		TCBs:   tcbs,
		Mutexs: mutex,
	}

	MapaParaTCBS[pcb.PID] = 0

	return pcb
}

// Genera un nuevo TCB y lo añade al PCB recibido por parámetro (pasar el pcb con &).
func Generar_TCB(pcb *types.PCB, prioridad int) types.TCB {

	var tid uint32
	if len(pcb.TCBs) == 0 {
		tid = MapaParaTCBS[pcb.PID]
	} else {
		MapaParaTCBS[pcb.PID]++
		tid = MapaParaTCBS[pcb.PID]
	}

	tcb := types.TCB{
		TID:       tid,
		Prioridad: prioridad,
		PID:       pcb.PID,
		Quantum:   utils.Configs.Quantum,
	}

	pcb.TCBs[tid] = tcb
	return tcb
}
