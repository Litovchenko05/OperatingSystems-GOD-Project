package memUsuario

import (
	"fmt"
	"log/slog"

	"github.com/sisoputnfrba/tp-golang/memoria/memSistema"
	"github.com/sisoputnfrba/tp-golang/memoria/utils"
	"github.com/sisoputnfrba/tp-golang/utils/types"
)

// var memoria global
var MemoriaDeUsuario []byte
var Particiones []types.Particion
var ParticionesDinamicas []int
var BitmapParticiones []bool
var PidAParticion map[uint32]int // Mapa para rastrear la asignación de PIDs a particiones

// Funcion para iniciar la memoria y definir las particiones
func Inicializar_Memoria_De_Usuario(logger *slog.Logger) {
	// Inicializar el espacio de memoria con 1024 bytes
	MemoriaDeUsuario = make([]byte, utils.Configs.MemorySize)

	// Asignar las particiones fijas en la memoria usando los datos de config
	var base uint32 = 0
	for i, limite := range utils.Configs.Partitions {
		particion := types.Particion{
			Base:   base,
			Limite: uint32(limite),
		}
		Particiones = append(Particiones, particion)
		logger.Info("Partición fija", "Número", i+1, "iniciada con Base", particion.Base, "y Límite", particion.Limite)
		base += uint32(limite)
	}
	// Inicializar el bitmap y el mapa de PIDs
	//todas las particiones estan libres = false
	BitmapParticiones = make([]bool, len(Particiones))
	PidAParticion = make(map[uint32]int)
}

// memoria para particiones dinamicas
func Inicializar_Memoria_Dinamica(logger *slog.Logger) {
	MemoriaDeUsuario = make([]byte, utils.Configs.MemorySize)
	particion := types.Particion{
		Base:   0,
		Limite: uint32(utils.Configs.MemorySize),
	}
	Particiones = []types.Particion{particion}
	BitmapParticiones = []bool{false} // Un solo valor para la memoria dinámica
	ParticionesDinamicas = append(ParticionesDinamicas, 1024)
	logger.Info(fmt.Sprintf("Memoria dinámica inicializada: ## Base = %d, Límite = %d", particion.Base, particion.Limite))
	// Inicializar el mapa de PIDs
	PidAParticion = make(map[uint32]int)
}

// Función para liberar una partición por PID
func LiberarParticionPorPID(pid uint32, logger *slog.Logger) error {
	particion, existe := PidAParticion[pid]
	if !existe {
		return fmt.Errorf("no se encontró el proceso %d asignado a ninguna partición", pid)
	}

	// Liberar la partición y actualizar el bitmap
	BitmapParticiones[particion] = false
	delete(PidAParticion, pid) // Eliminar la entrada del mapa

	// Comprobar si el esquema es dinámico
	if utils.Configs.Scheme == "DINAMICA" {
		var particionIndex int
		for particionIndex = 0; particionIndex < len(ParticionesDinamicas); particionIndex++ {
			// Verificar si hay particiones libres adyacentes
			adyacenteIzquierdaLibre := particionIndex > 0 && !BitmapParticiones[particionIndex-1]
			adyacenteDerechaLibre := particionIndex < len(Particiones)-1 && !BitmapParticiones[particionIndex+1]

			// Llamar a combinarParticionesLibres si alguna adyacente está libre
			if adyacenteIzquierdaLibre || adyacenteDerechaLibre {
				combinarParticionesLibres(particionIndex, logger)
			}
		}
	}
	return nil
}

// combinar particiones dinamicas libres en una sola
func combinarParticionesLibres(index int, logger *slog.Logger) {
	// Inicializar variables para los límites de la nueva partición combinada
	base := Particiones[index].Base
	limite := Particiones[index].Limite

	// Verificar partición anterior, si existe y está libre
	if index > 0 && !BitmapParticiones[index-1] {
		base = Particiones[index-1].Base
		limite += Particiones[index-1].Limite
		// Eliminar partición anterior ya que se combina
		Particiones = append(Particiones[:index-1], Particiones[index:]...)
		BitmapParticiones = append(BitmapParticiones[:index-1], BitmapParticiones[index:]...)
		index-- // Ajustar el índice ya que hemos eliminado la partición anterior
	}

	// Verificar partición siguiente, si existe y está libre
	if index < len(Particiones)-1 && !BitmapParticiones[index+1] {
		limite += Particiones[index+1].Limite
		// Eliminar partición siguiente ya que se combina
		Particiones = append(Particiones[:index+1], Particiones[index+2:]...)
		BitmapParticiones = append(BitmapParticiones[:index+1], BitmapParticiones[index+2:]...)
	}

	// Actualizar la partición combinada en el índice actual
	Particiones[index] = types.Particion{Base: base, Limite: limite}
	BitmapParticiones[index] = false // Marcar como libre

	logger.Info("Particiones combinadas", "Nueva Base", base, "Nuevo Límite", limite)
}

