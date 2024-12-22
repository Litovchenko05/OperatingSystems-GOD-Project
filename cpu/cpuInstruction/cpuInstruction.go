package cpuInstruction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/sisoputnfrba/tp-golang/cpu/client"
	"github.com/sisoputnfrba/tp-golang/cpu/mmu"
	"github.com/sisoputnfrba/tp-golang/cpu/utils"
	"github.com/sisoputnfrba/tp-golang/utils/types"
)

// Función para asignar el valor a un registro
func AsignarValorRegistro(registro string, valor uint32, tid uint32, logger *slog.Logger) {
	// Obtener una referencia a los registros
	registros := client.ReceivedContextoEjecucion

	// Asignar el valor al registro correspondiente
	switch registro {
	case "PC":
		registros.PC = valor
	case "AX":
		registros.AX = valor
	case "BX":
		registros.BX = valor
	case "CX":
		registros.CX = valor
	case "DX":
		registros.DX = valor
	case "EX":
		registros.EX = valor
	case "FX":
		registros.FX = valor
	case "GX":
		registros.GX = valor
	case "HX":
		registros.HX = valor
	case "Base":
		registros.Base = valor
	case "Limite":
		registros.Limite = valor
	default:
		logger.Error(fmt.Sprintf("Registro desconocido: %s", registro))
		return
	}

	// Log de la instrucción ejecutada
	logger.Info(fmt.Sprintf("## TID: %d - Ejecutando: SET - Registro: %s, Valor: %d", tid, registro, valor))
}

// Función para sumar el valor de dos registros
func SumarRegistros(registroDestino, registroOrigen string, tid uint32, logger *slog.Logger) {

	// Obtener los valores de los registros
	valorDestino := obtenerValorRegistro(registroDestino, logger)
	valorOrigen := obtenerValorRegistro(registroOrigen, logger)

	// Sumar los valores
	nuevoValor := valorDestino + valorOrigen

	// Asignar el nuevo valor al registro destino
	AsignarValorRegistro(registroDestino, nuevoValor, tid, logger)

	// Log de la instrucción ejecutada
	logger.Info(fmt.Sprintf("## TID: %d - Ejecutando: SUM - Registro Destino: %s, Registro Origen: %s", tid, registroDestino, registroOrigen))
}

// Función para restar el valor de dos registros
func RestarRegistros(registroDestino, registroOrigen string, tid uint32, logger *slog.Logger) {

	// Obtener los valores de los registros
	valorDestino := obtenerValorRegistro(registroDestino, logger)
	valorOrigen := obtenerValorRegistro(registroOrigen, logger)

	// Restar los valores
	nuevoValor := valorDestino - valorOrigen

	// Asignar el nuevo valor al registro destino
	AsignarValorRegistro(registroDestino, nuevoValor, tid, logger)

	// Log de la instrucción ejecutada
	logger.Info(fmt.Sprintf("## TID: %d - Ejecutando: SUB - Registro Destino: %s, Registro Origen: %s", tid, registroDestino, registroOrigen))
}

// Función para realizar el salto condicional JNZ
func SaltarSiNoCero(registro string, instruccion string, tid uint32, logger *slog.Logger) {

	// Obtener el valor del registro
	valorRegistro := obtenerValorRegistro(registro, logger)

	// Si el valor del registro es distinto de cero, actualizar el Program Counter (PC)
	if valorRegistro != 0 {
		// Convertir la instrucción a un valor numérico
		instruccionNueva, err := strconv.ParseUint(instruccion, 10, 32)
		if err != nil {
			logger.Error(fmt.Sprintf("Error al convertir instrucción para JNZ: %s", instruccion))
			return
		}

		// Asignar el nuevo valor del PC
		AsignarValorRegistro("PC", uint32(instruccionNueva), tid, logger)

		// Log de la instrucción ejecutada
		logger.Info(fmt.Sprintf("## TID: %d - Ejecutando: JNZ - Registro: %s, Nueva Instrucción: %s", tid, registro, instruccion))
	}
}

// Función para escribir en el log el valor de un registro
func LogRegistro(registro string, pidtid types.PIDTID, logger *slog.Logger) {
	// Obtener una referencia a los registros
	valor := obtenerValorRegistro(registro, logger)

	// Log de la instrucción ejecutada
	logger.Info(fmt.Sprintf("## TID: %d - Ejecutando: LOG - Registro: %s, Valor: %d", pidtid.TID, registro, valor))
}

// Función auxiliar para obtener el valor de un registro
func obtenerValorRegistro(registro string, logger *slog.Logger) uint32 {
	registros := client.ReceivedContextoEjecucion

	switch registro {
	case "PC":
		return registros.PC
	case "AX":
		return registros.AX
	case "BX":
		return registros.BX
	case "CX":
		return registros.CX
	case "DX":
		return registros.DX
	case "EX":
		return registros.EX
	case "FX":
		return registros.FX
	case "GX":
		return registros.GX
	case "HX":
		return registros.HX
	case "Base":
		return registros.Base
	case "Limite":
		return registros.Limite
	default:
		logger.Error(fmt.Sprintf("Registro desconocido: %s", registro))
		return 0
	}
}

