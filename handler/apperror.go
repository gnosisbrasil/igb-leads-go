package handler

import (
	"log"
	"net/http"

	"igb-leads-go/service"
)

// WriteAppError mirrors the controllers' handleError: AppError maps to
// its status, anything else is a logged 500.
func WriteAppError(w http.ResponseWriter, err error, logMsg string) {
	if ae, ok := service.AsAppError(err); ok {
		WriteError(w, ae.Status, ae.Message)
		return
	}
	log.Printf("%s: %v", logMsg, err)
	WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
}