func AsignarPID(pid uint32, tamanio_proceso int, path string, logger *slog.Logger) (bool, string) {
	var asigno = false
	algoritmo := utils.Configs.SearchAlgorithm
	esquema := utils.Configs.Scheme

	// Comprobación del esquema de memoria
	if esquema == "FIJAS" {
		// Algoritmos de particionamiento fijo
		switch algoritmo {
		case "FIRST":
			asigno = FirstFitFijo(pid, tamanio_proceso, path, logger)
		case "BEST":
			asigno = BestFitFijo(pid, tamanio_proceso, path, logger)
		case "WORST":
			asigno = WorstFitFijo(pid, tamanio_proceso, path, logger)
		}

		// Resultado para particiones fijas
		if asigno {
			return true, "OK"
		} else {
			return false, "NO SE PUDO INICIALIZAR EL PROCESO POR FALTA DE HUECOS EN LAS PARTICIONES"
		}
	} else if esquema == "DINAMICAS" {
		// Algoritmos de particionamiento dinámico
		switch algoritmo {
		case "FIRST":
			asigno = FirstFitDinamico(pid, tamanio_proceso, path)
		case "BEST":
			asigno = BestFitDinamico(pid, tamanio_proceso, path)
		case "WORST":
			asigno = WorstFitDinamico(pid, tamanio_proceso, path)
		}

		// Si se asignó la partición correctamente, retornamos
		if asigno {
			return true, "OK"
		} else {
			// Si no se pudo asignar, intentamos compactar
			compactar := SePuedeCompactar(tamanio_proceso)
			if compactar {
				return false, "COMPACTACION"
			} else {
				return false, "NO SE PUDO INICIALIZAR EL PROCESO POR FALTA DE HUECOS EN LAS PARTICIONES"
			}
		}
	}

	// Si el esquema es incorrecto, retornamos un error
	return false, "ESQUEMA MEMORIA ERROR"
}

// first fit para particiones fijas
func FirstFitFijo(pid uint32, tamanio_proceso int, path string, logger *slog.Logger) bool {
	particion := utils.Configs.Partitions
	for i := 0; i < len(BitmapParticiones); i++ {
		if !BitmapParticiones[i] {
			if tamanio_proceso <= particion[i] {
				PidAParticion[pid] = i
				BitmapParticiones[i] = true
				memSistema.CrearContextoPID(pid, uint32(Particiones[i].Base), uint32(Particiones[i].Limite))
				memSistema.CrearContextoTID(pid, 0, path)
				logger.Info(fmt.Sprintf("PROCESO: %d, ASIGNADO PARTICION: %d", pid, i))
				return true
			}
		}
	}
	return false
}

// best fit para particiones fijas
func BestFitFijo(pid uint32, tamanio_proceso int, path string, logger *slog.Logger) bool {

	particiones := utils.Configs.Partitions
	var menor = 1024
	var pos_menor = -1
	for i := 0; i < len(BitmapParticiones); i++ {
		if !BitmapParticiones[i] { // Verifica que la partición esté libre.
			if tamanio_proceso <= particiones[i] { // Verifica que la partición sea suficiente para el tamaño del proceso.
				if particiones[i] < menor { // Busca la partición más pequeña dentro de las válidas.
					menor = particiones[i]
					pos_menor = i
				}
			}
		}
	}
	if pos_menor == -1 {
		return false
	} else {
		PidAParticion[pid] = pos_menor      // Asocia el PID con la partición encontrada.
		BitmapParticiones[pos_menor] = true // Marca la partición como ocupada.
		memSistema.CrearContextoPID(pid, uint32(Particiones[pos_menor].Base), uint32(Particiones[pos_menor].Limite))
		memSistema.CrearContextoTID(pid, 0, path)

		logger.Info(fmt.Sprintf("PROCESO: %d ASIGNADO PARTICION: %d", pid, pos_menor))
		return true
	}
}

// worst fit para particiones fijas
func WorstFitFijo(pid uint32, tamanio_proceso int, path string, logger *slog.Logger) bool {
	particiones := utils.Configs.Partitions
	var mayor = 0 // Variables para guardar la mayor partición válida y su posición
	var pos_mayor = -1
	for i := 0; i < len(BitmapParticiones); i++ { // Recorremos todas las particiones
		if !BitmapParticiones[i] { // Verificamos si la partición está libre
			if tamanio_proceso <= particiones[i] { // Verificamos si la partición puede almacenar el proceso
				if particiones[i] > mayor { // Si es la mayor partición encontrada hasta ahora, actualizamos
					mayor = particiones[i] // Guardamos el tamaño de la partición
					pos_mayor = i          // Guardamos la posición de la partición
				}
			}
		}
	}
	if pos_mayor == -1 { // Si no se encontró una partición válida, retornamos false
		return false
	} else {
		PidAParticion[pid] = pos_mayor
		BitmapParticiones[pos_mayor] = true
		memSistema.CrearContextoPID(pid, uint32(Particiones[pos_mayor].Base), uint32(Particiones[pos_mayor].Limite))
		memSistema.CrearContextoTID(pid, 0, path)

		logger.Info(fmt.Sprintf("PROCESO: %d ASIGNADO PARTICION: %d", pid, pos_mayor))
		return true
	}
}

