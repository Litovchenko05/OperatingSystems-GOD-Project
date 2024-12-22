package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

func Enviar_Body[T any](dato T, ip string, puerto int, endpoint string, logger *slog.Logger) bool {

	body, err := json.Marshal(dato)
	if err != nil {
		logger.Error("Se produjo un error codificando el mensaje")
		return false
	}

	url := fmt.Sprintf("http://%s:%d/%s", ip, puerto, endpoint)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		logger.Error(fmt.Sprintf("Se produjo un error enviando mensaje a ip:%s puerto:%d", ip, puerto))
		return false
	}
	// Aseguramos que el body sea cerrado
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("La respuesta del servidor no fue OK")
		return false // Indica que la respuesta no fue exitosa
	}

	return true // Indica que la respuesta fue exitosa
}
func Enviar_QueryPath[T any](dato T, ip string, puerto int, endpoint string, verbo string, logger *slog.Logger) bool {
	cliente := &http.Client{}
	url := fmt.Sprintf("http://%s:%d/%s/%v", ip, puerto, endpoint, dato)
	req, err := http.NewRequest("verbo", url, nil)
	if err != nil {
		return false
	}
	// Establecer el encabezado Content-Type
	req.Header.Set("Content-Type", "application/json")

	// Enviar la solicitud
	resp, err := cliente.Do(req)
	if err != nil {
		logger.Error(fmt.Sprintf("Error al enviar la solicitud: %v", err))
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("La respuesta del servidor no fue OK")
		return false // Indica que la respuesta no fue exitosa
	}

	return true // Indica que la respuesta fue exitosa
}
