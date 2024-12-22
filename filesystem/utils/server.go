package utils

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/sisoputnfrba/tp-golang/utils/conexiones"
	"github.com/sisoputnfrba/tp-golang/utils/types"
)

func Iniciar_fileSystem(logger *slog.Logger) {
	mux := http.NewServeMux()

	// Endpoints
	mux.HandleFunc("POST /dump", DUMP(logger))

	conexiones.LevantarServidor(strconv.Itoa(Configs.Port), mux, logger)
}

func DUMP(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var magic types.DumpFile
		err := decoder.Decode(&magic)
		if err != nil {
			logger.Error(fmt.Sprintf("Error al decodificar mensaje: %s\n", err.Error()))
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Error al decodificar mensaje"))
			return
		}

		// Verificar si se cuenta con el espacio disponible
		bloquesNecesarios, espacioSuficiente := Verificar_Espacio_Disponible(magic.Tamanio, logger)
		if !espacioSuficiente {
			logger.Error("No hay espacio suficiente para el archivo")
			w.WriteHeader(http.StatusInsufficientStorage)
			w.Write([]byte("No hay espacio suficiente para el archivo"))
			return
		}

		// Reservar el bloque de índice y los bloques de datos correspondientes en el bitmap
		Reservar_Bloques_Del_Bitmap(bloquesNecesarios, len(bloquesNecesarios), magic.Nombre, logger)

		// Crear archivo de metadata
		Crear_Archivo_Metadata(magic.Nombre, int(bloquesNecesarios[0]), magic.Tamanio, logger) //Tengo dudas con el segundo parametro

		// Acceder al archivo de punteros y grabar todos los punteros reservados
		Escribir_Index_Block(int(bloquesNecesarios[0]), Convertir_Bytes_A_Uint32(bloquesNecesarios[1:]), magic.Nombre, logger)

		// Acceder bloque a bloque e ir escribiendo el contenido de la memoria.
		Escribir_Datos_En_Bloques(bloquesNecesarios[1:], magic.Datos, magic.Nombre, logger)

		logger.Info(fmt.Sprintf("## Archivo Creado: %s - Tamaño: %d", magic.Nombre, magic.Tamanio))
		logger.Info(fmt.Sprintf("## Fin de solicitud - Archivo: %s", magic.Nombre))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// Devuelve dos valores: un array con los indices a reservar([]byte) y si hay espacio suficiente para el archivo(true/false)
func Verificar_Espacio_Disponible(tamanioArchivo int, logger *slog.Logger) ([]byte, bool) {
	bloquesNecesarios := tamanioArchivo / Configs.BlockSize
	if tamanioArchivo%Configs.BlockSize > 0 {
		bloquesNecesarios++ // Si el tamaño no es multiplo del BlockSize, se necesita un bloque más
	}

	// Identificar los bloques libres
	var bitsLibres []byte
	totalBitsLibres := 0

	for i := 0; i < len(Bitmap); i++ {
		for j := 0; j < 8; j++ {
			if (Bitmap[i] & (1 << j)) == 0 {
				totalBitsLibres++

				if len(bitsLibres) < bloquesNecesarios+1 { // +1 para el bloque de índice
					bitsLibres = append(bitsLibres, byte(i*8+j))
				}
			}
		}
	}
	if bloquesNecesarios > totalBitsLibres {
		return bitsLibres, false
	}

	BloquesLibres = totalBitsLibres // Actualizar la cantidad de bloques libres en la variable global
	return bitsLibres, true
}

// Reservo los bloques en el slice
func Reservar_Bloques_Del_Bitmap(bloques []byte, cantidad int, nombreArchivo string, logger *slog.Logger) {

	// Reservar los bloques
	for i := 0; i < cantidad; i++ {
		bloque := bloques[i]
		byteIndex := bloque / 8
		bitIndex := bloque % 8
		Bitmap[byteIndex] |= (1 << bitIndex)
		BloquesLibres--
		logger.Info(fmt.Sprintf("## Bloque asignado: %d - Archivo: %s - Bloques Libres: %d", bloque, nombreArchivo, BloquesLibres)) // Marcar el bit como ocupado
	}

	file, err := os.OpenFile(Configs.MountDir+"/bitmap.dat", os.O_RDWR, 0644)
	if err != nil {
		logger.Error(fmt.Sprintf("Error al abrir el archivo bitmap.dat: %s", err.Error()))
	}
	defer file.Close()

	// Guardar los cambios en el archivo
	err = os.WriteFile(Configs.MountDir+"/bitmap.dat", Bitmap, 0644)
	if err != nil {
		logger.Error(fmt.Sprintf("Error al escribir el archivo bitmap.dat: %s", err.Error()))
		return
	}

	logger.Info(fmt.Sprintf("Reservados %d bloques en el archivo bitmap.dat", cantidad))
}

func Crear_Archivo_Metadata(nombreArchivo string, indice int, tamanio int, logger *slog.Logger) {
	rutaArchivo := Configs.MountDir + "/files/" + nombreArchivo

	metadata := fmt.Sprintf(`{
        "index_block": %d,
        "size": %d
    }`, indice, tamanio)

	err := os.WriteFile(rutaArchivo, []byte(metadata), 0644)
	if err != nil {
		logger.Error(fmt.Sprintf("error al crear archivo de metadata: %v", err))
		return
	}
}

func Escribir_Index_Block(indexBlock int, bloquesDatos []uint32, nombreArchivo string, logger *slog.Logger) bool {
	// Cargar archivo en un slice de bytes
	bloquesFile, err := os.OpenFile(Configs.MountDir+"/bloques.dat", os.O_RDWR, 0644)
	if err != nil {
		logger.Error(fmt.Sprintf("Error al abrir el archivo bloques.dat: %s", err.Error()))
		return false
	}
	defer bloquesFile.Close()

	posicion := indexBlock * Configs.BlockSize
	if _, err := bloquesFile.Seek(int64(posicion), 0); err != nil {
		logger.Error(fmt.Sprintf("Error al buscar posición en bloques.dat: %s", err.Error()))
		return false
	}

	for _, bloque := range bloquesDatos {
		buffer := make([]byte, 4)
		binary.BigEndian.PutUint32(buffer, uint32(bloque))

		if _, err := bloquesFile.Write(buffer); err != nil {
			logger.Error(fmt.Sprintf("Error al escribir en bloques.dat: %s", err.Error()))
			return false
		}

		// Latencia de acceso a bloque
		time.Sleep(time.Duration(Configs.BlockAccessDelay) * time.Millisecond)
	}

	logger.Info(fmt.Sprintf("## Acceso Bloque - Archivo: %s - Tipo Bloque: INDICE - Bloque File System %d", nombreArchivo, posicion/Configs.BlockSize))
	return true
}

func Convertir_Bytes_A_Uint32(bloquesBytes []byte) []uint32 {
	var bloquesUint32 []uint32
	for _, b := range bloquesBytes {
		bloquesUint32 = append(bloquesUint32, uint32(b))
	}
	return bloquesUint32
}

func Escribir_Datos_En_Bloques(bloquesReservados []byte, contenido []byte, nombreArchivo string, logger *slog.Logger) error {
	bloqueIndex := 0
	for i := 0; i < len(contenido); i += Configs.BlockSize {
		bloque := bloquesReservados[bloqueIndex]
		data := contenido[i:min(i+Configs.BlockSize, len(contenido))]

		// Escribir los datos en el bloque correspondiente
		err := Escribir_En_Bloque(int(bloque), data, nombreArchivo, logger)
		if err != nil {
			return fmt.Errorf("error al escribir datos en bloque %d: %v", bloque, err)
		}

		bloqueIndex++
	}

	return nil
}

func Escribir_En_Bloque(bloque int, data []byte, nombreArchivo string, logger *slog.Logger) error {

	rutaBloques := Configs.MountDir + "/bloques.dat"
	file, err := os.OpenFile(rutaBloques, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("error al abrir bloques.dat: %v", err)
	}
	defer file.Close()

	// Mover el puntero al bloque correcto y escribir los datos
	_, err = file.Seek(int64(bloque*Configs.BlockSize), 0)
	if err != nil {
		return fmt.Errorf("error al hacer seek en bloques.dat: %v", err)
	}

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("error al escribir en bloques.dat: %v", err)
	}
	logger.Info(fmt.Sprintf("## Acceso Bloque - Archivo: %s - Tipo Bloque: DATOS - Bloque File System %d", nombreArchivo, bloque))
	time.Sleep(time.Duration(Configs.BlockAccessDelay) * time.Millisecond)

	return nil
}