func FirstFitDinamico(pid uint32, tamanio_proceso int, path string) bool {
	for i := 0; i < len(BitmapParticiones); i++ {
		if !BitmapParticiones[i] && ParticionesDinamicas[i] >= tamanio_proceso {
			AsignarParticion(pid, i, tamanio_proceso, path)
			return true
		}
	}
	return false // No hay partición adecuada
}

func BestFitDinamico(pid uint32, tamanio_proceso int, path string) bool {
	var pos_menor = -1
	var menor = 30000 // Valor arbitrario para comparar

	for i := 0; i < len(BitmapParticiones); i++ {
		if !BitmapParticiones[i] && ParticionesDinamicas[i] >= tamanio_proceso {
			if ParticionesDinamicas[i] < menor { // Busca la partición más ajustada
				menor = ParticionesDinamicas[i]
				pos_menor = i
			}
		}
	}

	if pos_menor == -1 {
		return false // No hay partición adecuada
	}

	AsignarParticion(pid, pos_menor, tamanio_proceso, path)
	return true
}

// empiezo con un solo espacio de memoria de 1024 bytes, si no esta reservado lo hago con el pid entrante, sino no hay espacio
func WorstFitDinamico(pid uint32, tamanio_proceso int, path string) bool {
	var pos_mayor = -1
	var mayor = 0
	for i := 0; i < len(BitmapParticiones); i++ {
		if !BitmapParticiones[i] {
			if tamanio_proceso <= ParticionesDinamicas[i] {
				if ParticionesDinamicas[i] >= mayor {
					mayor = ParticionesDinamicas[i]
					pos_mayor = i
				}
			}
		}
	}
	if pos_mayor == -1 {
		return false
	} else {
		AsignarParticion(pid, pos_mayor, tamanio_proceso, path)
		return true
	}
}

func BaseDinamica(posicion int) uint32 {
	var base = 0
	for i := 0; i < posicion; i++ {
		base += ParticionesDinamicas[i]
	}
	return uint32(base + 1)
}

func AsignarParticion(pid uint32, posicion, tamanio_proceso int, path string) {
	espacioDisponible := ParticionesDinamicas[posicion]
	nuevaParticion := espacioDisponible - tamanio_proceso

	if nuevaParticion > 0 {
		// Si queda espacio, creamos una nueva partición
		ParticionesDinamicas = append(ParticionesDinamicas[:posicion+1], append([]int{nuevaParticion}, ParticionesDinamicas[posicion+1:]...)...)
		BitmapParticiones = append(BitmapParticiones[:posicion+1], append([]bool{false}, BitmapParticiones[posicion+1:]...)...)
	}

	// Actualizar la partición asignada
	ParticionesDinamicas[posicion] = tamanio_proceso
	BitmapParticiones[posicion] = true
	PidAParticion[pid] = posicion

	// Crear contexto de memoria
	base := BaseDinamica(posicion)
	memSistema.CrearContextoPID(pid, base, uint32(tamanio_proceso))
	memSistema.CrearContextoTID(pid, 0, path)
}

func SePuedeCompactar(tamanio_proceso int) bool {
	var espacioLibre = 0
	particiones := ParticionesDinamicas
	for i := 0; i < len(BitmapParticiones); i++ {
		if !BitmapParticiones[i] {
			espacioLibre += particiones[i]
		}
	}
	return espacioLibre > tamanio_proceso
}

func Compactar() bool {
	var espacioLibre = 0
	var nuevasParticiones []int // Para almacenar las particiones compactadas
	var nuevoBitmap []bool      // Para el nuevo estado del bitmap

	// Recorremos las particiones actuales
	for i := 0; i < len(BitmapParticiones); i++ {
		if BitmapParticiones[i] {
			// Si la partición está ocupada, la copiamos al nuevo arreglo
			nuevasParticiones = append(nuevasParticiones, ParticionesDinamicas[i])
			nuevoBitmap = append(nuevoBitmap, true)
		} else {
			// Si está libre, sumamos su espacio a `espacioLibre`
			espacioLibre += ParticionesDinamicas[i]
		}
	}

	// Actualizamos las particiones y el bitmap globales
	ParticionesDinamicas = nuevasParticiones
	BitmapParticiones = nuevoBitmap

	ParticionesDinamicas = append(ParticionesDinamicas, espacioLibre)
	BitmapParticiones = append(BitmapParticiones, false)

	// Retorna `true` si hubo compactación (es decir, si se encontró espacio libre)
	return true
}