// Función para leer un valor de una dirección física de memoria y almacenarlo en un registro
func LeerMemoria(registroDatos string, registroDireccion string, pidtid types.PIDTID, logger *slog.Logger) {

	// Obtener el valor de la dirección lógica del registro de dirección
	direccionLogica := obtenerValorRegistro(registroDireccion, logger)

	procesoPaquende := types.Proceso{
		Pid:               pidtid.PID,
		Tid:               pidtid.TID,
		ContextoEjecucion: *client.ReceivedContextoEjecucion,
	}

	direccionFisica, err := mmu.TraducirDireccion(&procesoPaquende, direccionLogica, logger)
	if err != nil {
		logger.Error(fmt.Sprintf("Error al traducir la dirección lógica en READ_MEM: %v", err))
		return
	}

	// Log obligatorio de Lectura de Memoria
	logger.Info(fmt.Sprintf("## TID: %d - Acción: LEER - Dirección Física: %d", pidtid.TID, direccionFisica))

	// Crear la estructura de solicitud para el módulo de Memoria
	requestData := struct {
		DireccionFisica uint32 `json:"direccion_fisica"`
		TID             uint32 `json:"tid"`
		PID             uint32 `json:"pid"`
	}{
		DireccionFisica: direccionFisica,
		TID:             pidtid.TID,
		PID:             pidtid.PID,
	}

	// Serializar los datos en JSON
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		logger.Error("Error al serializar la solicitud de READ_MEM", slog.Any("error", err))
		return
	}

	// Crear la URL del módulo de Memoria
	url := fmt.Sprintf("http://%s:%d/read_mem", utils.Configs.IpMemory, utils.Configs.PortMemory)

	// Crear la solicitud POST
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("Error al crear la solicitud de READ_MEM", slog.Any("error", err))
		return
	}

	// Establecer el encabezado de la solicitud
	req.Header.Set("Content-Type", "application/json")

	// Enviar la solicitud
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Error al enviar la solicitud de READ_MEM", slog.Any("error", err))
		return
	}
	defer resp.Body.Close()

	// Verificar si la respuesta fue exitosa
	if resp.StatusCode != http.StatusOK {
		logger.Error(fmt.Sprintf("Error en la respuesta de READ_MEM: Código de estado %d", resp.StatusCode))
		return
	}

	// Decodificar la respuesta para obtener el valor leído
	var responseData struct {
		Valor uint32 `json:"valor"`
	}
	err = json.NewDecoder(resp.Body).Decode(&responseData)
	if err != nil {
		logger.Error("Error al decodificar el valor leído de memoria", slog.Any("error", err))
		return
	}

	// Almacenar el valor leído en el registro correspondiente
	AsignarValorRegistro(registroDatos, responseData.Valor, pidtid.TID, logger)

	// Log de la instrucción ejecutada
	logger.Info(fmt.Sprintf("Instrucción Ejecutada: “## TID: %d - Ejecutando: READ_MEM - Dirección Física: %d, Valor Leído: %d”", pidtid.TID, direccionFisica, responseData.Valor))
}

// Función para escribir un valor de un registro en una dirección física de memoria
func EscribirMemoria(registroDireccion string, registroDatos string, pidtid types.PIDTID, logger *slog.Logger) {

	// Obtener el valor de la dirección lógica y el valor de datos de los registros
	direccionLogica := obtenerValorRegistro(registroDireccion, logger)
	valorDatos := obtenerValorRegistro(registroDatos, logger)

	procesoPaquende := types.Proceso{
		Pid:               pidtid.PID,
		Tid:               pidtid.TID,
		ContextoEjecucion: *client.ReceivedContextoEjecucion,
	}

	// Traducir la dirección lógica a una dirección física usando la MMU
	direccionFisica, err := mmu.TraducirDireccion(&procesoPaquende, direccionLogica, logger)
	if err != nil {
		logger.Error(fmt.Sprintf("Error al traducir la dirección lógica en WRITE_MEM: %v", err))
		return
	}

	// Log obligatorio de Escritura de Memoria
	logger.Info(fmt.Sprintf("## TID: %d - Acción: ESCRIBIR - Dirección Física: %d", pidtid.TID, direccionFisica))

	// Crear la estructura de solicitud para el módulo Memoria
	requestData := struct {
		DireccionFisica uint32 `json:"direccion_fisica"`
		Valor           uint32 `json:"valor"`
		TID             uint32 `json:"tid"`
	}{
		DireccionFisica: direccionFisica,
		Valor:           valorDatos,
		TID:             pidtid.TID,
	}

	// Serializar los datos en JSON
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		logger.Error("Error al serializar la solicitud de WRITE_MEM", slog.Any("error", err))
		return
	}

	// Crear la URL del módulo de Memoria
	ipMemory := utils.Configs.IpMemory     // La IP del módulo de Memoria
	portMemory := utils.Configs.PortMemory // El puerto del módulo de Memoria
	url := fmt.Sprintf("http://%s:%d/write_mem", ipMemory, portMemory)

	// Crear la solicitud POST
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("Error al crear la solicitud de WRITE_MEM", slog.Any("error", err))
		return
	}

	// Establecer el encabezado de la solicitud
	req.Header.Set("Content-Type", "application/json")

	// Enviar la solicitud
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Error al enviar la solicitud de WRITE_MEM", slog.Any("error", err))
		return
	}
	defer resp.Body.Close()

	// Verificar si la respuesta fue exitosa
	if resp.StatusCode != http.StatusOK {
		logger.Error(fmt.Sprintf("Error en la respuesta de WRITE_MEM: Código de estado %d", resp.StatusCode))
		return
	}

	// Log de la instrucción ejecutada
	logger.Info(fmt.Sprintf("## TID: %d - Ejecutando: WRITE_MEM - Dirección Física: %d, Valor: %d", pidtid.TID, direccionFisica, valorDatos))
}
