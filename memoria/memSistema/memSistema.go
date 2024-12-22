package memSistema

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/sisoputnfrba/tp-golang/memoria/utils"
	"github.com/sisoputnfrba/tp-golang/utils/types"
)

var mu sync.Mutex

// Mapa para almacenar los contextos de ejecución de los procesos y sus hilos asociados
var ContextosPID = make(map[uint32]types.ContextoEjecucionPID) // Contexto por PID

// Función para inicializar un contexto de ejecución de un proceso (PID)
func CrearContextoPID(pid uint32, base uint32, limite uint32) {
	ContextosPID[pid] = types.ContextoEjecucionPID{
		PID:    pid,
		Base:   base,
		Limite: limite,
		TIDs:   make(map[uint32]types.ContextoEjecucionTID),
	}
}

// Función para inicializar un contexto de ejecución de un hilo (TID) asociado a un proceso (PID)
func CrearContextoTID(pid uint32, tid uint32, archivoPseudocodigo string) {
	listaInstrucciones := CargarPseudocodigo(int(pid), int(tid), archivoPseudocodigo)
	if proceso, exists := ContextosPID[pid]; exists {
		proceso.TIDs[tid] = types.ContextoEjecucionTID{
			PC:                 0,
			AX:                 0,
			BX:                 0,
			CX:                 0,
			DX:                 0,
			EX:                 0,
			FX:                 0,
			GX:                 0,
			HX:                 0,
			LISTAINSTRUCCIONES: listaInstrucciones, // pseudocodigo
		}
		ContextosPID[pid] = proceso // Actualizar el contexto en el mapa
	}
}

// Función para eliminar el contexto de ejecución de un proceso (PID)
func EliminarContextoPID(pid uint32) {
	delete(ContextosPID, pid)
}

// Función para eliminar el contexto de ejecución de un hilo (TID) asociado a un proceso (PID)
func EliminarContextoTID(pid uint32, tid uint32) {
	if proceso, exists := ContextosPID[pid]; exists {
		if _, tidExists := proceso.TIDs[tid]; tidExists {
			delete(proceso.TIDs, tid)
			ContextosPID[pid] = proceso
		}
	}
}

func Actualizar_TID(pid uint32, tid uint32, contexto types.ContextoEjecucionTID) {
	if proceso, exists := ContextosPID[pid]; exists {
		if _, tidExists := proceso.TIDs[tid]; tidExists {
			mu.Lock()
			proceso.TIDs[tid] = contexto // Actualizar el contexto en el mapa
			ContextosPID[pid] = proceso  // Actualizar el contexto en el mapa
			mu.Unlock()
		}
	}
}

// Funcion para cargar el archivo de pseudocodigo
func CargarPseudocodigo(pid int, tid int, path string) map[string]string {
	file, err := os.Open(utils.Configs.InstructionPath + path)
	if err != nil {
		fmt.Printf("error al abrir el archivo %s: %v", path, err)
	}
	defer file.Close()
	var contextosEjecucion = make(map[int]map[int]*types.ContextoEjecucionTID)

	if contextosEjecucion[pid] == nil {
		contextosEjecucion[pid] = make(map[int]*types.ContextoEjecucionTID)
	}
	if contextosEjecucion[pid][tid] == nil {
		contextosEjecucion[pid][tid] = &types.ContextoEjecucionTID{
			LISTAINSTRUCCIONES: make(map[string]string),
		}
	}

	contexto := contextosEjecucion[pid][tid]

	scanner := bufio.NewScanner(file)
	instruccionNum := 0 // Indice de instrucciones

	//Empiezo a leer y guardo linea x linea
	for scanner.Scan() {
		linea := scanner.Text()
		contexto.LISTAINSTRUCCIONES[strconv.Itoa(instruccionNum)] = linea
		instruccionNum++
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("error al leer el archivo %s: %v", path, err)
	}
	return contexto.LISTAINSTRUCCIONES
}

func BuscarSiguienteInstruccion(pid, tid uint32, pc uint32) string {

	if proceso, exists := ContextosPID[pid]; exists {
		if hilo, tidExists := proceso.TIDs[tid]; tidExists {
			indiceInstruccion := pc
			instruccion, existe := hilo.LISTAINSTRUCCIONES[fmt.Sprintf("%d", indiceInstruccion)]
			if !existe {
				return ""
			}
			return instruccion
		} else {
			return ""
		}
	} else {
		return ""
	}
}
