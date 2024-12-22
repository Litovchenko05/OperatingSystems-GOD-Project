package logging

import (
	"log/slog"
	"os"
)

// Se le pasa por parametro la ruta del archivo de log y el nivel de log que está en el config
func Iniciar_Logger(rutaArchivo string, nivel string) *slog.Logger {
	// Abrir el archivo para log
	archivoLog, err := os.OpenFile(rutaArchivo, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}

	// Convertir el nivel de string a slog.Level
	var logLevel slog.Level
	switch nivel {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo // Nivel por defecto
	}

	// Handler con opciones personalizadas
	handlerOptions := &slog.HandlerOptions{
		Level: logLevel,
	}

	// Crear un handler que escriba en el archivo
	handler := slog.NewTextHandler(archivoLog, handlerOptions)

	// Crear el logger con el handler
	logger := slog.New(handler)

	return logger
}
